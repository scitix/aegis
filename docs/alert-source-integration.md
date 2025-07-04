# Alert Source Integration

Aegis accepts alerts from multiple monitoring and observability systems and normalises them into a unified **AegisAlert** object.  This document describes the three HTTP endpoints that Aegis exposes for ingesting alert messages, the expected payloads, and how to configure common sources.

> **Why three endpoints?**  Different ecosystems already have de‑facto payload formats.  Instead of forcing every source through a single schema, Aegis offers purpose‑built entry points while still converging on the same internal model.

---

## Endpoints Overview

| Endpoint                   | Purpose                                                      | Typical Source                                                                                                   |
| -------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `POST /ai/alert`           | Parse arbitrary JSON via **AIAlertParser** (LLM‑powered).    | Datadog, Elastic, Sysdig, New Relic, Zabbix, Nagios, VictoriaMetrics, Kubernetes Event Exporter, bespoke systems |
| `POST /alertmanager/alert` | Native **Alertmanager** webhook format.                      | Prometheus Alertmanager & compatibles                                                                            |
| `POST /alert`              | Minimal **Custom JSON** format for lightweight integrations. | Internal scripts & tools                                                                                         |

---

## 1  AIAlertParser (`/ai/alert`)

Send any JSON payload and let the LLM convert it into the AegisAlert schema.  The full prompt‑construction and mapping strategy are documented in [docs/ai-alert-parse.md](docs/ai-alert-parse.md).

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

To enable this endpoint, mount an LLM configuration ConfigMap (see `deploy/llm-config.yaml`) and add the argument `ai` (or `openai`) to the controller deployment.


## 2  Alertmanager Webhook (`/alertmanager/alert`)

Aegis natively understands the [Alertmanager v4 webhook spec](https://prometheus.io/docs/alerting/latest/alertmanager/).  The snippet below forwards all alerts to Aegis:

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

No additional configuration is required on the Aegis side—just point Alertmanager to the webhook.


## 3  Custom JSON (`/alert`)

For scenarios where you control the emitting code, use the lightweight schema below.  It is versioned, explicit, and free of nested structures.

```go
// Golang definition

type Alert struct {
    AlertSourceType AlertSourceType // e.g. "Custom"
    Type            string              `json:"type"`    // Alert name
    Status          string              `json:"status"`  // Firing | Resolved
    InvolvedObject  AlertInvolvedObject `json:"involvedObject"`
    Details         map[string]string   `json:"details"`
    FingerPrint     string              `json:"fingerprint"`
}

type AlertInvolvedObject struct {
    Kind      string `json:"kind"`      // Node | Pod | ...
    Name      string `json:"name"`
    Namespace string `json:"namespace"`
    Node      string `json:"node"`
}
```

### Example request

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

## Source‑Specific Notes

| Source                                                               | Recommended Endpoint  | Notes                                                 |
| -------------------------------------------------------------------- | --------------------- | ----------------------------------------------------- |
| Prometheus Alertmanager                                              | `/alertmanager/alert` | Full fidelity ingestion.                              |
| Datadog, Elastic, Sysdig, New Relic, Zabbix, Nagios, VictoriaMetrics | `/ai/alert`           | AI parser handles differing JSON structures.          |
| Kubernetes Event Exporter                                            | `/ai/alert`           | Works out‑of‑the‑box; no Helm chart changes required. |
| Home‑grown scripts                                                   | `/alert`              | Small schema, easy to serialise.                      |
