# PyTorchJob 诊断功能

## 目录

* [背景](#背景)
* [PyTorchJob 定义示例](#pytorchjob-定义示例)
* [诊断流程](#诊断流程)
* [典型案例](#典型案例)
* [自定义 Prompt 支持](#自定义-prompt-支持)

  * [机制](#机制)
  * [如何提供自定义 Prompt](#如何提供自定义-prompt)
  * [可用变量](#可用变量)
* [诊断结果格式](#诊断结果格式)
* [结果示例](#结果示例)
* [Prompt 模板版本管理](#prompt-模板版本管理)
* [总结](#总结)

---

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

## 典型案例

1. **Job 已创建但无 Pod 运行**
   → 调度失败，因 GPU 资源不足。

2. **Job 创建 → 运行 → 失败**
   → Master 副本因 OOM（退出码 137）失败。

3. **Job 长时间 Pending**
   → 无匹配节点，PodScheduled=False，事件显示资源不足。

4. **Job 成功**
   → 健康，无异常。

---

## 自定义 Prompt 支持

用户可通过 **自定义诊断 Prompt** 控制分析结果的结构与表述风格。

### 机制

* 系统优先查找名为 `aegis-prompts` 的 ConfigMap 中 `/aegis/prompts/` 下的 **覆盖 Prompt**。
* 若存在覆盖 Prompt，则使用该 Prompt；否则使用内置默认 Prompt。

### 如何提供自定义 Prompt

示例 ConfigMap：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aegis-prompts
data:
  pytorchjob.tmpl: |
    You are a Kubernetes + Kubeflow diagnostic expert...
    Job Name: {{ index .Metadata "JobName" }}
    Job Status: {{ index .Metadata "JobStatus" }}
    ...
```

### 可用变量

模板中可引用：

* `.Metadata["JobName"]` — Job 名称
* `.Metadata["JobStatus"]` — Job 状态（Succeeded / Failed / Running / Created）
* `.Metadata["LauncherStatus"]` — Launcher 副本状态
* `.Metadata["MasterExpected"]`、`.Metadata["MasterCreatedCount"]`
* `.Metadata["WorkerExpected"]`、`.Metadata["WorkerCreatedCount"]`
* `.Metadata["MasterDiagnosis"]` — Master Pod 诊断摘要
* `.Metadata["WorkerDiagnosis"]` — Worker Pod 诊断摘要

以及：

* `.ErrorInfo` — 提取的错误信息
* `.EventInfo` — 相关 Kubernetes 事件
* `.LogInfo` — 相关 Pod 日志

---

## 诊断结果格式

诊断输出采用结构化格式：

```
Healthy: {Yes / No}
Error: {一句话总结可能原因}
Analysis: {简明分析根因，结合 Job / Pod 状态、事件、日志}
Solution: {分步骤可操作建议}
```

