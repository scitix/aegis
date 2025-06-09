# Node Memory Pressure Auto-Healing Case

## Use Case: `NodeHasMemoryPressure`

## Scenario & Trigger

This automated operation is triggered when a node experiences **memory pressure** exceeding a critical threshold.

* **Alert Source**: Prometheus rule `NodeHasMemoryPressure`
* **Trigger Condition**:

  * If node memory usage exceeds 95% (with a 5% safety buffer)
  * Expression logic evaluates memory allocatable vs working set
* **Trigger Type**: `Firing`

## Automated Remediation

Once the alert fires, the Aegis system initiates a self-healing workflow defined by the `AegisOpsTemplate`. The workflow performs the following:

1. Logs current memory usage.
2. Clears Linux system page cache using:

   ```bash
   echo 3 > /proc/sys/vm/drop_caches
   ```

   (via `nsenter` to operate on host namespace)
3. Logs memory usage after dropping cache.

## YAML File Structure

| Kind                | Purpose                                  |
| ------------------- | ---------------------------------------- |
| `PrometheusRule`    | Detects memory pressure on nodes         |
| `AegisAlertOpsRule` | Maps alert to remediation action         |
| `AegisOpsTemplate`  | Defines the self-healing workflow (Argo) |

## How to Use

1. Deploy all 3 manifests in the Kubernetes cluster:

```bash
kubectl apply -f alert.yaml
kubectl apply -f ops.yaml
```

2. Ensure the Aegis controller, Argo Workflows, and Prometheus Alertmanager are properly configured.
3. Monitor alerts and watch Aegis trigger workflows for memory cleanup when threshold is breached.

## Expected Outcome

* On memory pressure, the system **automatically runs a workflow** on the affected node.
* Page cache is dropped.
* Memory pressure should be reduced without human intervention.