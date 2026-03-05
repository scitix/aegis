# 自愈工单决策设计说明

## 概述

自愈组件在处理节点故障时，需要判断"这个工单是否该自己处理"。本文档描述了完整的决策流程及工单归属判定逻辑。

## 整体流程

```
selfhealing node <node-name>
        |
        v
  [1] precheck: 节点是否有指定 Taint
        |
       通过
        v
  [2] 是否禁用自愈 (aegis.io/disable=true)
        |
       未禁用
        v
  [3] 查询 Prometheus 获取节点状态
        |
        v
  [4] 按优先级分析、选定要处理的问题
        |
       有问题
        v
  [5] 工单归属判定 (CanDealWithTicket)
        |
       可处理
        v
  [6] SOP Evaluate / GateKeeper / Master 节点检查
        |
       全部通过
        v
  [7] 执行 SOP
```

## 阶段说明

### 阶段 1: Precheck (Taint 前置检查)

检查节点是否有 `AEGIS_PRECHECK_TAINTS` 环境变量中指定的 Taint。如果有，移除该 Taint 并 cordon 节点，然后直接退出。不进入后续自愈流程。

### 阶段 2: 自愈开关

检查节点是否有标签 `aegis.io/disable=true`。如果节点禁用了自愈：
- 将已有工单转派给 SRE (`DispatchTicketToSRE`)
- 直接退出

### 阶段 3-4: 问题分析与优先级排序

通过 Prometheus 查询节点状态指标，然后按 `priority.conf` 配置文件中的优先级分类：

| 优先级值 | 分类 | 含义 |
|---------|------|------|
| 0 | NodeNotReady | 节点不就绪，最高优先级 |
| 1 | NodeCordon | 节点已 cordon |
| 2-99 | Emergency | 紧急问题，需要立即处理 |
| 100-999 | CanIgnore | 可忽略的问题 |
| >999 | MustIgnore | 必须忽略，不做处理 |

从所有问题中按以下优先级选取 **一个** 最需要处理的问题：

1. NodeNotReady 最优先
2. NodeCordon（仅在没有紧急问题时）
3. Emergency 列表中优先级值最小的

如果没有可处理的问题，直接退出。

### 阶段 5: 工单归属判定（核心决策）

这是决定 aegis 是否自行处理工单的核心逻辑。

#### CanDealWithTicket 判定规则

所有工单系统实现统一的判定逻辑：

```go
func CanDealWithTicket(ctx context.Context) bool {
    return ticket == nil || ticket.Supervisor == "" || ticket.Supervisor == user
}
```

满足以下任一条件即可直接处理：
- 工单不存在（新问题）
- 工单的 Supervisor 为空
- 工单的 Supervisor 是 aegis 自己

#### 完整决策树

```
                    工单存在？
                   /         \
                 否            是
                 |              |
             直接处理      Supervisor 为空或是 aegis？
                           /              \
                         是                否 (工单属于其他人, 如 SRE)
                         |                  |
                     直接处理         旧工单的 SOP 可被抢占？ [注1]
                                     /          \
                                   是            否
                                   |              |
                            新问题的 SOP      启用了 --ticket.claim？
                            也是可抢占的？      /       \
                            /        \       是        否
                          是          否      |         |
                          |           |    认领工单     放弃
                     放弃(同级)   删除旧工单  (Adopt)   不做处理
                     不做处理    继续处理     继续处理
```

**[注1] 抢占机制 (Preempt)**

部分低严重度 SOP 实现了 `PreemptableSOP` 接口，允许更高严重度的新问题抢占正在处理的旧工单。抢占的条件：

1. 旧工单对应的 SOP 实现了 `PreemptableSOP` 接口且 `IsPreemptable()` 返回 `true`
2. 新问题对应的 SOP **不是** 可抢占的（避免同层级互相抢占）

当前可被抢占的 SOP：`baseboard`、`network`

#### 认领机制 (Claim / Adopt)

通过启动参数 `--ticket.claim=true` 启用。当工单属于其他负责人（如 SRE）且不满足抢占条件时，aegis 会尝试认领该工单，将 Supervisor 修改为自己，然后继续处理。

### 阶段 6: 后续门控

通过工单归属判定后，还有三道检查：

1. **SOP Evaluate**: SOP 自身的真实性评估，确认告警确实存在
2. **GateKeeper**: 如果 SOP 需要 cordon 节点，GateKeeper 做全局准入控制（避免同时 cordon 太多节点）
3. **Master 节点检查**: Master 节点不执行自愈操作

## 工单系统

自愈组件支持四种工单系统后端，通过 `--ticket.system` 参数选择：

| 系统 | 参数值 | 存储方式 | 适用场景 |
|------|--------|---------|---------|
| None | `None` | 无存储 | 测试/无需工单记录 |
| Node | `Node` | Node Annotation (`aegis.io/ticketing`) | 轻量级，无需外部依赖 |
| Scitix | `Scitix` | OP 工单系统 API | Scitix 内部环境 |
| UCP | `UCP` | UCP 工单系统 API | UCP 平台环境 |

所有工单系统均实现 `TicketManagerInterface` 接口，`CanDealWithTicket` 判定逻辑一致。

## 工单生命周期

```
Created → Assigned → Resolving → Resolved → Closed
```

典型的自愈工单流转：

1. aegis 发现问题，通过 `CanDealWithTicket` 判定可以处理
2. SOP 执行过程中创建工单 (`CreateTicket`)，Supervisor 设为 `aegis`
3. SOP 记录根因 (`AddRootCauseDescription`)、执行步骤 (`AddWorkflow`)
4. 修复成功：解决工单 (`ResolveTicket`)
5. 修复失败或需人工介入：转派给 SRE (`DispatchTicketToSRE`)

## 相关代码

| 文件 | 说明 |
|------|------|
| `selfhealing/node/node.go` | 自愈入口和决策流程 |
| `pkg/ticketmodel/interface.go` | 工单管理接口定义 |
| `pkg/ticketmodel/types.go` | 工单数据结构和工作流类型 |
| `internal/selfhealing/ticket/manager.go` | 工单系统工厂 |
| `pkg/nodeticket/manager.go` | Node Annotation 工单实现 |
| `pkg/opticket/manager.go` | OP 工单系统实现 |
| `pkg/uticket/manager.go` | UCP 工单系统实现 |
| `pkg/noneticket/manager.go` | 空实现（无工单） |
| `internal/selfhealing/node_sop/registry.go` | SOP 注册表和 PreemptableSOP 接口 |
| `internal/selfhealing/analysis/analysis.go` | 节点状态优先级分析 |
