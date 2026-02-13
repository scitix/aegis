# PyTorchJob 诊断改造 Proposal

## 现状问题

### 1. Job Condition 分析不可靠

`pytorchjob.go` 当前直接取 `conds[len(conds)-1]` 作为当前状态：

```go
last := conds[len(conds)-1]
result.Metadata["JobStatus"] = string(last.Type)
```

kubeflow 的 `Conditions []JobCondition` 是**多个 condition 同时存在**的数组，一个 Failed 的 Job 状态如下：

```
[{Created, True}, {Running, False}, {Failed, True}]
```

**问题 a**：取最后一个元素依赖 kubeflow 内部追加顺序，`last.Status` 完全未检查。当 Job 从 Running 向 Failed 过渡时，`last` 可能是 `{Running, False}`，命中 `default` 分支产生误导性 Warning。

**问题 b**：`Restarting` 和 `Suspended` 两个 condition 类型（已在 `common_types.go` 定义）在 switch 里未覆盖，一律落 default。

### 2. 子 Pod 分析字段未透传

`analyzePytorchJobPods` 调用子 `PodAnalyzer.Analyze()` 时缺失四个字段：

| 缺失字段 | 影响 |
|----------|------|
| `PodLogConfig` | keyword 过滤失效，走 legacy 60 行路径 |
| `EnablePodLog` | 日志开关被忽略 |
| `EnableProm` | Prometheus 事件获取失效 |
| `AIClient` | 无法做 Pod 级 LLM 诊断 |

### 3. Job Prompt 信噪比低

当前 `Summarize()` 将完整原始日志拼入 Job Prompt，8 个 Worker 异常时 Prompt 膨胀约 480 行原始日志，关键信号被稀释。

---

## 改造目标

1. **修复 condition 分析**：按严重级别优先扫描所有 condition，取第一个 `Status=True` 的。
2. **透传所有子 Pod 字段**：确保 `PodLogConfig`、`EnablePodLog`、`EnableProm`、`AIClient` 全部传入子分析。
3. **用 Pod LLM 结论替换原始摘要**：Explain 模式下对每个 Pod 独立调用 LLM，将结构化结论（~4 行）存入 `MasterDiagnosis`/`WorkerDiagnosis`，Job LLM 拿到的是已消化的结论而非原始日志。
4. **并发执行 Pod 分析**：Master + 异常 Worker 同时发起，消除串行 LLM 等待。

---

## 方案设计

### Condition 修复

定义优先级顺序（最严重优先），扫描所有 condition 找第一个 `Status=True` 的：

```go
var condPriority = []kubeflowv1.JobConditionType{
    kubeflowv1.JobFailed,
    kubeflowv1.JobRestarting,
    kubeflowv1.JobSucceeded,
    kubeflowv1.JobSuspended,
    kubeflowv1.JobRunning,
    kubeflowv1.JobCreated,
}

func findActiveCondition(conds []kubeflowv1.JobCondition) *kubeflowv1.JobCondition {
    for _, pt := range condPriority {
        for i := range conds {
            if conds[i].Type == pt && conds[i].Status == v1.ConditionTrue {
                return &conds[i]
            }
        }
    }
    return nil
}
```

`Analyze()` 中替换原有逻辑：

```go
active := findActiveCondition(job.Status.Conditions)
if active == nil {
    result.Metadata["JobStatus"] = "Unknown"
    result.Warning = append(result.Warning, common.Warning{Text: "Job has no active condition"})
} else {
    result.Metadata["JobStatus"] = string(active.Type)
    switch active.Type {
    case kubeflowv1.JobSucceeded:
        result.Info = append(result.Info, common.Info{Text: "Job completed successfully."})
        skipPodAnalysis = true
    case kubeflowv1.JobFailed:
        result.Error = append(result.Error, kcommon.Failure{
            Text: fmt.Sprintf("Job failed: %s - %s", active.Reason, active.Message),
        })
    case kubeflowv1.JobRestarting:
        result.Warning = append(result.Warning, common.Warning{
            Text: fmt.Sprintf("Job is restarting: %s - %s", active.Reason, active.Message),
        })
    case kubeflowv1.JobSuspended:
        result.Info = append(result.Info, common.Info{
            Text: fmt.Sprintf("Job is suspended: %s", active.Reason),
        })
        skipPodAnalysis = true
    case kubeflowv1.JobRunning, kubeflowv1.JobCreated:
        result.Info = append(result.Info, common.Info{
            Text: fmt.Sprintf("Job is %s.", active.Type),
        })
    }
}
```

### 数据流

```
PytorchJobAnalyzer.Analyze(a)
  │
  ├─ [串行] findActiveCondition → Job conditions / events 采集
  │
  └─ analyzePytorchJobPods(a, job, result)
       │
       ├─ list pods（保持不变）
       ├─ categorizeWorkers() → abnormal / normal 分类
       │
       ├─ [goroutine] analyzePodWithExplain(master)     ─┐
       ├─ [goroutine] analyzePodWithExplain(worker[0])   │  sync.WaitGroup
       ├─ [goroutine] analyzePodWithExplain(worker[1])   │  并发执行
       └─ [goroutine] analyzePodWithExplain(worker[N])  ─┘
            │
            └─ analyzePodWithExplain(a, pod, podAnalyzer) string
                 ├─ podAnalyzer.Analyze(sub)     ← 完整字段透传
                 ├─ if a.AIClient != nil:        ← Explain 模式
                 │    podAnalyzer.Prompt(r)
                 │    → AIClient.GetCompletion() → 结构化结论 (~4行)
                 └─ else:
                      → podAnalyzer.Summarize(r) ← 无 LLM 时降级
```

### 新增函数：`analyzePodWithExplain`

```go
// analyzePodWithExplain 对单个 Pod 做诊断：
//   - Explain 模式（a.AIClient != nil）：调 Pod LLM，返回结构化结论
//   - 非 Explain 模式：返回 Summarize() 原始摘要（降级兼容）
func (p PytorchJobAnalyzer) analyzePodWithExplain(
    a common.Analyzer,
    pod *v1.Pod,
    podAnalyzer PodAnalyzer,
) string {
    sub := common.Analyzer{
        Analyzer: kcommon.Analyzer{
            Client:    a.Client,
            Context:   a.Context,
            Namespace: a.Namespace,
            AIClient:  a.AIClient,
        },
        Name:           pod.Name,
        CollectorImage: a.CollectorImage,
        EnableProm:     a.EnableProm,
        EnablePodLog:   a.EnablePodLog,
        PodLogConfig:   a.PodLogConfig,
    }

    r, err := podAnalyzer.Analyze(sub)
    if err != nil || r == nil {
        return fmt.Sprintf("Pod %s: analysis failed: %v", pod.Name, err)
    }

    if a.AIClient != nil {
        prompt := podAnalyzer.Prompt(r)
        if prompt != "" {
            if explain, err := a.AIClient.GetCompletion(a.Context, prompt); err == nil {
                return explain
            }
        }
    }

    return podAnalyzer.Summarize(r)
}
```

### `analyzePytorchJobPods` 并发改写

```go
func (p PytorchJobAnalyzer) analyzePytorchJobPods(
    a common.Analyzer, job *kubeflowv1.PyTorchJob, result *common.Result,
) error {
    // ... list pods, 分类（不变）...

    podAnalyzer := NewPodAnalyzer(p.prometheus)
    abnormal, normal := categorizeWorkers(workerPods)

    // 构造并发任务：master + 前 maxDetailedAbnormalWorkers 个异常 worker
    type task struct {
        pod        *v1.Pod
        isMaster   bool
    }
    var tasks []task
    if masterPod != nil {
        tasks = append(tasks, task{masterPod, true})
    }
    for i, wp := range abnormal {
        if i >= maxDetailedAbnormalWorkers {
            break
        }
        tasks = append(tasks, task{wp, false})
    }

    // 并发执行
    explains := make([]string, len(tasks))
    var wg sync.WaitGroup
    for i, t := range tasks {
        wg.Add(1)
        go func(i int, t task) {
            defer wg.Done()
            explains[i] = p.analyzePodWithExplain(a, t.pod, podAnalyzer)
        }(i, t)
    }
    wg.Wait()

    // 填充 metadata
    idx := 0
    if masterPod != nil {
        result.Metadata["MasterDiagnosis"] = explains[0]
        idx = 1
    }
    var workerLines []string
    for _, wp := range abnormal[:min(len(abnormal), maxDetailedAbnormalWorkers)] {
        workerLines = append(workerLines,
            fmt.Sprintf("Worker Pod %s (Abnormal):\n%s", wp.Name, explains[idx]))
        idx++
    }
    // 正常 worker 直接附简短文字，不走 LLM
    normalLimit := maxRunningSummaryWorkers
    if len(abnormal) == 0 {
        normalLimit = len(normal)
    }
    for i, wp := range normal {
        if i >= normalLimit {
            break
        }
        workerLines = append(workerLines,
            fmt.Sprintf("Worker Pod %s: Running and Ready.", wp.Name))
    }
    if len(workerLines) > 0 {
        result.Metadata["WorkerDiagnosis"] = strings.Join(workerLines, "\n---\n")
    }
    return nil
}
```

### Prompt 模板修复

修复 `pytorchJobPromptTemplate` 中 `Analysis` 字段缺少闭合 `}` 的问题：

```
// 当前（缺少 }）
Analysis: {结合任务状态、Pod 状态、事件、日志等...（即用三个反引号包裹日志片段）
Solution: {给出最关键的一句总结，不超过 100 字}

// 修后
Analysis: {结合任务状态、Pod 状态、事件、日志等...（即用三个反引号包裹日志片段）}
Solution: {给出最关键的一句总结，不超过 100 字}
```

---

## 延迟对比

```
改造前（串行）：
  master(analyze) → worker[0](analyze) → ... → worker[N](analyze) → job LLM
  总耗时 ≈ (N+1) × T_analyze + T_job_llm
         ≈ 6×1s + 5s = 11s（5 异常 Worker 场景）

改造后（并发）：
  ┌─ master(analyze + pod LLM) ─┐
  ├─ worker[0](analyze + pod LLM)┤ → (汇总) → job LLM
  └─ worker[N](analyze + pod LLM)┘
  总耗时 ≈ max(T_analyze + T_pod_llm) + T_job_llm
         ≈ (1s + 3s) + 5s = 9s（LLM 调用减少约串行一半延迟）
```

非 Explain 模式（无 LLM）时并发同样生效，Pod analyze 本身也是 IO 密集型（API 调用 + 日志拉取），并发收益明显。

---

## 改动文件

| 文件 | 改动内容 |
|------|----------|
| `pkg/analyzer/pytorchjob.go` | ① `findActiveCondition` + condition switch 重写；② `categorizeWorkers` 抽离为独立函数；③ `analyzePodWithExplain` 新增；④ `analyzePytorchJobPods` 并发改写；⑤ `analyzeWorkerPods` 删除（逻辑并入 ④） |
| `pkg/ai/prompt_templates.go` | 修复 `Analysis` 字段缺失闭合 `}` |

`diagnosis.go`、`common/types.go` 无需改动。
