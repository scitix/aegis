# 告警来源集成（Alert Source Integration）

Aegis 支持接收来自多种监控与可观测性系统的告警信息，并统一转换为标准的 **AegisAlert** 对象。本文档介绍了 Aegis 暴露的三个告警接收端点（HTTP 接口）、所需的消息格式，以及如何配置常见的告警来源。

> **为什么提供三个端点？** 不同系统生态已有约定俗成的消息格式。Aegis 提供专用接收端点以适配多种格式，同时最终统一转为标准模型。

---

## 接口概览

| 接口地址                       | 用途                                    | 典型来源系统                                                                                         |
| -------------------------- | ------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `POST /ai/alert`           | 通过 **AIAlertParser**（LLM 支持）解析任意 JSON | Datadog、Elastic、Sysdig、New Relic、Zabbix、Nagios、VictoriaMetrics、Kubernetes Event Exporter、自定义系统 |
| `POST /alertmanager/alert` | 原生支持 **Alertmanager** 的 Webhook 格式    | Prometheus Alertmanager 及兼容系统                                                                  |
| `POST /alert`              | 精简的自定义 JSON 格式，适合轻量级接入                | 内部脚本、自建工具                                                                                      |

---

## 1  AIAlertParser (`/ai/alert`)

通过 LLM 模型将任意结构的 JSON 告警转换为 AegisAlert 统一格式。完整的提示词构建和映射策略请见 [docs/ai-alert-parse.md](docs/ai-alert-parse.md)。

```bash
curl -X POST http://aegis.monitoring:8080/ai/alert \
     -H "Content-Type: application/json" \
     -d '{
       "title": "High CPU Usage on node01",
       "text": "CPU usage is above 90% on node01.",
       "priority": "P1",
       "host": "node01",
       "timestamp": "2025-06-04T04:00:00Z"
     }'
```

启用该接口需挂载 LLM 配置 ConfigMap（见 `deploy/llm-config.yaml`），并在控制器启动参数中加入 `ai` 或 `openai`。

---

## 2  Alertmanager Webhook (`/alertmanager/alert`)

Aegis 原生兼容 [Alertmanager v4 Webhook 规范](https://prometheus.io/docs/alerting/latest/alertmanager/)。以下配置可将告警推送至 Aegis：

```yaml
# alertmanager.yaml
receivers:
  - name: aegis
    webhook_configs:
      - url: http://aegis.monitoring:8080/alertmanager/alert
        send_resolved: true
route:
  receiver: aegis
  group_by: [alertname]
  group_wait: 0s
  group_interval: 5m
  repeat_interval: 12h
```

Aegis 端无需额外配置，只需配置 Alertmanager 向 webhook 推送即可。

---

## 3  自定义 JSON (`/alert`)

如告警来源可控，建议使用如下结构化、显式且无嵌套结构的轻量级格式：

```go
// Golang 定义

type Alert struct {
    AlertSourceType AlertSourceType // 如 "Custom"
    Type            string              `json:"type"`    // 告警名称
    Status          string              `json:"status"`  // Firing | Resolved
    InvolvedObject  AlertInvolvedObject `json:"involvedObject"`
    Details         map[string]string   `json:"details"`
    FingerPrint     string              `json:"fingerprint"`
}

type AlertInvolvedObject struct {
    Kind      string `json:"kind"`      // Node | Pod 等
    Name      string `json:"name"`
    Namespace string `json:"namespace"`
    Node      string `json:"node"`
}
```

### 示例请求

```bash
curl -X POST http://aegis.monitoring:8080/alert \
     -H "Content-Type: application/json" \
     -d '{
       "type": "NodeOutOfDiskSpace",
       "status": "Firing",
       "involvedObject": {
         "kind": "Node",
         "name": "node1"
       },
       "details": {
         "startAt": "2022-02-11T22:00:00Z",
         "node": "node1"
       },
       "fingerprint": "5f972974ccf1ee9b"
     }'
```

---

## 来源系统建议接入方式

| 告警来源系统                                                         | 推荐接入接口                | 说明                     |
| -------------------------------------------------------------- | --------------------- | ---------------------- |
| Prometheus Alertmanager                                        | `/alertmanager/alert` | 高度兼容 Alertmanager 标准格式 |
| Datadog、Elastic、Sysdig、New Relic、Zabbix、Nagios、VictoriaMetrics | `/ai/alert`           | 自动解析异构 JSON 结构         |
| Kubernetes Event Exporter                                      | `/ai/alert`           | 无需修改 Helm，即可兼容         |
| 自定义脚本/工具                                                       | `/alert`              | 接口轻量，易于序列化             |
