# Standard Automated Cluster Operations Workflow

This document explains the standard process for defining and triggering automated cluster operations in Aegis. The workflow is event-driven—when an alert is received, it is matched against predefined rules and automatically triggers operational workflows via Argo.

## 📋 Step 1: Define SOP (Standard Operating Procedure)

Write a basic shell command or script that defines the expected operational behavior. For example, cordoning a node:

```bash
kubectl cordon $node
```

This command will be executed in the automated workflow once the alert is matched.


## ⚙️ Step 2: Define Ops Rule and Template

You need to define both an `AegisAlertOpsRule` and an `AegisOpsTemplate`. The rule links the alert type to the workflow template, and the template describes the actual steps to execute.

### Sample `rule.yaml`

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

In the template, `{{.node}}` will be automatically rendered from the alert context.

## 🚀 Step 3: Deploy the Rule

```bash
kubectl apply -f rule.yaml
```

Then verify that the rule and template have been registered:

```bash
kubectl get aegisalertopsrule
kubectl get aegisopstemplate
```

---

## 📡 Step 4: Trigger Automated Ops

Send a test alert to the Aegis controller to simulate a real failure:

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


## 📊 Step 5: Monitor the Alert Lifecycle

You can watch how the alert flows through the Aegis system:

```bash
kubectl -n monitoring get aegisalert --watch | grep default
```

You’ll see transitions like:

```
Pending → Triggered → Running → Succeeded
```



## 🧾 Step 6: Inspect Workflow Execution

Check the rendered Argo Workflow:

```bash
kubectl -n monitoring get workflow | grep nodehasemergencyevent
```

Inspect the associated pods:

```bash
kubectl -n monitoring get pods | grep nodehasemergencyevent
```

And finally, verify the execution logs:

```bash
kubectl -n monitoring logs <POD_NAME>
```

Expected output:

```
node/dev1 cordoned
```
