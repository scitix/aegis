# Automated Node Isolation and Recovery for HPC Clusters

This repository provides a practical example of **automated failure remediation** in an HPC-style Kubernetes cluster. It uses [Aegis](https://github.com/scitix/aegis) as the alert-driven automation framework and integrates with Prometheus alerting and Argo Workflows to implement **automatic node cordon and uncordon** actions.

## Use Case

In high-performance computing (HPC) environments, stability and job scheduling fairness are critical. When a node encounters serious issues (e.g., gpu hardware failures, bmc fan failures), it should be temporarily removed from the scheduler to prevent task loss or corruption. Once the issue is resolved, the node can be safely re-enabled.

This solution enables:

* 🧠 **Auto cordon** of unhealthy nodes on critical alerts
* 🔁 **Auto uncordon** of nodes when alerts disappear
* 🔧 Infrastructure-as-code definition of operational actions
* 🔐 Fine-grained control and retry logic via Argo Workflows

---

## Architecture Overview

```text
DataSource → Prometheus Alert → AegisAlertOpsRule → AegisOpsTemplate → Argo Workflow → Kubectl cordon/uncordon
```

### Components

| Component           | Description                                                                |
| ------------------- | -------------------------------------------------------------------------- |
| `DataSource`    | Exporters such as `dcgm-exporter` `node-exporter` `ipmi-exporter` `kube-state-metrics`   |
| `PrometheusRule`    | Triggers alerts like `NodeCriticalIssue` or `NodeCriticalIssueDisappeared` |
| `AegisAlertOpsRule` | Maps alert conditions to operational workflows                             |
| `AegisOpsTemplate`  | Defines remediation logic as a Kubernetes-native YAML                      |
| `Argo Workflow`     | Executes the actual commands (e.g., `kubectl cordon`) on affected nodes    |

---

## Included Templates

### 1. Node Critical Issue (Cordon)

```yaml
kubectl cordon {{.node}}
```

Triggered when `NodeCriticalIssue` is firing. Prevents the node from scheduling any new pods.

### 2. Node Recovery (Uncordon)

```yaml
kubectl uncordon {{.node}}
```

Triggered when the alert disappears (via `NodeCriticalIssueDisappeared`), re-enabling the node for scheduling.

Both templates are run via Argo Workflows and are bound to the `aegis-workflow` service account.

---

## How to Use

1. Deploy the `PrometheusRule` to detect critical node issues.
2. Create `AegisAlertOpsRule` resources to bind alerts to actions.
3. Define `AegisOpsTemplate` with Argo Workflow manifests.
4. Aegis will automatically trigger the workflow when alerts are firing or resolved.

> Note: Your cluster must have Argo Workflows and Aegis properly deployed and integrated with Prometheus Alertmanager.
