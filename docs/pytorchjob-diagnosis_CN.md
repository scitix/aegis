# PyTorchJob 诊断功能

## 背景

在基于 Kubernetes 的机器学习平台中，**Kubeflow PyTorchJob** 被广泛用于管理分布式训练任务。
然而在实际使用中，PyTorchJob 常常会遇到各种问题，例如：

* 任务失败（Failed 状态）
* 资源调度问题（Pod Pending）
* 异常训练行为（OOM、loss=NaN 等）
* 副本状态不一致

通过自动化诊断，可以快速定位和分析问题根因，降低人工排查成本。

---

## PyTorchJob 定义示例

```yaml
apiVersion: kubeflow.org/v1
kind: PyTorchJob
metadata:
  name: train-job
spec:
  pytorchReplicaSpecs:
    Master:
      replicas: 1
    Worker:
      replicas: 2
status:
  conditions:
    - type: Succeeded / Failed / Running
      reason: WorkerFailed / JobCancelled / JobSucceeded
      message: "worker-0 exited with code 137"
  replicaStatuses:
    Master:
      succeeded: 1
    Worker:
      failed: 1
```

---

## 诊断流程

**PyTorchJob 诊断**遵循多层次流程：

1. **资源定义层** *(用户责任)*
   检查资源配置（副本数量、亲和性、容忍度等）。
   注意：这通常是静态检查，依赖用户正确配置。诊断工具不会进行完整静态校验，但配置错误可能引发后续问题。
2. **控制层（Job 状态和 Pod 状态）**
   分析 `PyTorchJob.status` 以及 Pod 层状态（`Pending`、`Running`、`Failed`、`Succeeded`）。
   此层可检测如 Pod 未调度、副本启动失败等问题。
3. **执行层（Pod 日志）**
   使用 LLM 解析 Pod 日志和相关 Kubernetes 事件，识别常见训练错误，如 OOM、NaN loss、缺少包等。

以下图示展示了详细流程：

![pytorchjob-diagnosis-process](../docs/assets/pytorchjob-diagnosis-process.png)

---

## 诊断用例：使用自定义 Prompt 对 PyTorchJob 进行诊断

这是一个使用**自定义 Prompt 模板**对 PyTorchJob 进行诊断的示例。

* 📄 自定义 Prompt 定义在 [`deploy/prompt-config.yaml`](../deploy/prompt-config.yaml)
* 📄 诊断资源定义在 [`examples/diagnosis/pytorchjob/diagnosis-pytorchjob.yaml`](../examples/diagnosis/pytorchjob/diagnosis-pytorchjob.yaml)

执行诊断：

```bash
kubectl apply -f examples/diagnosis/pytorchjob/diagnosis-pytorchjob.yaml
kubectl get aegisdiagnosises.aegis.io -n monitoring --watch
```

诊断完成后查看结果：

```bash
kubectl describe -n monitoring aegisdiagnosises.aegis.io pytorchjob-test
```

✅ 此示例展示了如何通过 ConfigMap 使用自定义模板覆盖系统默认 Prompt。
💡 即使不配置自定义 Prompt，Aegis 仍会使用**内置默认 Prompt**正常工作并生成诊断报告。

---

## 自定义提示词支持（Custom Prompt Support）

用户可以**自定义诊断提示词（prompt）**，以控制分析结果的结构和表达方式。

### 可用变量及模板用法

在模板中，您可以使用如下方式引用变量，例如：

```gotemplate
{{ index .Metadata "JobName" }}
```

### `.Metadata` 字段

这些字段用于描述 PyTorchJob 的基本状态与角色信息：

* `{{ index .Metadata "JobName" }}` — 任务名称
* `{{ index .Metadata "JobStatus" }}` — 任务状态（Succeeded / Failed / Running / Created）
* `{{ index .Metadata "LauncherStatus" }}` — Launcher 副本的状态
* `{{ index .Metadata "MasterExpected" }}` — Master 预期副本数
* `{{ index .Metadata "MasterCreatedCount" }}` — Master 实际已创建副本数
* `{{ index .Metadata "WorkerExpected" }}` — Worker 预期副本数
* `{{ index .Metadata "WorkerCreatedCount" }}` — Worker 实际已创建副本数
* `{{ index .Metadata "MasterDiagnosis" }}` — Master Pod 的诊断摘要
* `{{ index .Metadata "WorkerDiagnosis" }}` — Worker Pods 的诊断摘要

### 其他字段

这些字段包含诊断过程中提取的异常、事件和日志信息：

* `{{ .ErrorInfo }}` — 提取的错误信息摘要
* `{{ .EventInfo }}` — 相关 Kubernetes 告警事件
* `{{ .LogInfo }}` — Pod 级别的关键日志片段

➡️ 使用方式详见 [自定义提示词指南（Custom Prompt Guide）](./diagnosis-custom-prompt-guide_CN.md)。
## 诊断结果格式

诊断输出采用结构化格式：

```
Healthy: {Yes / No}
Error: {一句话总结可能原因}
Analysis: {简明分析根因，结合 Job / Pod 状态、事件、日志}
Solution: {分步骤可操作建议}
```

