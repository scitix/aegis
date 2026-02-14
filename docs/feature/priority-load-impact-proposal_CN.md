# 告警负载影响属性增强 —— 实现方案

## 背景

当前 Aegis 的优先级系统（`priority.conf` + `analysis.go`）仅能区分故障的处理优先级（Emergency/CanIgnore 等），但无法表达"某个故障是否影响节点的可用负载"。Device-Aware 控制器将设备状态同步到节点 Annotation，但调度器无法通过 Label Selector 感知节点负载受损情况。

此外，`node_condition_informer.go` 中的 `parseXxxStatus` 系列函数通过 switch-case 硬编码了每个故障条件的处理逻辑，导致两个问题：

1. **新增故障必须改代码**：`priority.conf` / ConfigMap 新增条件后，还需同步修改对应的 parse 函数才能被 device-aware 感知。
2. **两套独立的优先级加载逻辑**：`nodepoller` 中的 `PriorityWatcher` 已实现 ConfigMap watch + 热重载，而 `device_aware` 仍使用静态文件加载（`analysis.InitAnalysisConfig`），功能重复且 device_aware 无法热更新。

本方案在现有架构上进行最小侵入式扩展，同时解决以上三个问题。

---

## 现有架构

```
priority.conf / ConfigMap
  Condition:Priority
        │
        ├─► analysis.InitAnalysisConfig()      (device_aware 静态加载，无热重载)
        │     nodeOperateConfig[cond] = Priority
        │
        └─► nodepoller.PriorityWatcher          (ConfigMap watch + 热重载)
              config[cond] = Priority
              IsCritical(cond) / IsCordon(cond)

node_condition_informer.go
  queryMetric(type) → parseGPUStatus(statuses)
                        switch condition {          ← 硬编码每个条件
                        case GpuHung: all disabled
                        case GpuCheckFailed: bitmask
                        case GpuNvlinkError: index
                        case GpuRegisterFailed: continue  ← 明确忽略
                        default: Warning (新条件到这里)
                        }
```

**缺失点**：
- 没有"是否影响负载"的属性，调度器无法感知。
- parse 函数每次新增条件都需要人工介入。
- device_aware 和 nodepoller 的优先级配置是两条独立路径。

---

## 方案设计

### 1. 扩展配置格式为四列

将现有两列格式扩展为四列，向后兼容（第三、四列缺省时取安全默认值）。

```
# 格式：Condition:Priority:AffectsLoad:DeviceIDMode
```

**`AffectsLoad`**（bool，缺省 `false`）：
- `true`：该故障导致部分或全部设备不可用，影响工作负载实际执行。
- `false`：仅告警/监控，不影响调度（温度告警、应用级 XID、端口降速等）。

**`DeviceIDMode`**（缺省 `-`）：控制该条件在 `StatusMap` 中如何标记受影响的设备实例：

| 值 | 语义 | 适用场景 |
|---|---|---|
| `all` | 忽略 ID，将该类型全部设备标记为故障 | GpuHung、IBLost、GpfsDown 等整体性故障 |
| `index` | `status.ID` 为整数索引，标记对应编号的设备 | 大多数单卡 GPU 故障（XID、NVLink 等） |
| `mask` | `status.ID` 为位串（如 `"10100000"`），按位标记各设备 | GpuCheckFailed |
| `id` | `status.ID` 为设备标识符字符串（卡名、路径等） | IB、RoCE、GPFS 挂载点等 |
| `-` | 不标记任何设备（明确忽略） | GpuRegisterFailed、MetricsHang 等 |

**示例配置（节选）：**

```
# Condition:Priority:AffectsLoad:DeviceIDMode

NodeNotReady:0:true:-
NodeCordon:1:true:-
NodeHasRestarted:100:false:-

# GPU
GpuDown:2:true:all
GpuHung:100:true:all
GpuErrResetRequired:100:true:all
GpuP2PNotSupported:100:true:all
GPUIbgdaNotEnabled:100:true:all
GpuCheckFailed:100:true:mask
GpuNvlinkInactive:9:true:index
GpuNvlinkError:9:true:index
GpuTooManyPageRetired:9:true:index
GpuAggSramUncorrectable:9:true:index
GpuVolSramUncorrectable:9:true:index
GpuSmClkSlowDown:100:false:index
GpuGpuHWSlowdown:100:false:index
GpuPcieLinkDegraded:100:false:index
HighGpuTemp:100:false:-
HighGpuMemoryTemp:100:false:-
XIDApplicationErr:999:false:-
XIDECCMemoryErr:9:false:-
XIDHWSystemErr:9:true:index
Xid95UncontainedECCError:9:true:index
Xid64ECCRowremapperFailure:9:true:index
Xid74NVLinkError:9:true:index
Xid79GPULost:9:true:index
Xid48GPUMemoryDBE:9:false:-
Xid63ECCRowremapperPending:9:false:-
GpuRegisterFailed:99:false:-
GpuMetricsHang:100:false:-
GpuRowRemappingFailure:9:false:-

# IB / RoCE
IBLost:9:true:all
IBModuleLost:9:true:all
IBLinkAbnormal:9:true:id
IBNetDriverFailedLoad:9:true:id
IBPCIeMRRNotAlign:9:true:id
IBPortSpeedAbnormal:9:false:id
IBPCIeSpeedAbnormal:9:false:id
IBPCIeWidthAbnormal:9:false:id
IBProtoclAbnormal:9:false:id
IBLinkFrequentDown:9:false:id
IBPortDown:999:false:-
IBPhysicalPortDown:999:false:-
IBPingFailed:999:false:-
IBRegisterFailed:99:false:-
RoceHostOffline:99:true:all
RoceDeviceBroken:99:true:all
RoceHostGatewayNotMatch:99:true:all
RoceHostRouteMiss:99:true:all
RocePodOffline:99:true:all
RocePodGatewayNotMatch:99:true:all
RocePodRouteMiss:99:true:all
RoceNodeLabelMiss:99:true:all
RocePodDeviceMiss:99:true:all
RoceVfDeviceMiss:99:true:id
RoceNodeResourceMiss:99:true:id
RoceSriovInitError:99:true:id
RoceNodeUnitLabelMiss:99:true:all
RoceNodePfNamesLabelMiss:99:true:all
RoceNodeResourceLabelMiss:99:true:all
RoceNodeNetworkLabelMiss:99:true:all
RoceRegisterFailed:99:false:id

# GPFS
GpfsDown:2:true:all
GpfsMountLost:10:true:id
GpfsThreadDeadlock:11:true:all

# Network
NetworkLinkDown:50:true:id

# Baseboard
BaseBoardCriticalIssue:50:true:id

# System resource
CPUPressure:99:true:id
MemoryPressure:99:true:id
```

---

### 2. `analysis.ParsePriorityConfig` 扩展

新增 `ConditionConfig` 结构，`ParsePriorityConfig` 解析四列并返回完整配置，保持向后兼容（三、四列缺省时取安全默认值）：

```go
// ConditionConfig 保存单个故障条件的完整配置
type ConditionConfig struct {
    Priority    Priority
    AffectsLoad bool
    DeviceIDMode string // "all" / "index" / "mask" / "id" / "-"
}

// ParseConditionConfig 解析四列格式，兼容旧的两列格式
func ParseConditionConfig(content string) (map[string]ConditionConfig, error) {
    result := make(map[string]ConditionConfig)
    scanner := bufio.NewScanner(strings.NewReader(content))
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        strs := strings.Split(line, ":")
        if len(strs) < 2 || len(strs) > 4 {
            return nil, fmt.Errorf("invalid config format: %s", line)
        }

        pri, err := strconv.Atoi(strings.TrimSpace(strs[1]))
        if err != nil {
            return nil, fmt.Errorf("error conv priority %s: %s", strs[1], err)
        }

        cfg := ConditionConfig{
            Priority:     Priority(pri),
            AffectsLoad:  false,
            DeviceIDMode: "-",
        }

        if len(strs) >= 3 {
            cfg.AffectsLoad, err = strconv.ParseBool(strings.TrimSpace(strs[2]))
            if err != nil {
                return nil, fmt.Errorf("error conv AffectsLoad %s: %s", strs[2], err)
            }
        }
        if len(strs) == 4 {
            cfg.DeviceIDMode = strings.TrimSpace(strs[3])
        }

        result[strings.TrimSpace(strs[0])] = cfg
    }
    return result, nil
}

// ParsePriorityConfig 保持原有签名不变，内部调用 ParseConditionConfig
func ParsePriorityConfig(content string) (map[string]Priority, error) {
    full, err := ParseConditionConfig(content)
    if err != nil {
        return nil, err
    }
    result := make(map[string]Priority, len(full))
    for k, v := range full {
        result[k] = v.Priority
    }
    return result, nil
}
```

---

### 3. `PriorityWatcher` 提升为共享依赖并扩展

将 `PriorityWatcher` 的构造函数导出，新增 `IsLoadAffecting` 和 `GetIDMode` 接口，`reload` 同时解析四列：

```go
// nodepoller/priority.go

type PriorityWatcher struct {
    configs map[string]analysis.ConditionConfig
    mu      sync.RWMutex
}

// NewPriorityWatcher 导出，供 aegis.go 统一创建后注入
func NewPriorityWatcher() *PriorityWatcher {
    return &PriorityWatcher{
        configs: make(map[string]analysis.ConditionConfig),
    }
}

func (w *PriorityWatcher) IsCritical(condition string) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    c, ok := w.configs[condition]
    return ok && c.Priority <= analysis.Emergency
}

func (w *PriorityWatcher) IsCordon(condition string) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    c, ok := w.configs[condition]
    return ok && c.Priority == analysis.NodeCordon
}

// IsLoadAffecting 返回该条件是否影响节点负载（新增）
func (w *PriorityWatcher) IsLoadAffecting(condition string) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    c, ok := w.configs[condition]
    return ok && c.AffectsLoad
}

// GetIDMode 返回该条件的设备 ID 解析模式（新增）
func (w *PriorityWatcher) GetIDMode(condition string) string {
    w.mu.RLock()
    defer w.mu.RUnlock()
    c, ok := w.configs[condition]
    if !ok {
        return "-" // 未知条件：保守，不标记任何设备
    }
    return c.DeviceIDMode
}

func (w *PriorityWatcher) reload(data map[string]string) {
    content, ok := data["priority"]
    if !ok {
        klog.Warningf("nodepoller: priority ConfigMap has no key \"priority\", skipping reload")
        return
    }

    parsed, err := analysis.ParseConditionConfig(content)
    if err != nil {
        klog.Errorf("nodepoller: failed to parse priority config: %v", err)
        return
    }

    w.mu.Lock()
    w.configs = parsed
    w.mu.Unlock()
    klog.V(4).Infof("nodepoller: priority config reloaded (%d entries)", len(parsed))
}
```

---

### 4. `aegis.go` 统一创建 `PriorityWatcher` 并注入两个控制器

`PriorityWatcher` 实例在 `NewAegisController` 中创建一次，分别注入 `nodeStatusPoller` 和 `deviceawareController`。ConfigMap Watch 由 nodePoller 的 `Run` 负责启动，device_aware 被动消费，无需自己 watch：

```go
// internal/controller/aegis.go NewAegisController 中

priorityWatcher := nodepoller.NewPriorityWatcher()

nodePoller := nodepoller.NewNodeStatusPoller(
    prometheus,
    alertInterface,
    nodeInformer.Lister(),
    pollerCfg,
    priorityWatcher,   // 注入
)

deviceawareController, err := deviceaware.NewController(
    cfg.Client,
    nodeInformer,
    prometheus,
    priorityWatcher,   // 注入，取代原 analysis.InitAnalysisConfig
)
```

---

### 5. `NodeStatus` 新增 `AffectsLoad` 字段

```go
// node_condition_informer.go

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

### 6. `parseXxxStatus` 变为数据驱动

`parseXxxStatus` 系列函数接收 `PriorityWatcher`，通过 `GetIDMode` 分支替代 switch-case 中的条件枚举。以 `parseGPUStatus` 为例，这是改动最大的函数：

```go
// 改造前：约 80 行 switch-case，每个条件硬编码
// 改造后：按 IDMode 分支，新条件只需改 ConfigMap

func parseGPUStatus(statuses []prom.AegisNodeStatus, watcher *PriorityWatcher) string {
    disabled := make([]bool, 8)

    for _, status := range statuses {
        switch watcher.GetIDMode(status.Condition) {
        case "all":
            for i := range disabled {
                disabled[i] = true
            }
        case "mask":
            for i, ch := range status.ID {
                if b, err := strconv.ParseBool(string(ch)); err == nil {
                    disabled[i] = b
                } else {
                    klog.Warningf("parse gpu mask index %d char %c failed: %s", i, ch, err)
                }
            }
        case "index":
            if status.ID != "" {
                id, err := strconv.Atoi(status.ID)
                if err != nil || id >= len(disabled) {
                    klog.Warningf("parse gpu index %q failed or out of range: %v", status.ID, err)
                } else {
                    disabled[id] = true
                }
            }
        case "-", "":
            // 明确忽略，不标记任何 GPU
        default:
            klog.Warningf("unknown DeviceIDMode %q for condition %s", watcher.GetIDMode(status.Condition), status.Condition)
        }
    }

    indexes := make([]string, 0)
    for i, d := range disabled {
        if d {
            indexes = append(indexes, strconv.Itoa(i))
        }
    }
    return strings.Join(indexes, ",")
}
```

其他 parse 函数（IB、RoCE、GPFS、CPU、Memory 等）只使用 `id` / `all` / `-` 三种模式，改造更简单：

```go
func parseIBDeviceStatus(statuses []prom.AegisNodeStatus, watcher *PriorityWatcher) string {
    disabledMap := make(map[string]bool)
    for _, status := range statuses {
        switch watcher.GetIDMode(status.Condition) {
        case "id":
            if status.ID != "" {
                disabledMap[status.ID] = true
            }
        case "all":
            disabledMap[defaultDeviceId] = true
        case "-", "":
            // 忽略
        }
    }
    // ...
}
```

---

### 7. `AffectsLoad` 在原始 metrics 层计算

在 `queryMetric` 中，parse 函数调用前遍历原始条件计算 `AffectsLoad`，结果写入返回的 `NodeStatus`：

```go
func (i *NodeStatusInformer) queryMetric(ctx context.Context, deviceType DeviceType) (map[string]*NodeStatus, error) {
    // ... 原有查询逻辑 ...

    for node, statuses := range nodeStatuses {
        // 计算 AffectsLoad（新增）
        affectsLoad := false
        for _, s := range statuses {
            if i.priorityWatcher.IsLoadAffecting(s.Condition) {
                affectsLoad = true
                break
            }
        }

        // 调用数据驱动的 parse 函数（签名加 watcher 参数）
        var s string
        switch deviceType {
        case DeviceTypeGPU:
            s = parseGPUStatus(statuses, i.priorityWatcher)
        case DeviceTypeIB:
            s = parseIBDeviceStatus(statuses, i.priorityWatcher)
        // ... 其他 case 不变 ...
        }

        if s != "" || affectsLoad {
            if _, exists := statusMap[node]; !exists {
                statusMap[node] = &NodeStatus{
                    NodeName:  node,
                    StatusMap: make(map[DeviceType]string),
                    Timestamp: time.Now(),
                }
            }
            if s != "" {
                statusMap[node].StatusMap[deviceType] = s
            }
            // AffectsLoad 在 fetchAllStatus 中 OR 聚合（见下）
            statusMap[node].AffectsLoad = statusMap[node].AffectsLoad || affectsLoad
        }
    }
    // ...
}
```

`fetchAllStatus` 合并多设备结果时对 `AffectsLoad` 做 OR 聚合：

```go
// fetchAllStatus 合并时（已有的合并逻辑中补充）
existing.AffectsLoad = existing.AffectsLoad || status.AffectsLoad
```

---

### 8. `device_aware.go` 同步 Label

新增 Label 常量，`OnAdd/OnUpdate/OnDelete` 将 `AffectsLoad` 同步到节点 Label，无故障时删除（符合 Kubernetes `DoesNotExist` 惯例）：

```go
const (
    AEGIS_DEVICE_ANNOTATION   = "aegis.io/device-errors"   // 现有
    AEGIS_LOAD_AFFECTED_LABEL = "aegis.io/load-affected"   // 新增
)

func (h *NodeStatusHandler) OnAdd(new *NodeStatus) {
    // ... 原有 annotation 逻辑不变 ...

    // 同步 load-affected label
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
    // ... 原有 annotation 删除不变 ...
    if node.Labels != nil {
        delete(node.Labels, AEGIS_LOAD_AFFECTED_LABEL)
    }
    _, err = h.kubeClient.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
    // ...
}
```

`InitDeviceStatusCache` 重启后从节点 Label 恢复 `AffectsLoad` 状态：

```go
if labelVal, ok := node.Labels[AEGIS_LOAD_AFFECTED_LABEL]; ok {
    statuses[node.Name].AffectsLoad = labelVal == "true"
}
```

---

## 整体数据流

```
ConfigMap: aegis-priority
  Condition:Priority:AffectsLoad:DeviceIDMode
        │
        ▼ watch（nodepoller 启动，device_aware 被动消费）
PriorityWatcher（单例，aegis.go 创建后注入两处）
  IsCritical(cond)      → nodepoller 分类
  IsCordon(cond)        → nodepoller 分类
  IsLoadAffecting(cond) → device_aware AffectsLoad 计算
  GetIDMode(cond)       → device_aware parseXxxStatus 逻辑分支
        │
        ├──────────────────────────────────┐
        ▼                                  ▼
nodepoller.classify()            device_aware.queryMetric()
  告警触发逻辑（现有）              for each raw condition:
                                    affectsLoad |= IsLoadAffecting(c)
                                  parseXxxStatus(statuses, watcher):
                                    switch GetIDMode(condition):
                                      "all"   → 全部设备
                                      "index" → 按整数 ID
                                      "mask"  → 按位串
                                      "id"    → 按字符串 ID
                                      "-"     → 忽略
                                        │
                                        ▼
                                  NodeStatus{
                                    StatusMap:   map[DeviceType]string
                                    AffectsLoad: bool
                                  }
                                        │
                                        ▼
                              NodeStatusHandler.OnAdd/OnUpdate
                                node.Annotations["aegis.io/device-errors"] （现有）
                                node.Labels["aegis.io/load-affected"]      （新增）
```

---

## 修改文件清单

| 文件 | 变更类型 | 变更内容 |
|------|----------|----------|
| `internal/selfhealing/analysis/priority.conf` | 格式扩展 | 每行新增第三、四列 `:AffectsLoad:DeviceIDMode` |
| `internal/selfhealing/analysis/analysis.go` | 功能扩展 | 新增 `ConditionConfig` 结构和 `ParseConditionConfig()`；`ParsePriorityConfig` 保持签名不变，内部复用 |
| `internal/controller/nodepoller/priority.go` | 重构扩展 | 构造函数 `NewPriorityWatcher()` 导出；内部改用 `ConditionConfig` map；新增 `IsLoadAffecting()` 和 `GetIDMode()`；`reload` 调用 `ParseConditionConfig` |
| `internal/controller/nodepoller/poller.go` | 接口调整 | `NewNodeStatusPoller` 接收外部 `*PriorityWatcher` 参数（原为内部 `newPriorityWatcher()`） |
| `internal/device_aware/node_condition_informer.go` | 重构 | `NodeStatus` 加 `AffectsLoad`；构造函数注入 `*PriorityWatcher`；`queryMetric` 计算 `AffectsLoad`；各 `parseXxxStatus` 增加 `watcher` 参数，switch-case 改为 `GetIDMode` 分支 |
| `internal/device_aware/device_aware.go` | 功能扩展 | 新增 label 常量；`OnAdd/OnUpdate/OnDelete` 同步 label；`InitDeviceStatusCache` 恢复 label 状态 |
| `internal/controller/aegis.go` | 组装调整 | `NewAegisController` 创建 `PriorityWatcher` 单例，注入 `nodePoller` 和 `deviceawareController` |

---

## 新增故障条件的操作流程（改造后）

改造完成后，新增一个故障条件只需：

1. 在 ConfigMap（或 `priority.conf`）中增加一行，填写四列属性
2. 无需修改任何 Go 代码
3. 若 device_aware 已运行，PriorityWatcher 热重载后下一个 10s 周期自动生效
