# AI 告警解析器

AI 告警解析器提供了一种灵活的方式，将来自不同外部系统、格式各异的 JSON 告警消息，转换为统一的 Aegis 告警模型（`AegisAlert`）。

该解析器通过调用 LLM 自动解析原始告警消息，显著降低了维护大小写敏感或固定字段映射的成本。

## 支持的 API

* `POST /ai/alert`
  接收任意 JSON 格式的告警消息，使用 LLM 解析为标准化的 `AegisAlert` 对象。

## 工作流程

* 外部系统通过 HTTP POST 将告警发送到 `/ai/alert`。
* AIAlertParser 根据原始告警内容构建 LLM Prompt。
* LLM 将告警转换为标准化的 `AegisAlert` JSON。
* 控制器验证并存储解析后的 `AegisAlert`。

## 支持的告警来源

* Alertmanager
* Datadog
* Elastic Stack
* Sysdig
* New Relic
* Zabbix
* Nagios
* VictoriaMetrics
* Kubernetes Event Exporter
* 其他自定义系统

## 示例请求

### Alertmanager 示例

<details>
<summary>点击展开</summary>

```bash
curl -X POST http://127.0.0.1:18080/ai/alert \
  -H "Content-Type: application/json" \
  -d '{
    "version": "4",
    "groupKey": "alert-group-odysseus004",
    "status": "firing",
    "receiver": "aegis",
    "groupLabels": {
      "alertname": "NodeOutOfDiskSpace"
    },
    "commonLabels": {
      "alertname": "NodeOutOfDiskSpace",
      "severity": "critical"
    },
    "commonAnnotations": {
      "summary": "Disk space issue detected on nodes"
    },
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "NodeOutOfDiskSpace",
          "kind": "Node",
          "instance": "odysseus004",
          "node": "odysseus004",
          "involved_object_name": "odysseus004",
          "namespace": "default"
        },
        "annotations": {
          "description": "Disk space is critically low on node odysseus004"
        },
        "startsAt": "2022-02-11T22:00:00Z",
        "endsAt": "0001-01-01T00:00:00Z",
        "fingerprint": "5f972974ccf1ee9b"
      }
    ]
  }'
```

</details>

---

### Datadog 示例

<details>
<summary>点击展开</summary>

```bash
curl -X POST http://127.0.0.1:18080/ai/alert \
  -H "Content-Type: application/json" \
  -d '{
    "title": "High CPU Usage on node01",
    "text": "CPU usage is above 90% on node01.",
    "priority": "P1",
    "alert_id": "123456",
    "tags": ["env:production", "service:nginx"],
    "host": "node01",
    "timestamp": "2025-06-04T04:00:00Z",
    "url": "https://app.datadoghq.com/logs?query=host:node01%20service:nginx"
  }'
```

</details>

---

### Kubernetes Event Exporter 示例

<details>
<summary>点击展开</summary>

```bash
curl -X POST http://127.0.0.1:18080/ai/alert \
  -H "Content-Type: application/json" \
  -d '{
    "event": {
      "type": "Warning",
      "reason": "Killing",
      "message": "Pod mypod was killed due to memory pressure",
      "source": {
        "component": "kubelet",
        "host": "node01"
      },
      "firstTimestamp": "2025-06-04T04:00:00Z",
      "lastTimestamp": "2025-06-04T04:05:00Z",
      "count": 1,
      "involvedObject": {
        "kind": "Pod",
        "name": "mypod",
        "namespace": "default",
        "uid": "abc123"
      },
      "metadata": {
        "namespace": "default",
        "name": "mypod",
        "uid": "abc123"
      }
    }
  }'
```

</details>

---

## AI 告警解析器配置

首先，准备一个用于配置 LLM 的 ConfigMap，参考 [示例配置](../deploy/llm-config.yaml)。

然后，在 Deployment 配置中，将启动参数增加 `ai`。
（本项目默认启动参数为 `openai`。）

完成以上配置后，即可通过 `/ai/alert` API 使用 AIAlertParser 解析告警消息。
