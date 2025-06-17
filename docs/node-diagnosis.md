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

* A **Collector Pod** can be launched on the target Node to collect additional system-level information.
* The Collector Pod runs a user-defined image and script.
* Logs from the Collector Pod are parsed as *Info* context.
* By default, AegisDiagnosis uses the built-in image:  
  `registry-ap-southeast.scitix.ai/k8s/collector:v1.0.0`  
  located under [`manifests/collector`](../manifests/collector)  
  with a default shell script [`collect.sh`](../manifests/collector/collect.sh).
* 👉 See [Collector Pod Guide](#collector-pod-guide) for instructions on providing your own image and logic.


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

## Collector Pod Guide

👉 The Collector Pod mechanism allows users to deploy their own customized Pod for gathering detailed system-level information from Nodes.

By default, AegisDiagnosis uses an internal collector image, but you can override it with your own container and script for advanced use cases such as specialized log collection, hardware checks, or compliance auditing.

### Structure Overview

To use a custom collector, you must provide:

* A **diagnosis CR** (`AegisDiagnosis`) with `collectorConfig` defined
* A **custom image** that includes the collector logic
* A **shell script** (e.g., `collect.sh`) as the entrypoint

You can find a working example under:

```
examples/diagnosis/node/collector/
├── collect.sh
├── Dockerfile.collector
└── diagnosis-node-custom-collector.yaml
```

### 📜 1. Sample Script (`collect.sh`)

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

This script prints a simple success log with timestamp and hostname, and stores it to the mounted log path.

### 🐳 2. Dockerfile (`Dockerfile.collector`)

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y util-linux grep coreutils bash

COPY collect.sh /collector/collect.sh
RUN chmod +x /collector/collect.sh

CMD ["/bin/bash", "/collector/collect.sh"]
```

This builds a lightweight container with basic shell utilities and includes the custom script.

Build it locally:

```bash
docker build -f Dockerfile.collector -t myregistry/mycustom-collector:latest .
docker push myregistry/mycustom-collector:latest
```

### 📦 3. AegisDiagnosis YAML (`diagnosis-node-custom-collector.yaml`)

```yaml
apiVersion: aegis.io/v1alpha1
kind: AegisDiagnosis
metadata:
  name: node-diagnosis-sample
spec:
  object:
    kind: Node
    name: node-01
  timeout: "10m"
  collectorConfig:
    image: myregistry/mycustom-collector:latest
    command:
      - "/bin/bash"
      - "-c"
      - "/collector/collect.sh"
    volumeMounts:
      - name: custom-logs
        mountPath: /var/log/custom
    volumes:
      - name: custom-logs
        hostPath:
          path: /var/log/custom
```

This instructs AegisDiagnosis to launch a Pod on the target node using your image and command. Output logs written to `/var/log/custom` on the node will be picked up and parsed as part of the diagnosis `Info` section.
