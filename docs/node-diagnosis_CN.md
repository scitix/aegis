# AI 驱动的节点诊断

**AI 驱动的节点诊断**功能会自动分析 Kubernetes 节点的健康状态，并生成结构化的 Prompt，以支持大型语言模型 (LLM) 进行诊断分析。

它通过联合节点条件、事件信息、系统底层记录，进行综合分析，帮助用户快速辨别节点的问题和情况。

---

## 设计结构与流程

完整诊断流程包括下列步骤：

### 1. 确定目标节点

* 根据指定名称查找节点对象，验证其是否存在。

### 2. 诊断数据采集

#### **Failure**

* 分析节点的 **Condition** 状态，如 `NotReady`，`MemoryPressure`，`DiskPressure`等。
* 支持两种数据来源：

  * 如果启用 Prometheus，则通过 Prometheus 查询节点条件指标。
  * 否则从 Kubernetes API 获取 `status.conditions`。
* 一切不健康的 Condition 都会被记录为 *Failure* 类型。

#### **Warning**

* 获取与节点相关的 Kubernetes **事件 (Event)**。
* 支持两种方式：

  * 如果启用 Prometheus 事件导出，则使用 Prometheus 获取事件。
  * 否则使用 Kubernetes API 直接获取。
* 被分类记录为 *Warnings*。

#### **Info**

* 会在节点上启动一个 **Collector Pod** ，以收集系统底层记录。
* Collector Pod 会运行自定义的镜像和进入点脚本，输出内容会被分析成 *Info* 类型。
* Collector Pod 镜像通过主控管理器 (Aegis controller) 启动参数综合配置，如：

```yaml
aegs-controller:
  args:
    - --diagnosis.collectorImage=myregistry/mycustom-collector:latest
```

* 默认镜像为：

```text
registry-ap-southeast.scitix.ai/k8s/collector:v1.0.0
```

* 更多内容请参见 [Collector Pod 使用指南](#collector-pod-guide)。

---

### 3. AI Prompt 构造

将所有诊断数据收集合成一个结构化的 Prompt，使大型语言模型进行诊断分析。

Prompt 包括：

* 角色设定：将 AI 设定为 "节点诊断工程师"
* 说明指令：指引 AI 如何分析提供的数据
* 节点信息：

  * *Errors* 条件失效
  * *Warnings* 事件
  * *Infos* Collector Pod 输出
* 返回格式：

  * Healthy
  * Error
  * Analysis
  * Solution

---

## 实例：使用内置 Collector 诊断节点

### Step 1: 创建 Diagnosis CR

```bash
kubectl apply -f examples/diagnosis/node/diagnosis-node.yaml
```

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

### Step 2: 监控诊断执行

```bash
kubectl get aegisdiagnosises.aegis.io -n your-namespace --watch
```

完成后，将看到状态为 `Completed`：

```
NAME            PHASE       AGE
diagnose-node   Completed   38s
```

### Step 3: 查看结果

```bash
kubectl describe aegisdiagnosises.aegis.io -n your-namespace diagnose-node
```

示例输出：

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
      [gpfs.health]
      - <no data>
      [gpfs.log]
      - [SKIPPED] mmfs.log.latest not found
```

---

## Collector Pod Guide

**Collector Pod** 方案支持在节点上启动自定义镜像和脚本执行诊断分析。

默认使用公用镜像，如需指定自己镜像，需在 aegis controller 启动参数中指定。

### 1. 配置 Collector 镜像

在 aegis controller 的 deployment.yaml 中指定

```yaml
args:
  - --diagnosis.collectorImage=myregistry/mycustom-collector:latest
```

### 2. Collector 镜像要求

用户自定义的镜像需要：

* 包含输入点脚本 (collect.sh)
* 配备 bash 、coreutils 等基本工具
* 将输出日志写入 `/var/log/custom/diagnosis.log`

### 3. 示例 collect.sh

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

### 4. 镜像 Dockerfile 示例

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y util-linux grep coreutils bash

COPY collect.sh /collector/collect.sh
RUN chmod +x /collector/collect.sh

CMD ["/bin/bash", "/collector/collect.sh"]
```

构建并上传：

```bash
docker build -f Dockerfile.collector -t myregistry/mycustom-collector:latest .
docker push myregistry/mycustom-collector:latest
```
