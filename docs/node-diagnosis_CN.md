# 基于 AI 的节点诊断

**AI 节点诊断**功能能够自动分析 Kubernetes 节点的健康状态，并构建结构化的提示（Prompt），以支持基于大语言模型（LLM）的智能诊断。

该功能通过结合多个数据源（如节点 Condition、事件、系统级信息等），快速定位问题，并通过 AI 生成清晰、可执行的诊断结论。

---

## 架构与工作流程

节点诊断过程主要包括以下步骤：

### 1. 定位目标节点

* 诊断对象是一个具体的 Node，通过名称指定。
* 系统会从 Kubernetes API 中拉取该 Node 对象以确认其存在。

### 2. 数据采集

以下几类诊断数据会被采集：

#### **Failure（失败）**

* 分析节点的 **Condition 状态**，如 `NotReady`、`MemoryPressure`、`DiskPressure` 等。
* 支持两种数据源：

  * 若启用了 Prometheus，则从 Prometheus 中查询 Condition 指标；
  * 否则，从 Kubernetes API 的 `status.conditions` 字段中获取。
* 若存在不健康的 Condition，将其记录为 *Failure*。

#### **Warning（警告）**

* 拉取与该节点相关的 **Kubernetes Events**。
* 同样支持两种获取方式：

  * 若启用了 Prometheus Event Export，则通过 Prometheus 获取；
  * 否则，直接从 Kubernetes API 拉取。
* 相关 Event 会作为 *Warnings* 被记录。

#### **Info（信息）**

* 可以在目标节点上启动一个 **Collector Pod**，用于采集额外的系统级信息。
* Collector Pod 会运行用户自定义的镜像与脚本。
* 采集到的日志内容会被解析并记录在诊断结果的 *Info* 区域中。
* 默认情况下，AegisDiagnosis 使用内置镜像：
  `registry-ap-southeast.scitix.ai/k8s/collector:v1.0.0`
  存放于 [`manifests/collector`](../manifests/collector)
  默认脚本为 [`collect.sh`](../manifests/collector/collect.sh)。
* 👉 如何自定义镜像与采集逻辑，请参考 [Collector Pod 使用指南](#collector-pod-guide)。

---

### 3. 构造 AI Prompt

系统将收集到的所有诊断信息组装为结构化 Prompt，用于 LLM 分析：

* **角色设定**：定义 AI 的职责，例如“节点健康状态分析助手”
* **任务指令**：引导 AI 如何理解输入信息
* **节点上下文信息**：

  * *Errors* — 节点 Condition 异常
  * *Warnings* — Kubernetes Event
  * *Info* — Collector Pod 输出
* **响应格式规范**：要求输出以下结构：

  * `Healthy`
  * `Error`
  * `Analysis`
  * `Solution`

这种设计使得 LLM 能够基于结构化上下文进行类人判断并生成可执行的诊断建议。

---

## 示例用法

### 使用内置 Collector 诊断控制节点

本示例演示如何使用默认的内置镜像诊断某个节点。

**步骤 1：应用诊断 CR**

```bash
kubectl apply -f examples/diagnosis/node/diagnosis-node.yaml
```

`diagnosis-node.yaml` 内容如下：

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

**步骤 2：监控诊断进度**

```bash
kubectl get aegisdiagnosises.aegis.io -n your-namespace --watch
```

执行成功后应看到如下输出：

```
NAME            PHASE       AGE
diagnose-node   Completed   38s
```

**步骤 3：查看诊断结果**

```bash
kubectl describe aegisdiagnosises.aegis.io -n your-namespace diagnose-node
```

输出示例：

```yaml
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
      - ...
      [gpfs.health]
      - <no data>
      [gpfs.log]
      - [SKIPPED] mmfs.log.latest not found
```

在该示例中，Collector 成功运行并采集了节点日志，包括内核消息与 GPFS 状态等内容。这些将以 `Info` 字段的形式呈现，为后续分析提供参考。

---

## Collector Pod 使用指南

👉 Collector Pod 机制允许用户使用自定义镜像与脚本，在节点上采集更多底层信息，提升诊断深度与灵活性。

默认情况下，系统使用内置镜像；若你有更复杂的需求（如采集定制日志、执行硬件检查等），可按以下方式自定义 Collector。

### 结构说明

自定义 Collector 需提供以下内容：

* 带有 `collectorConfig` 字段的 **诊断 CR**
* 包含采集逻辑的 **自定义镜像**
* 实际执行的脚本（如 `collect.sh`）

参考目录如下：

```
examples/diagnosis/node/collector/
├── collect.sh
├── Dockerfile.collector
└── diagnosis-node-custom-collector.yaml
```

### 📜 1. 采集脚本示例（`collect.sh`）

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

该脚本记录基本信息并将日志写入挂载路径 `/var/log/custom`。


### 🐳 2. Dockerfile 示例（`Dockerfile.collector`）

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y util-linux grep coreutils bash

COPY collect.sh /collector/collect.sh
RUN chmod +x /collector/collect.sh

CMD ["/bin/bash", "/collector/collect.sh"]
```

构建与推送：

```bash
docker build -f Dockerfile.collector -t myregistry/mycustom-collector:latest .
docker push myregistry/mycustom-collector:latest
```

### 📦 3. 自定义诊断 CR（`diagnosis-node-custom-collector.yaml`）

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

该 CR 将使用你的镜像与命令，在目标节点上启动 Collector Pod，并将 `/var/log/custom` 中的日志内容纳入诊断结果。
