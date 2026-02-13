# Pod 日志关键字过滤优化 Proposal

## 现状分析

`getContainerLogs`（`pkg/analyzer/pod.go:287`）当前行为：
```go
podLogOpts.TailLines = &[]int64{int64(60)}[0]  // 硬编码取最近 60 行
```
60 行直接送给 LLM，无任何过滤——对于日志量大的容器，60 行往往是大量正常输出，真正的异常信息反而被稀释。

---

## 目标

1. **拉取更多日志**：每次拉取最近 **1000 行**（可配置）
2. **关键字过滤**：对 1000 行按可配置关键字列表做行级匹配，筛出异常行
3. **裁剪至上限**：从过滤结果中取最近 **N 行**（默认 60，可配置）
4. **不足时补齐**：关键字行不足 N 条时，用最近的原始日志补齐至 N 条

---

## 配置结构

在 `common.Analyzer` 中增加一个 `PodLogConfig` 字段（对调用方完全向后兼容，`nil` 时退回原有行为）：

```go
// pkg/analyzer/common/types.go
type PodLogConfig struct {
    FetchLines     int      // 拉取行数，默认 1000
    Keywords       []string // 关键字列表（大小写不敏感子串匹配）
    MaxOutputLines int      // 最终输出行数上限，默认 60
}

type Analyzer struct {
    kcommon.Analyzer
    Name           string
    CollectorImage string
    EnableProm     bool
    EnablePodLog   *bool
    PodLogConfig   *PodLogConfig  // 新增；nil → 原有行为不变
    Owner          metav1.Object
}
```

---

## 过滤算法

### 三步流程

```
raw[0..999]  ← 按时间顺序排列（999 = 最新）

Step 1: 关键字匹配
  matched_idx = [ i ∈ [0,999] | raw[i] 含任意关键字 ]

Step 2: 取最近 MaxOutputLines 条关键字行
  keyword_idx = tail(matched_idx, MaxOutputLines)   // ≤60 个下标

Step 3: 按需补齐
  deficit = MaxOutputLines - len(keyword_idx)
  if deficit > 0:
    recent_idx = tail([0..999] \ keyword_idx, deficit)  // 最近 deficit 条非关键字行
    final_idx  = sort(keyword_idx ∪ recent_idx)         // 按原始行号升序，保留时间顺序
  else:
    final_idx  = keyword_idx

output = [ raw[i] for i in final_idx ]   // ≤60 行，时间顺序
```

### 三种场景

| 场景 | 关键字命中 | 补齐行 | 输出 |
|---|---|---|---|
| 关键字行充足 | ≥60 | 0 | 最近 60 条关键字行 |
| 关键字行不足 | k < 60 | 60-k 条最近非关键字行 | k 条关键字行 + (60-k) 条最近行，时间有序 |
| 无关键字命中 | 0 | 60 条最近行 | 退化为原有行为 |

### 示意图（MaxOutputLines=5 简化版）

```
raw (10行):
  0: INFO  boot
  1: INFO  ready
  2: ERROR oom             ← keyword
  3: INFO  serving
  4: INFO  request ok
  5: WARN  slow response
  6: ERROR timeout         ← keyword
  7: INFO  retry
  8: INFO  ok
  9: PANIC nil deref       ← keyword

keyword_idx = [2, 6, 9]   → tail(3) → [2, 6, 9]   (3条 < 5)
deficit = 2
recent_idx  = tail([0..9] \ {2,6,9}, 2) = [7, 8]

final_idx = sort({2,6,7,8,9}) = [2, 6, 7, 8, 9]

output:
  2: ERROR oom
  6: ERROR timeout
  7: INFO  retry           ← 补齐：提供关键字行前后的最新上下文
  8: INFO  ok
  9: PANIC nil deref
```

补齐行取的是**最新的**非关键字行，而不是关键字行的上下文邻行——这样 LLM 既能看到所有异常，也能看到崩溃发生前的最新状态。

---

## 关键字匹配规则

- `strings.Contains(strings.ToLower(line), strings.ToLower(keyword))` — 大小写不敏感子串匹配
- 关键字列表为空 `[]` 时：不过滤，直接对 raw tail 取 MaxOutputLines（退化为原有语义）

### 建议默认关键字集合

```
error, fatal, panic, exception, oom, killed, segfault,
traceback, crash, timeout, failed, refused
```

---

## 配置来源

关键字列表从 diagnosis controller 的配置（ConfigMap 或启动参数）读入，传给 `common.Analyzer.PodLogConfig`，不需要 ConfigMap watcher（诊断是一次性触发，启动时加载即可）。

---

## 文件改动

| 文件 | 变更 |
|---|---|
| `pkg/analyzer/common/types.go` | 新增 `PodLogConfig` 结构体；`Analyzer` 增加 `PodLogConfig *PodLogConfig` 字段 |
| `pkg/analyzer/pod.go` | `getContainerLogs` 接收 `*PodLogConfig`，实现 fetch→filter→tail 三步；新增 `filterByKeywords` 纯函数 |
| diagnosis controller 调用侧 | 构造 `Analyzer` 时填充 `PodLogConfig`，从控制器配置读关键字 |

---

## 向后兼容

- `PodLogConfig == nil`：`FetchLines=60`，不过滤，行为与现在完全一致
- 关键字列表为空 `[]`：拉 1000 行后直接 tail(MaxOutputLines)，无过滤
