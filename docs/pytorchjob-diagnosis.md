# PyTorchJob Diagnostic Feature

## Table of Contents

* [Background](#background)
* [PyTorchJob Definition Example](#pytorchjob-definition-example)
* [Diagnostic Process](#diagnostic-process)
* [Example Cases](#example-cases)
* [Custom Prompt Support](#custom-prompt-support)

  * [Mechanism](#mechanism)
  * [How to provide a custom prompt](#how-to-provide-a-custom-prompt)
  * [Available Variables](#available-variables)
* [Result Format](#result-format)
* [Example Output](#example-output)
* [Prompt Template Versioning](#prompt-template-versioning)
* [Summary](#summary)

---

## Background

In Kubernetes-based machine learning platforms, **Kubeflow PyTorchJob** is widely used for managing distributed training tasks.
However, in real-world usage, PyTorchJobs often encounter various issues such as:

* Task failure (Failed status)
* Resource scheduling problems (Pending pods)
* Abnormal training behavior (OOM, loss=NaN, etc.)
* Inconsistent replica status

Automated diagnosis helps quickly identify and analyze the root cause of these issues, reducing manual troubleshooting effort.

---

## PyTorchJob Definition Example

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

## Diagnostic Process

**PyTorchJob diagnosis** follows a multi-level process:

1. **Resource definition layer** *(User responsibility)*
   Validate resource configuration (replica counts, affinity, tolerations, etc.).
   Note: This is typically a static check that relies on correct user configuration. The diagnosis tool does not perform a full static validation, but incorrect resource definitions may cause subsequent issues.
2. **Control layer (Job status and pod status)**
   Analyze `PyTorchJob.status` and pod-level status (`Pending`, `Running`, `Failed`, `Succeeded`).
   This layer helps detect issues such as pods not being scheduled or replicas failing to start.
3. **Execution layer (Pod logs)**
   Use LLM-based analysis to interpret pod logs and related Kubernetes events, identifying common training errors such as OOM, NaN loss, or missing packages.

The detailed process is illustrated in the figure below:

![pytorchjob-diagnosis-process](../docs/assets/pytorchjob-diagnosis-process.png)

---

## Example Cases

1. **Job Created but no pods running**
   → Scheduling failure due to insufficient GPUs.

2. **Job Created → Running → Failed**
   → Master replica failed due to OOM (exit code 137).

3. **Job long-term Pending**
   → No matching nodes found, PodScheduled=False, events indicate insufficient resources.

4. **Job succeeded**
   → Healthy, no issues detected.

---

## Custom Prompt Support

Users can **customize the diagnosis prompt** to control how the analysis result is structured and phrased.

### Mechanism

* The system first looks for an **override prompt** in a ConfigMap named `aegis-prompts`, under `/aegis/prompts/`.
* If an override prompt is provided, it is used instead of the built-in default prompt.
* If not, the default system prompt is used.

### How to provide a custom prompt

Example ConfigMap:

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

### Available Variables

In the template, you can reference:

* `.Metadata["JobName"]` — Job name
* `.Metadata["JobStatus"]` — Job status (Succeeded / Failed / Running / Created)
* `.Metadata["LauncherStatus"]` — Launcher replica status
* `.Metadata["MasterExpected"]`, `.Metadata["MasterCreatedCount"]`
* `.Metadata["WorkerExpected"]`, `.Metadata["WorkerCreatedCount"]`
* `.Metadata["MasterDiagnosis"]` — Master pod diagnosis summary
* `.Metadata["WorkerDiagnosis"]` — Worker pod diagnosis summary

And:

* `.ErrorInfo` — Extracted error information
* `.EventInfo` — Related Kubernetes events
* `.LogInfo` — Relevant pod logs

---

## Result Format

The diagnosis output follows a structured format:

```
Healthy: {Yes / No}
Error: {One-line summary of the likely cause}
Analysis: {Concise analysis of the root cause, using Job / pod status, events, logs}
Solution: {Step-by-step actionable recommendations}
```

<!-- ---

## Prompt Template Versioning

When maintaining multiple custom prompts, we recommend adopting a simple versioning strategy to avoid conflicts and unexpected changes in production environments.

Recommended practices:

* Use filename suffix for versioning: `pytorchjob-v1.tmpl`, `pytorchjob-v2.tmpl`, etc.
* Maintain version history in Git to allow rollbacks if needed.
* Clearly document which prompt version is used in each environment (e.g. staging vs. production).
* When upgrading prompts, validate them in a non-production environment before rollout.

Following this practice ensures your LLM-based diagnostic experience remains **stable and predictable** across deployments. -->

