# Custom Prompt Guide for Aegis Diagnosis

Aegis supports **customizable prompt** for all diagnosis types to allow flexible and context-aware LLM analysis output.

## How It Works

1. The system checks for an override prompt in the ConfigMap named `aegis-prompt`.
2. File path is `/aegis/prompt/` inside the container.
3. If present, the custom prompt will **replace the default system prompt** for the corresponding diagnosis type.

## How to Define

### ⚠️ Important: Enable Prompt ConfigMap in Deployment

To use custom prompts, make sure:

1. The `ConfigMap` is created in your cluster (e.g., `aegis-prompts`).
2. Your deployment includes the corresponding `volumes` and `volumeMounts` entries, like below:

```yaml
volumeMounts:
  - name: prompt-config
    mountPath: /aegis/prompt/
    readOnly: true

volumes:
  - name: prompt-config
    configMap:
      name: aegis-prompts
```

If these are commented out in your deployment YAML (they are by default), **you must uncomment them** to activate the feature.

> Otherwise, the prompt loader may fail to find the expected path `/aegis/prompt/`, causing container startup issues or runtime errors during diagnosis.

### Example ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aegis-prompts
data:
  pytorchjob.tmpl: |
    You are a Kubernetes + Kubeflow diagnostic expert...
    ...
  node.tmpl: |
    You are a node diagnosis assistant...
    ...
  pod.tmpl: |
    You are a pod diagnosis assistant...
    ...
````

Each key (e.g. `pytorchjob.tmpl`, `node.tmpl`) corresponds to a diagnosis type.

> We provide an example custom prompt for PyTorchJob [here](../deploy/prompt-config.yaml).

## Supported Diagnosis Types

* [Node](./node-diagnosis.md#available-variables)
* [Pod](./pod-diagnosis.md#available-variables)
* [PyTorchJob](./pytorchjob-diagnosis.md#available-variables)

Click each type above to view its **available template variables**.
