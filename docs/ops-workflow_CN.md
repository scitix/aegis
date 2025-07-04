# 标准化集群自动运维工作流（Standard Automated Cluster Operations Workflow）

本文档阐述在 Aegis 中**定义并触发自动化集群运维**的标准流程。该流程为事件驱动：当告警被接收后，系统将其与预定义规则进行匹配，并通过 Argo 自动触发运维工作流。

---

## 📋 步骤 1：定义 SOP（标准操作流程）

首先编写一个基础的 Shell 命令或脚本，用以描述期望的运维行为。例如：隔离节点（cordon）：

```bash
kubectl cordon $node
```

一旦告警匹配，此命令将在自动化工作流中执行。

---

## ⚙️ 步骤 2：定义 Ops Rule 与 Template

你需要同时定义 `AegisAlertOpsRule` 和 `AegisOpsTemplate`：

* **OpsRule**：将告警类型与工作流模板关联。
* **OpsTemplate**：描述实际执行步骤（Argo Workflow）。

### 示例 `rule.yaml`

```yaml
---
apiVersion: aegis.io/v1alpha1
kind: AegisAlertOpsRule
metadata:
  name: nodehasemergencyevent
spec:
  alertConditions:
  - type: NodeHasEmergencyEvent
    status: Firing
  opsTemplate:
    kind: AegisOpsTemplate
    apiVersion: aegis.io/v1alpha1
    namespace: monitoring
    name: nodehasemergencyevent
---
apiVersion: aegis.io/v1alpha1
kind: AegisOpsTemplate
metadata:
  name: nodehasemergencyevent
spec:
  manifest: |
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    spec:
      serviceAccountName: aegis-workflow
      ttlSecondsAfterFinished: 60
      entrypoint: start
      templates:
      - name: start
        retryStrategy:
          limit: 1
        container:
          image: bitnami/kubectl:latest
          command:
          - /bin/bash
          - -c
          - |
            kubectl cordon {{.node}}
```

> `{{.node}}` 会根据告警上下文动态渲染。

---

## 🚀 步骤 3：部署规则

```bash
kubectl apply -f rule.yaml
```

验证规则与模板是否已注册：

```bash
kubectl get aegisalertopsrule
kubectl get aegisopstemplate
```

---

## 📡 步骤 4：触发自动化运维

向 Aegis 控制器发送测试告警，模拟真实故障：

```bash
curl -X POST http://127.0.0.1:8080/default/alert -d '{
    "type": "NodeHasEmergencyEvent",
    "status": "Firing",
    "involvedObject": {
        "Kind": "Node",
        "Name": "dev1"
    },
    "details": {
        "startAt": "2022021122",
        "node": "dev1"
    },
    "fingerprint": "5f972974ccf1ee9b"
}'
```

---

## 📊 步骤 5：监控告警生命周期

实时观察告警在 Aegis 中的流转：

```bash
kubectl -n monitoring get aegisalert --watch | grep default
```

常见状态流转：

```
Pending → Triggered → Running → Succeeded
```

---

## 🧾 步骤 6：检查工作流执行情况

查看渲染后的 Argo Workflow：

```bash
kubectl -n monitoring get workflow | grep nodehasemergencyevent
```

查看相关 Pod：

```bash
kubectl -n monitoring get pods | grep nodehasemergencyevent
```

最后，检查执行日志：

```bash
kubectl -n monitoring logs <POD_NAME>
```

预期输出示例：

```
node/dev1 cordoned
```
