# 告警负载影响属性增强 —— 实现方案

## 背景

当前 Aegis 的优先级系统（`priority.conf` + `analysis.go`）仅能区分故障的处理优先级（Emergency/CanIgnore 等），但无法表达"某个故障是否影响节点的可用负载"。Device-Aware 控制器将设备状态同步到节点 Annotation，但调度器无法通过 Label Selector 感知节点负载受损情况。

本方案在现有架构上进行最小侵入式扩展，在告警/故障条件上新增"是否影响节点负载"属性，并将该属性同步到节点 Label，供调度器及外部系统使用。

---

## 现有架构

```
priority.conf                    analysis.go                     device_aware.go
─────────────────               ─────────────────────────       ───────────────────────────
Condition:Priority     →   nodeOperateConfig map    →  NodeStatus.StatusMap
e.g. GpuDown:2             分桶: Emergency/CanIgnore        ↓
     IBPortDown:999         排序后传给 self-healing      node annotation
                                                       aegis.io/device-errors
                                                       (JSON: {"gpu":"0,1", "ib":"all"})
```

**缺失点**：没有任何地方标记"某个告警条件是否影响节点可调度负载"，调度器无法感知哪些节点因故障而不应接受新工作负载。

---

## 方案设计

### 1. 扩展 `priority.conf` 格式

将现有两列格式 `Condition:Priority` 扩展为三列 `Condition:Priority:AffectsLoad`，向后兼容（第三列缺省为 `false`）。

**语义定义：**
- `AffectsLoad: true` — 该故障会导致节点上部分或全部设备不可用，影响工作负载的实际执行（如 GPU 失效、IB 全断、存储不可挂载）
- `AffectsLoad: false` — 仅监控/告警，不影响工作负载调度（如端口降速告警、应用级 XID、温度预警）

**示例配置（部分）：**

```
# format: Condition:Priority:AffectsLoad

NodeNotReady:0:true
NodeCordon:1:true
NodeHasRestarted:100:false

# GPU
GpuDown:2:true                    # GPU 全部不可用，严重影响 GPU 型负载
GpuHung:100:true
GpuErrResetRequired:100:true
GpuCheckFailed:100:true           # 特定 GPU 失效，影响 GPU 负载
HighGpuTemp:100:false             # 温度告警，暂不影响调度
HighGpuMemoryTemp:100:false
GpuSmClkSlowDown:100:false
GpuPcieLinkDegraded:100:false
XIDApplicationErr:999:false
XIDECCMemoryErr:9:false           # ECC 内存错误，需修复但不立即下线
XIDHWSystemErr:9:true
GpuRegisterFailed:99:false
GpuMetricsHang:100:false

# IB / RoCE
IBLost:9:true                     # IB 网络全部断开，影响 RDMA 型负载
IBModuleLost:9:true
IBLinkAbnormal:9:true
IBNetDriverFailedLoad:9:true
IBPortDown:999:false              # 单端口，优先级低，不影响整体
IBPhysicalPortDown:999:false
IBPingFailed:999:false
IBRegisterFailed:99:false
RoceHostOffline:99:true
RoceDeviceBroken:99:true

# GPFS
GpfsDown:2:true                   # 存储全部不可用
GpfsMountLost:10:true             # 挂载点丢失，影响 IO 型负载
GpfsThreadDeadlock:11:true

# Network
NetworkLinkDown:50:true           # 节点网络断开

# Baseboard
BaseBoardCriticalIssue:50:true    # 硬件关键故障

# System resource
CPUPressure:99:true               # 资源压力影响新任务调度
MemoryPressure:99:true
```

---

### 2. `analysis.go` 变更

新增并行配置 map `nodeLoadImpactConfig`，在 `InitAnalysisConfig` 中解析第三列，并对外暴露 `IsLoadAffecting()` 查询接口。

```go
// 新增：负载影响配置 map
var nodeLoadImpactConfig map[string]bool = make(map[string]bool)

// InitAnalysisConfig 解析扩展的三列格式（向后兼容两列）
func InitAnalysisConfig(config string) error {
    // ...
    strs := strings.Split(line, ":")
    if len(strs) < 2 || len(strs) > 3 {
        return fmt.Errorf("invalid config format: %s", line)
    }

    pri, err := strconv.Atoi(strs[1])
    if err != nil {
        return fmt.Errorf("error conv priority %s: %s", strs[1], err)
    }
    nodeOperateConfig[strs[0]] = Priority(pri)

    // 解析第三列（可选，缺省 false）
    if len(strs) == 3 {
        affectsLoad, err := strconv.ParseBool(strs[2])
        if err != nil {
            return fmt.Errorf("error conv AffectsLoad %s: %s", strs[2], err)
        }
        nodeLoadImpactConfig[strs[0]] = affectsLoad
    }
    // ...
}

// IsLoadAffecting 返回指定故障条件是否影响节点负载
func IsLoadAffecting(condition string) bool {
    v, ok := nodeLoadImpactConfig[condition]
    return ok && v
}
```

---

### 3. `NodeStatus` 结构扩展

在 `node_condition_informer.go` 的 `NodeStatus` 中新增 `AffectsLoad` 字段：

```go
type NodeStatus struct {
    NodeName    string
    StatusMap   map[DeviceType]string
    Version     int64
    VersionMap  map[DeviceType]int64
    Timestamp   time.Time
    AffectsLoad bool   // 新增：当前节点是否存在影响负载的故障
}
```

---

### 4. `node_condition_informer.go` 中计算 `AffectsLoad`

各 `parseXxxStatus` 函数签名修改为返回 `(string, bool)`，在遍历条件时通过 `analysis.IsLoadAffecting()` 累积负载影响标志。

以 `parseGPUStatus` 为例：

```go
func parseGPUStatus(statuses []prom.AegisNodeStatus) (string, bool) {
    disabled := make([]bool, 8)
    affectsLoad := false

    for _, status := range statuses {
        if analysis.IsLoadAffecting(status.Condition) {
            affectsLoad = true
        }
        // 原有 disabled 数组逻辑不变 ...
        switch status.Condition {
        case string(basic.ConditionTypeGpuHung):
            // ...
        }
    }

    indexs := make([]string, 0)
    for i, disable := range disabled {
        if disable {
            indexs = append(indexs, fmt.Sprintf("%d", i))
        }
    }
    return strings.Join(indexs, ","), affectsLoad
}
```

`queryMetric` 在合并多设备结果时对 `AffectsLoad` 做 OR 聚合：

```go
// 在 fetchAllStatus 中合并时
existing.AffectsLoad = existing.AffectsLoad || status.AffectsLoad
```

---

### 5. `device_aware.go` 同步 Label

新增 Label 常量，在 `OnAdd/OnUpdate/OnDelete` 中将 `AffectsLoad` 同步到节点 Label：

```go
const (
    AEGIS_DEVICE_ANNOTATION   = "aegis.io/device-errors"     // 现有
    AEGIS_LOAD_AFFECTED_LABEL = "aegis.io/load-affected"     // 新增
)

func (h *NodeStatusHandler) OnAdd(new *NodeStatus) {
    klog.Infof("[NODE ADD/UPDATE] %s status: %v affectsLoad: %v", new.NodeName, new.StatusMap, new.AffectsLoad)

    node, err := h.nodeLister.Get(new.NodeName)
    // ...

    // 原有：同步 device-errors annotation
    value, err := json.Marshal(new.StatusMap)
    // ...
    node.Annotations[AEGIS_DEVICE_ANNOTATION] = string(value)

    // 新增：同步 load-affected label
    if node.Labels == nil {
        node.Labels = make(map[string]string)
    }
    if new.AffectsLoad {
        node.Labels[AEGIS_LOAD_AFFECTED_LABEL] = "true"
    } else {
        delete(node.Labels, AEGIS_LOAD_AFFECTED_LABEL)
    }

    _, err = h.kubeClient.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
    // ...
}

func (h *NodeStatusHandler) OnDelete(old *NodeStatus) {
    klog.Infof("[NODE DELETE] %s", old.NodeName)

    node, err := h.nodeLister.Get(old.NodeName)
    // ...

    // 原有：删除 annotation
    delete(node.Annotations, AEGIS_DEVICE_ANNOTATION)

    // 新增：删除 load-affected label
    delete(node.Labels, AEGIS_LOAD_AFFECTED_LABEL)

    _, err = h.kubeClient.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
    // ...
}
```

`InitDeviceStatusCache` 也需要在恢复缓存时读取节点当前的 `load-affected` label，以保证重启后状态一致：

```go
// InitDeviceStatusCache 中，恢复 AffectsLoad 字段
if labelVal, ok := node.Labels[AEGIS_LOAD_AFFECTED_LABEL]; ok {
    statuses[node.Name].AffectsLoad = labelVal == "true"
}
```

---

## 整体数据流

```
priority.conf（三列）
  Condition:Priority:AffectsLoad
        │
        ▼
analysis.InitAnalysisConfig()
  nodeOperateConfig[cond]    = Priority   （原有）
  nodeLoadImpactConfig[cond] = bool       （新增）
        │
        └─► analysis.IsLoadAffecting(cond) ◄── device-aware 查询
                    │
                    ▼
device_aware.queryMetric()
  parseXxxStatus(statuses) → (deviceStr, affectsLoad)
  fetchAllStatus 合并: AffectsLoad = OR(所有设备)
                    │
                    ▼
          NodeStatus.AffectsLoad = true/false
                    │
                    ▼
  NodeStatusHandler.OnAdd/OnUpdate
    node.Annotations["aegis.io/device-errors"] = JSON   （原有）
    node.Labels["aegis.io/load-affected"]      = "true" （新增，无故障时删除）
```

---

## 修改文件清单

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `internal/selfhealing/analysis/priority.conf` | 格式扩展 | 每行新增第三列 `:AffectsLoad`（true/false） |
| `internal/selfhealing/analysis/analysis.go` | 功能扩展 | 解析第三列；新增 `nodeLoadImpactConfig`；暴露 `IsLoadAffecting()` |
| `internal/device_aware/node_condition_informer.go` | 功能扩展 | `NodeStatus` 新增 `AffectsLoad`；各 `parseXxx` 返回 `(string, bool)`；聚合 `AffectsLoad` |
| `internal/device_aware/device_aware.go` | 功能扩展 | 新增 label 常量；`OnAdd/OnUpdate/OnDelete` 同步 label；`InitDeviceStatusCache` 恢复 label 状态 |

---

## 待定决策点

1. **config 格式**：直接扩展 `priority.conf` 三列 vs. 新建独立 `load-impact.conf`？
   - 三列方案：改动集中，但语义耦合
   - 独立文件：关注点分离，但需维护两份文件

2. **label value 策略**：仅在 `AffectsLoad=true` 时打 label 并在恢复后删除，vs. 始终保持 `"true"/"false"` 双值？
   - 删除方案：符合 Kubernetes 惯例，DoesNotExist 即健康
   - 双值方案：状态更明确，便于审计

3. **包依赖方向**：`device_aware` 直接 import `analysis` 包调用 `IsLoadAffecting()`，需确认无循环依赖（当前 `device_aware` 仅 import `basic` 和 `prom`）。
