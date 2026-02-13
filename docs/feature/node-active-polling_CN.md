# 主动轮询驱动的节点自愈

## 背景

现有节点自愈为**告警驱动**模式，完整链路如下：

```
外部告警 (HTTP POST)
  → AegisAlert CRD
  → AlertController 匹配 Rule/Template
  → 创建 Argo Workflow
  → selfhealing Pod 执行 SOP
```

该模式依赖外部告警系统（Prometheus Alertmanager 等）的可靠推送。存在以下盲区：

- 告警系统本身故障或配置遗漏时，节点异常无法触达自愈
- 存量节点问题（控制器重启前已存在的异常）不会重新触发
- 告警系统与自愈系统的延迟链路过长

## 目标

在**不破坏现有告警驱动链路**的前提下，新增主动轮询能力：定期扫描集群内所有节点的 Prometheus 指标，发现异常后自动触发自愈流程。两者互为补充——告警驱动提供低延迟实时响应，轮询提供兜底和存量覆盖。

## 现有实现参考

### 告警驱动的三条触发规则

生产中使用如下三条 PromQL 表达式驱动自愈，轮询器复用同等业务语义：

```promql
# 1. 上升沿：节点存在严重异常 → 触发自愈
alert: NodeCriticalIssue
expr: count by (region, cluster, node) (
    aegis_node_status_condition{
      condition!~"NodeCordon|NodeHasRestarted|HighGpuTemp|HighGpuMemoryTemp|GpfsThreadDeadlock",
      node!~"^[0-9].*"
    }
  ) unless count by (region, cluster, node) (
    kube_node_labels{label_aegis_io_disable="true"}
  )

# 2. 下降沿-A：节点仅剩 NodeCordon、严重异常已消失 → 触发收尾/解除封锁
alert: NodeCriticalIssueDisappeared
expr: count by (region, cluster, node) (
    aegis_node_status_condition{condition="NodeCordon"}
  ) unless count by (region, cluster, node) (
    aegis_node_status_condition{condition!~"NodeCordon|NodeHasRestarted|HighGpuTemp|HighGpuMemoryTemp|GpfsThreadDeadlock"}
  ) unless count by (region, cluster, node) (
    kube_node_labels{label_aegis_io_disable="true"}
  )

# 3. 下降沿-B：节点存在 nodecheck 污点 → 触发 precheck 清理
alert: NodeCriticalIssueDisappeared
expr: count by (region, cluster, node) (
    kube_node_spec_taint{key="scitix.ai/nodecheck"} == 1
  ) unless count by (region, cluster, node) (
    kube_node_labels{label_aegis_io_disable="true"}
  )
```

### `device_aware` 可复用组件

`internal/device_aware/node_condition_informer.go` 已实现了一套完整的轮询框架，轮询器在此基础上构建：

| 组件 | 复用方式 |
|---|---|
| `NodeStatusInformer` 轮询骨架（ticker + fullSync） | 直接复用 |
| `NodeStatusEventHandler` 接口（`OnAdd/OnUpdate/OnDelete`） | 适配复用，handler 实现不同 |
| 防毛刺保护（空结果时不清空 cache） | 直接复用 |
| 版本哈希变更检测（SHA256 → int64） | 直接复用 |
| `prom.ListNodeStatusesWithQuery()` | 直接复用 |
| 并发查询模式（WaitGroup + mutex） | 直接复用 |

## 设计

### 单条 PromQL

只执行一条查询，拉取全量数据，所有过滤和分类逻辑在 Go 代码中完成：

```promql
aegis_node_status_condition
```

通过 `prom.ListNodeStatusesWithQuery()` 解析为 `[]AegisNodeStatus`，与现有代码完全兼容。节点过滤（`aegis.io/disable=true` 标签、`^[0-9].*` 节点名）通过 **Node lister** 在 Go 里实现，不依赖 PromQL。

### 优先级驱动的异常分类

复用 `analysis.go` 的 Priority 定义，在代码中分类（不再编码进 PromQL）：

| Priority 值 | 语义 | 轮询处理 |
|---|---|---|
| 0 (`NodeNotReady`) | 节点不可用 | 严重异常，触发 `NodeCriticalIssue` |
| 1 (`NodeCordon`) | 节点封锁 | 特殊状态，用于下降沿-A 检测 |
| 2~99 (`Emergency`) | 紧急异常 | 严重异常，触发 `NodeCriticalIssue` |
| 100~999 (`CanIgnore`) | 可忽略 | 跳过，不触发 |
| >999 (`MustIgnore`) | 强制忽略 | 跳过，不触发 |

原 PromQL 排除列表（`NodeHasRestarted`、`HighGpuTemp` 等）在生产 ConfigMap 中均为 priority=100（`CanIgnore`），这套分类与原规则完全等价，且随 ConfigMap 变更自动生效。

### Priority ConfigMap 动态加载

通过 ConfigMap informer watch `aegis-priority`（`monitoring` namespace），ConfigMap 变更时自动 reload，无需重启控制器：

```go
type PriorityWatcher struct {
    config map[string]analysis.Priority
    mu     sync.RWMutex
}

func (w *PriorityWatcher) IsCritical(condition string) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    p, ok := w.config[condition]
    if !ok {
        return false // 未知 condition 保守策略：不触发
    }
    return p <= analysis.Emergency // priority 0~99
}

func (w *PriorityWatcher) IsCordon(condition string) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    p, ok := w.config[condition]
    return ok && p == analysis.NodeCordon
}
```

同步将 `analysis.InitAnalysisConfig` 中的文件解析逻辑提取为 `ParsePriorityConfig(content string)`，供 watcher 和原有路径共同调用（对现有逻辑改动最小）。

### 三触发模式

```
PollInterval (默认 10s)
  ├── 查 Prometheus: aegis_node_status_condition → []AegisNodeStatus
  ├── 查 Node lister → 过滤 disable 节点和 ^[0-9].* 节点名
  ├── 按 node 聚合，用 PriorityWatcher 分类：
  │     criticalSet   = nodes where any condition priority 0~99
  │     cordonOnlySet = nodes where only NodeCordon (no critical)
  │
  ├── criticalSet 与 criticalCache 对比
  │     新节点出现  → OnAdd    ← 上升沿：创建 NodeCriticalIssue Alert
  │     节点消失    → OnDelete ← （清除 criticalCache 条目）
  │     （版本哈希未变 → noop，去重）
  │
  └── cordonOnlySet 与 cordonOnlyCache 对比
        新节点出现  → OnAdd    ← 下降沿-A：创建 NodeCriticalIssueDisappeared Alert
        节点消失    → OnDelete ← （清除 cordonOnlyCache 条目）

CordonResyncInterval (默认 10min)
  └── 扫描 cordonOnlyCache 中所有节点
        → 重触发 NodeCriticalIssueDisappeared Alert（无论当前 Alert 状态）
        （NodeCordon 收尾动作可能需要多次尝试）

ResyncInterval (默认 1h)
  └── 扫描 criticalCache 中所有节点
        → 若对应 AegisAlert 已消失或已完成 → 重建 NodeCriticalIssue Alert
        （防止 Alert 被 TTL 清理后条件仍在，导致遗漏）

Node Informer (独立，不依赖 Prometheus)
  └── Watch 节点 Spec.Taints 变化
        → 出现 scitix.ai/nodecheck 污点 ← 下降沿-B：触发 precheck 清理
```

### 数据结构

```go
// 严重异常节点的跟踪项
type criticalEntry struct {
    alertName    string               // 已创建的 AegisAlert name
    lastStatuses []prom.AegisNodeStatus
    since        time.Time
}

// Poller 主结构
type NodeStatusPoller struct {
    promClient  *prom.PromAPI
    cfg         NodePollerConfig
    priority    *PriorityWatcher

    criticalCache    map[string]*criticalEntry // node → entry
    cordonOnlyCache  map[string]struct{}        // node → present
    cacheLock        sync.RWMutex

    kubeClient  kubernetes.Interface
    nodeLister  corelisters.NodeLister
    alertClient alertclientset.Interface
}
```

### 触发动作

```go
// 上升沿：创建 AegisAlert{type=NodeCriticalIssue}
func (p *NodeStatusPoller) onCriticalRisingEdge(node string, statuses []prom.AegisNodeStatus)

// 下降沿-A：创建 AegisAlert{type=NodeCriticalIssueDisappeared}
func (p *NodeStatusPoller) onCordonOnlyRisingEdge(node string)

// 下降沿-B：通过 Node Informer 检测到 nodecheck 污点
func (p *NodeStatusPoller) onNodeCheckTaintAppear(node string)
```

## 配置

```go
type NodePollerConfig struct {
    Enabled              bool
    PollInterval         time.Duration // Prometheus 拉取 + 边沿检测间隔，默认 10s
    ResyncInterval       time.Duration // 严重异常全量重触发间隔，默认 1h
    CordonResyncInterval time.Duration // NodeCordon 下降沿定时重触发间隔，默认 10min
    MaxAlertsPerRound    int           // 单轮上升沿最多触发 Alert 数，默认 20
    PriorityConfigMap    string        // 优先级配置 ConfigMap 名，默认 "aegis-priority"
    PriorityNamespace    string        // ConfigMap 所在 namespace，默认 "monitoring"
}
```

## 文件结构

```
internal/controller/
  nodepoller/
    poller.go         # 主循环：fullSync、criticalResync、cordonResync
    classifier.go     # 单条 PromQL 结果的 Go 端分类逻辑
    priority.go       # PriorityWatcher：ConfigMap watch + 动态 reload
    handler.go        # 创建 AegisAlert CRD（onCriticalRisingEdge 等）
    cache.go          # criticalCache / cordonOnlyCache 及边沿检测

internal/selfhealing/analysis/analysis.go   # 提取 ParsePriorityConfig(string)（小改动）
internal/controller/aegis.go                # 注册 NodeStatusPoller（同 device_aware 模式）
selfhealing/config/config.go                # 增加 NodePollerConfig 字段
```

## 触发时序示例

```
t=0        PollInterval 触发
             criticalExpr → [node-A]（新出现）
             → OnAdd(node-A)，创建 AegisAlert{NodeCriticalIssue, node=A}

t=10s      PollInterval 触发
             criticalExpr → [node-A]（无变化）→ noop

t=10min    CordonResyncInterval 触发
             cordonOnlyCache 为空 → noop

t=30min    node-A 问题修复，指标消失，仅剩 NodeCordon
             criticalCache: OnDelete(node-A) → 清除
             cordonOnlyCache: OnAdd(node-A)  → 创建 AegisAlert{NodeCriticalIssueDisappeared}

t=40min    CordonResyncInterval 触发
             cordonOnlyCache: [node-A] → 重触发 NodeCriticalIssueDisappeared

t=50min    解封锁完成，NodeCordon 消失
             cordonOnlyCache: OnDelete(node-A) → 清除，不再重触发

t=1h       ResyncInterval 触发
             criticalCache 为空 → noop

--- 另一场景：Alert 被 TTL 清理但异常未恢复 ---

t=0        OnAdd(node-B)，创建 Alert

t=30min    Alert Workflow 完成，Alert 被 TTL 自动清理
             但 node-B 仍在 criticalExpr 结果中（异常未恢复）
             PollInterval 轮询：版本未变 → noop（不重复触发）

t=1h       ResyncInterval 触发
             criticalCache[node-B].alertName 对应 Alert 已消失
             → 重建 Alert{NodeCriticalIssue, node=B}  ← 定时保障
```

## 与现有告警驱动的关系

| 维度 | 告警驱动 | 主动轮询 |
|---|---|---|
| 触发时机 | 外部系统推送 | 周期性内部扫描（10s） |
| 延迟 | 取决于告警系统配置 | 最大 `PollInterval` |
| 存量问题覆盖 | 不覆盖 | 覆盖（控制器重启后自动恢复） |
| 条件过滤来源 | PromQL 硬编码 | Priority ConfigMap 动态配置 |
| 执行路径 | AlertController → Workflow | 相同（创建同类型 AegisAlert） |
| 重复触发防护 | 告警去重 | 版本哈希 + Alert 存在性检查 |
