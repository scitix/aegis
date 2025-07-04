# AI HPC 节点自愈（AI HPC Node Self‑healing）

Aegis 自愈系统针对 AI HPC 环境中常见的节点故障提供**实时检测、优先级评估与 SOP‑驱动的自动修复**能力，实现“无人值守、秒级修复”目标。


## ✨ 功能概览

1. **周期巡检**：守护进程定时拉取节点状态（Exporter / PromQL），构建 `AegisNodeStatus`。
2. **异常检测**：基于 Condition 规则及阈值识别硬件、系统与容器异常。
3. **优先级计算**：将所有 Condition 映射到四大优先级队列，确保最紧急问题先处理。
4. **SOP 调度**：按优先级选择合适的 **SOP 插件**，执行自愈动作。
5. **工单追踪**：通过 *Node Ticketing* Annotation 持续记录操作步骤与结果。


## 🏗️ SOP 插件架构

```go
// 核心接口

type SOP interface {
    CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error // 初始化

    Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool // 真伪评估

    Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error // 自愈动作
}
```

* **插件化**：每类故障一个独立 Go 包，热插拔、易扩展。
* **幂等性**：每次执行前通过 `Evaluate` 再确认，避免重复修复。
* **三阶段**：`CreateInstance`→`Evaluate`→`Execute`。

> **已覆盖领域**：Node / CPU / Disk / GPFS / GPU / IB / Network / Memory / Process / PeerMem / System。


## 📜 Condition & SOP 列表（节选）

| 分类          | 代表 Condition（节选）                                                               |
| ----------- | ------------------------------------------------------------------------------ |
| **Node**    | NodeNotReady · NodeCordon · NodeFrequentDown · KubeletFailedCreatePodContainer |
| **CPU**     | CPUPressure · CpuUnhealthy                                                     |
| **Disk**    | DiskUnhealthy                                                                  |
| **GPFS**    | GpfsDown · GpfsMountLost · GpfsInactive                                        |
| **GPU**     | GpuHung · GpuDown · XIDECCMemoryErr · GpuNvlinkInactive                        |
| **IB**      | IBDown · IBLinkFrequentDown                                                    |
| **Network** | NetworkLinkFrequentDown · ICETxTimeout                                         |
| **Memory**  | MemoryPressure · KubeletMemoryPressure                                         |
| **System**  | KernelPanic · HighLoad                                                         |



## 🔀 优先级队列划分

| 优先级区间                    | 队列名称               | 说明            |
| ------------------------ | ------------------ | ------------- |
| `== NodeNotReady`        | **NotReady**       | 最紧急：节点不可用     |
| `== NodeCordon`          | **Cordon**         | 已被手动隔离，需要快速处理 |
| `(1, Emergency]`         | **EmergencyList**  | 影响计算严重，需要立即修复 |
| `(Emergency, CanIgnore]` | **CanIgnoreList**  | 可暂时容忍的小问题     |
| `> CanIgnore`            | **MustIgnoreList** | 明确配置为“必须忽略”   |

队列由 **调度器** 每轮循环动态重排，确保高优告警优先触发 SOP。


## 🗂️ Node Ticketing Annotation 规范

Aegis 通过节点 Annotation `aegis.io/ticketing` 记录故障上下文与操作流水，示例：

```yaml
# kubectl annotate node dev1 aegis.io/ticketing='|
  condition: GPUCheckFailed
  reason: too many reboot
  supervisor: alice,bob
  status: resolving
  workflows:
    - action: cordon
      status: Succeeded
    - action: healthcheck
      status: Succeeded
    - action: reboot
      status: Failed'
```

> **提示**：字段尽量精简，方便 Annotation 长度受限场景。

## 🐳 自愈镜像与统一入口脚本

> **镜像目录**：`/selfhealing/job/*.yaml`（Job 模板）
>
> **命名空间**：`monitoring`

### 关键 Job 模板

```
restart_node.yaml   # 重新启动节点
shutdown_node.yaml  # 关机
healthcheck_node.yaml
repair_node.yaml
remedy_node.yaml
```


## 📈 运行监控与指标

| 指标                        | 含义                      |
| ------------------------- | ----------------------- |
| `aegis_sop_total{status}` | SOP 执行次数（按结果细分）         |
| `aegis_selfheal_seconds`  | 自愈流程整体耗时                |
| `aegis_ticket_open_total` | 当前 open 状态 Ticket 数     |
| `aegis_condition_gauge`   | 各 Condition 在集群中的活跃节点数量 |

通过 Grafana Dashboard 可视化修复速度与成功率，持续优化 SOP 策略。
