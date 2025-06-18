# AI-powered Node Diagnosis

The **AI-powered Node Diagnosis** feature automatically analyzes the health and state of Kubernetes Nodes, and generates a structured prompt to support large language model (LLM)-based diagnosis.

It helps users quickly identify potential issues on Nodes by combining multiple data sources—such as Node Conditions, Events, and system-level Info—and producing clear, actionable insights through AI analysis.

---

## Architecture and Workflow

The diagnosis process consists of the following steps:

### 1. Locate the Node Object

* The diagnosis targets a specific Node, identified by its name.
* The Node resource is retrieved from the Kubernetes API to verify its existence.

### 2. Data Collection

The following types of diagnostic data are collected:

#### **Failure**

* Node **Condition** statuses are analyzed to detect failures (e.g. `NotReady`, `MemoryPressure`, `DiskPressure`).
* Two condition data sources are supported:
  * If Prometheus is enabled, node condition metrics are queried from Prometheus.
  * Otherwise, conditions are retrieved directly from the Kubernetes API (`status.conditions`).
* Any unhealthy condition will be recorded as a *Failure*.


#### **Warning**

* Kubernetes **Events** related to the Node are fetched.
* Two modes are supported:

  * If Prometheus Event export is enabled, Events are fetched via Prometheus.
  * Otherwise, Events are fetched from the Kubernetes API.
* Warning Events are recorded as *Warnings*.

#### **Info**

* A **Collector Pod** is launched on the target Node to collect system-level diagnostic data.

* The Collector Pod runs a container image that includes a diagnostic script (e.g., system info, logs).

* Logs from the container are collected and presented as *Info* in the diagnosis result.

* The collector image is configured **globally** via the Aegis controller's startup arguments.

  Example configuration in `deployment.yaml`:

  ```yaml
  args:
    - --diagnosis.collectorImage=myregistry/mycustom-collector:latest
  ```

* 🧩 The default image used is:

  ```text
  registry-ap-southeast.scitix.ai/k8s/collector:v1.0.0
  ```

* 👉 To build and configure your own collector image, refer to the [Collector Pod Guide](#collector-pod-guide).


### 3. AI Prompt Construction

All collected data is assembled into a structured prompt for large language model (LLM)-based diagnosis. The prompt includes:

* **Role Setting:** Defines the AI assistant’s role (Node health diagnosis).
* **Instruction:** Guides the AI on how to interpret the provided data.
* **Node Context:**

  * *Errors* — Node Condition failures.
  * *Warnings* — Events.
  * *Info* — Collector Pod outputs.
* **Response Format:** Specifies the required AI output structure:

  * `Healthy`
  * `Error`
  * `Analysis`
  * `Solution`

This design enables LLMs to perform human-like reasoning on Node health and generate actionable diagnostic reports.

## Example Use Cases

### Diagnose a Control Plane Node Using Default Collector

This example shows how to use the built-in collector image to diagnose a Kubernetes Node.

**Step 1: Apply the Diagnosis CR**

```bash
kubectl apply -f examples/diagnosis/node/diagnosis-node.yaml
````

The file `diagnosis-node.yaml` defines an `AegisDiagnosis` resource with the following content:

```yaml
apiVersion: aegis.io/v1alpha1
kind: AegisDiagnosis
metadata:
  name: diagnose-node
  namespace: your-namespace
spec:
  object:
    kind: Node
    name: your-node
```

**Step 2: Monitor Diagnosis Execution**

```bash
kubectl get aegisdiagnosises.aegis.io -n your-namespace --watch
```

Once completed, you should see:

```
NAME            PHASE       AGE
diagnose-node   Completed   38s
```

**Step 3: View Diagnosis Result**

```bash
kubectl describe aegisdiagnosises.aegis.io -n your-namespace diagnose-node
```

Example output:

```
Status:
  Phase: Completed
  Explain: Healthy: No
  Error: Infiniband device failed to register on the node (IBRegisterFailed)

  Result:
    Failures:
      - condition: IBRegisterFailed
        type: ib
        id: ""
        value: "1"

    Infos:
      [kernel]
      - [Fri May 30 05:41:33 2025] IPVS: rr: TCP 172.17.115.192:443 - no destination available
      - [Fri May 30 05:42:33 2025] IPVS: rr: TCP 172.17.115.192:443 - no destination available
      - ...
      [gpfs.health]
      - <no data>
      [gpfs.log]
      - [SKIPPED] mmfs.log.latest not found

```

In this case, the collector ran successfully on the node and captured system logs like kernel messages and GPFS health information. The `Info` field provides valuable context for further diagnosis.

---

## Custom Prompt Support

Users can **customize the diagnosis prompt** to control how the analysis result is structured and phrased.

### Available Variables

You can reference the following variables in your Node diagnosis prompt template:


* `{{ .ErrorInfo }}` — Summary of abnormal indicators
* `{{ .EventInfo }}` — Related Kubernetes events for the node, useful for historical context
* `{{ .LogInfo }}` — Relevant pod logs on the node, optional but helpful for root cause analysis


These fields are automatically populated and passed into the template during a Node diagnosis task.

➡️ For details on how to customize prompts, see the [Custom Prompt Guide](./diagnosis-custom-prompt-guide.md)

---

## Collector Pod Guide

👉 The **Collector Pod** mechanism enables advanced diagnosis by executing a custom script in a Pod on the target Node.

The Collector image is **globally configured** in the Aegis controller deployment and **not** set per-diagnosis.


### 1. Collector Image Configuration

Edit your `Deployment` to specify the collector image using the flag `--diagnosis.collectorImage`.

```yaml
args:
  - --diagnosis.collectorImage=myregistry/mycustom-collector:latest
```

### 2. Collector Image Requirements

You must provide a valid container image that:

* Contains a shell script as the entrypoint (e.g., `collect.sh`)
* Includes essential shell tools (`bash`, `coreutils`, `grep`, etc.)
* Writes log output to a standard location (e.g., `/var/log/custom/diagnosis.log`)

### 3. Example: `collect.sh`

```bash
#!/bin/bash
set -e

LOG_FILE="/var/log/custom/diagnosis.log"
mkdir -p "$(dirname "$LOG_FILE")"

log() {
  echo "$1" | tee -a "$LOG_FILE"
}

log "[custom.collector]"
log "- Custom collector script executed successfully."
log "- Timestamp: $(date)"
log "- Hostname: $(hostname)"
```

### 4. Example: Dockerfile

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y util-linux grep coreutils bash

COPY collect.sh /collector/collect.sh
RUN chmod +x /collector/collect.sh

CMD ["/bin/bash", "/collector/collect.sh"]
```

Build & push:

```bash
docker build -f Dockerfile.collector -t myregistry/mycustom-collector:latest .
docker push myregistry/mycustom-collector:latest
```
