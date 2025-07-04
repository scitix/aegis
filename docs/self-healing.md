# AI HPC Node Self‑healing

The Aegis self‑healing subsystem delivers **real‑time detection, priority assessment, and SOP‑driven automatic remediation** for common node failures in AI HPC clusters, enabling “lights‑out, seconds‑level recovery.”

---

## ✨ Feature Overview

1. **Periodic Inspection** – A daemon routinely collects node metrics (Exporter / PromQL) and builds an `AegisNodeStatus` snapshot.
2. **Anomaly Detection** – Hardware, OS, and container anomalies are recognised via rule‑based Conditions and thresholds.
3. **Priority Queuing** – Every Condition is mapped to one of four priority queues so the most urgent issues are tackled first.
4. **SOP Scheduling** – The scheduler selects an appropriate **SOP plug‑in** based on priority and executes the remediation steps.
5. **Ticket Tracking** – *Node Ticketing* annotations record every action and result for full auditability.

---

## 🏗️ SOP Plug‑in Architecture

```go
// Core interface

type SOP interface {
    CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error // Initialisation

    Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool // Pre‑check / idempotency

    Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error // Remediation action
}
```

* **Plug‑in model** – Each fault category ships as a standalone Go package; hot‑pluggable and easy to extend.
* **Idempotent** – `Evaluate` runs before every execution to avoid duplicate repairs.
* **Three stages** – `CreateInstance` → `Evaluate` → `Execute`.

> **Covered domains:** Node / CPU / Disk / GPFS / GPU / IB / Network / Memory / Process / PeerMem / System.

---

## 📜 Condition & SOP Catalogue (Excerpt)

| Domain      | Representative Conditions (sample)                                             |
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

Full list available in *docs/self‑healing‑conditions.md*.

---

## 🔀 Priority Queue Mapping

| Priority Range           | Queue Name         | Description                             |
| ------------------------ | ------------------ | --------------------------------------- |
| `== NodeNotReady`        | **NotReady**       | Highest urgency – node unreachable      |
| `== NodeCordon`          | **Cordon**         | Manually cordoned; needs quick recovery |
| `(1, Emergency]`         | **EmergencyList**  | Severe impact on workloads              |
| `(Emergency, CanIgnore]` | **CanIgnoreList**  | Minor issues tolerable for a short time |
| `> CanIgnore`            | **MustIgnoreList** | Explicitly configured *must ignore*     |

The **scheduler** re‑orders queues each cycle to guarantee high‑priority Conditions trigger an SOP first.

---

## 🗂️ Node Ticketing Annotation

Node annotations under key `aegis.io/ticketing` capture context and workflow history. Example:

```yaml
kubectl annotate node dev1 aegis.io/ticketing='|
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

> **Tip:** Keep fields concise; annotations have a size limit.

---

## 🐳 Self‑healing Images & Unified Entrypoint

> **Job templates:** `/selfhealing/job/*.yaml`
>
> **Namespace:** `monitoring`

### Core Job Templates

```
restart_node.yaml   # Node reboot
shutdown_node.yaml  # Shutdown
healthcheck_node.yaml
repair_node.yaml
remedy_node.yaml
```

---

## 📈 Metrics & Observability

| Metric                    | Meaning                                            |
| ------------------------- | -------------------------------------------------- |
| `aegis_sop_total{status}` | Count of SOP executions by result                  |
| `aegis_selfheal_seconds`  | End‑to‑end self‑healing latency                    |
| `aegis_ticket_open_total` | Number of tickets in *open* state                  |
| `aegis_condition_gauge`   | Active node count per Condition across the cluster |

Grafana dashboards visualise repair speed and success rates to guide SOP optimisation.
