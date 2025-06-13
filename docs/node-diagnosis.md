## AI-powered Node Diagnosis

The **AI-powered Node Diagnosis** feature automatically analyzes the health and state of Kubernetes Nodes, and generates a structured prompt to support large language model (LLM)-based diagnosis.

It helps users quickly identify potential issues on Nodes by combining multiple data sources—such as Node Conditions, Events, and system-level Info—and producing clear, actionable insights through AI analysis.

---

### Architecture and Workflow

The diagnosis process consists of the following steps:

### 1. Locate the Node Object

* The diagnosis targets a specific Node, identified by its name.
* The Node resource is retrieved from the Kubernetes API to verify its existence.

### 2. Data Collection

The following types of diagnostic data are collected:

#### **Failure**

* Node **Condition** statuses are queried from Prometheus.
* Unhealthy Conditions (e.g. `NotReady`, `MemoryPressure`, `DiskPressure`) are recorded as *Failures*.

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
* **👉 Note:** See [Collector Pod Guide](#collector-pod-guide) for instructions on providing your own Collector Pod image.

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

### Example Use Cases

* Automated detection of Node hardware or system failures.
* AI-based reasoning for complex multi-symptom Node issues.
* Enhanced observability and troubleshooting in large Kubernetes clusters.

---

## Collector Pod Guide

👉 The Collector Pod mechanism allows users to deploy their own customized Pod for gathering detailed system-level information from Nodes.

You must provide a suitable Collector Pod image. Detailed instructions and examples will be added here.
