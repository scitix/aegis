# AI Alert Parser

The AI Alert Parser provides a flexible way to integrate alerts from various external systems with different JSON formats into the Aegis unified alert model (`AegisAlert`).

It leverages LLM-based parsing to automatically transform raw alert messages into the required format, significantly reducing the cost of maintaining case-sensitive or fixed mappings across multiple systems.

## Supported API

* `POST /ai/alert`
  Accepts arbitrary JSON alert messages and parses them into standardized `AegisAlert` objects using an LLM-based parser.

## How it works

* External systems send alerts to `/ai/alert` via HTTP POST.
* The AIAlertParser builds an LLM prompt based on the raw alert.
* The LLM converts the alert into a standardized `AegisAlert` JSON.
* The controller validates and stores the parsed `AegisAlert`.

## Supported Sources

* Alertmanager
* Datadog
* Elastic Stack
* Sysdig
* New Relic
* Zabbix
* Nagios
* VictoriaMetrics
* Kubernetes Event Exporter
* Other custom systems

## Example Requests

### Alertmanager Example

<details>
<summary>Click to expand</summary>

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

### Datadog Example

<details>
<summary>Click to expand</summary>

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

### Kubernetes Event Exporter

<details>
<summary>Click to expand</summary>

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

## AI Alert Parser Configuration

First, prepare a ConfigMap to configure the LLM, like [this example](../deploy/llm-config.yaml).

Next, add the argument `ai` to the container's start arguments in your Deployment. (In this project, the default argument is `openai`.)

After completing these steps, you can use the `/ai/alert` API to parse alert messages.
