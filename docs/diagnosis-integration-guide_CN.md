# Aegis Diagnosis 集成指南

**Aegis Diagnosis** 系统通过 Kubernetes 的 CRD（自定义资源定义）机制，提供了一种简单且可扩展的资源诊断能力。
第三方系统只需创建对应的 CR 对象即可发起诊断任务，无需了解 Aegis 内部的实现细节。

---

**架构与流程**

* Aegis Diagnosis 系统基于 Kubernetes CRD 机制实现。
* 第三方系统通过创建 `AegisDiagnosis` CR 对象，指定诊断目标对象（类型、名称、命名空间等）。创建方式包括：

  * 手动编写并 apply YAML。
  * 通过 API 编程方式动态创建 CR 对象（如通过 Kubernetes `dynamicClient`，很多平台采用此方式）。
* 第三方系统通常通过 API 轮询诊断状态，并将结果展示给用户。

---

**支持的诊断对象类型**

（当前支持的对象类型包括但不限于：）

* Node
* Pod
* PytorchJob（参考 [Kubeflow PytorchJob](https://www.kubeflow.org/docs/components/training/overview/#pytorchjob)）

后续版本将支持更多对象类型。

---

**诊断任务生命周期**

诊断任务通常经历以下阶段：

1. `Pending` — 任务已创建但尚未开始执行。
2. `Diagnosing` — 正在诊断中。
3. `Completed` — 诊断成功完成。
4. `Failed` — 诊断执行失败。
5. `Unknown` — 初始或状态未知阶段。

---

**CR 对象定义与示例**

`AegisDiagnosis` CRD 定义如下（简化版）：

```yaml
apiVersion: aegis.io/v1alpha1
kind: AegisDiagnosis
metadata:
  name: pytorchjob-test
  namespace: monitoring
spec:
  object:
    kind: PytorchJob
    name: llama2-13b-tp2-pp1-np08-gbs256-hpe-node144-0
    namespace: default
```

示例结果：

```yaml
status:
  phase: Completed
  result:
    failures: ["..."]
    warnings: ["..."]
    infos: ["..."]
  explain: "Healthy: No\nError: ...\nSolution: ..."
  errorResult: ""
```

创建诊断任务时需要关注的字段：

| 字段名                   | 说明                            |
| --------------------- | ----------------------------- |
| spec.object.kind      | 目标对象类型（如 Pod、Node、PytorchJob） |
| spec.object.name      | 目标对象名称                        |
| spec.object.namespace | 目标对象命名空间（如适用）                 |
| spec.timeout（可选）      | 诊断超时时间（如 `60s`，`5m`）          |
| spec.ttlStrategy（可选）  | 诊断完成后的自动清理策略（TTL）             |

---

**Aegis Diagnosis APIs**

### 1. 创建诊断任务

* **URL**: `/api/v1/cks/diagnosis/create`
* **Method**: `POST`
* **功能**: 创建新的诊断任务。

请求示例：

```json
POST /api/v1/cks/diagnosis/create
{
  "clusterId": "osuo9o1b0xu35dc2",
  "type": "Pod",
  "name": "my-app-pod-0",
  "namespace": "my-app"
}
```

响应示例：

```json
{
  "id": "dt-xxxx"
}
```

---

### 2. 查询诊断任务

* **URL**: `/api/v1/cks/diagnosis/get`
* **Method**: `GET`
* **功能**: 查询指定诊断任务的状态与结果。

请求示例：

```
GET /api/v1/cks/diagnosis/get?clusterId=osuo9o1b0xu35dc2&id=dt-xxxx
```

响应示例：

```json
{
  "id": "dt-xxxx",
  "object": {
    "kind": "Pod",
    "name": "ib-hca-exporter-f2jfc",
    "namespace": "monitoring"
  },
  "startTime": "2025-05-19T08:03:27Z",
  "completionTime": "2025-05-19T08:03:35Z",
  "phase": "Completed",
  "result": {
    "failures": ["..."],
    "warnings": ["..."],
    "infos": ["..."]
  },
  "explain": "Healthy: No\nError: ...\nSolution: ...",
  "errorResult": "",
  "status": "Completed"
}
```

---

### 3. 批量查询诊断任务

* **URL**: `/api/v1/cks/diagnosis/list`
* **Method**: `GET`
* **功能**: 批量查询集群内历史诊断任务。

请求示例：

```
GET /api/v1/cks/diagnosis/list?clusterId=osuo9o1b0xu35dc2
```

响应示例：返回一组诊断任务对象，字段格式与 `GetDiagnosisTask` 返回一致。

---

**平台推荐集成流程**

1. 平台 UI 触发 `/diagnosis/create`，发起诊断请求。
2. 平台获取返回的诊断任务 ID（如 `dt-xxxx`）。
3. 平台轮询 `/diagnosis/get` 检查诊断状态：

   * `Pending` 或 `Diagnosing`：继续轮询。
   * `Completed` 或 `Failed`：停止轮询，展示结果。
4. 平台可以提供历史诊断任务查询界面，接入 `/diagnosis/list`。

**前置准备：**

* 平台需具备调用 CKS API 的能力。
* 平台需传递用户 / 租户 / 团队信息以支持权限校验。
* 推荐通过轮询或异步通知机制获取诊断结果。

---

**诊断结果展示建议**

* `explain` 字段为核心的用户可读诊断摘要，建议重点展示。
* `result.failures` / `warnings` / `infos` 可分类分区展示在 UI 界面中。
* 推荐增加 “查看原始 YAML” 功能，展示 `AegisDiagnosis` 资源原文。

内部结构示例：

```yaml
apiVersion: aegis.io/v1alpha1
kind: AegisDiagnosis
metadata:
  name: dt-xxxx
  namespace: monitoring
spec:
  object:
    kind: Pod
    name: my-app-pod-0
    namespace: my-app
status:
  phase: Completed
  result:
    failures: [...]
    warnings: [...]
    infos: [...]
  explain: "..."
  errorResult: ""
```

