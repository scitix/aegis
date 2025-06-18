# Aegis 诊断自定义提示词指南（Custom Prompt Guide）

Aegis 支持**所有诊断类型的提示词自定义功能**，以实现灵活且具上下文感知能力的大模型分析输出。

## 工作原理

1. 系统会检查是否存在名为 `aegis-prompt` 的 ConfigMap。
2. 提示词文件会被挂载到容器内的路径 `/aegis/prompt/`。
3. 如果找到对应类型的提示词模板文件，系统将使用该自定义模板**替代内置默认提示词**。

## 如何定义

### ⚠️ 重要提示：启用 Deployment 中的 Prompt ConfigMap 挂载配置

如果你希望使用自定义 Prompt 功能，请确保以下两点：

1. 已在集群中创建对应的 `ConfigMap`（例如：`aegis-prompts`）；
2. 在你的 Deployment 配置中，**取消注释**如下内容：

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

默认情况下，这部分挂载配置可能是被注释掉的。如果你不取消注释，容器在启动或执行诊断任务时将**找不到 `/aegis/prompt/` 路径或模板文件**，可能导致以下问题：

* 容器启动失败；
* 自定义 prompt 载入失败，导致诊断逻辑异常或输出为空。

> 请务必在启用自定义 prompt 前，核对 Deployment 是否正确挂载了 prompt ConfigMap。

### 示例 ConfigMap

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
```

每个键（例如 `pytorchjob.tmpl`、`node.tmpl`）对应一个诊断类型。

> 我们提供了一个 PyTorchJob 的自定义提示词示例：[点击查看](../deploy/prompt-config.yaml)。

## 支持的诊断类型

* [Node](./node-diagnosis_CN.md#可用变量)
* [Pod](./pod-diagnosis_CN.md#可用变量)
* [PyTorchJob](./pytorchjob-diagnosis_CN.md#可用变量)

点击上述链接可查看每种诊断类型所支持的**模板变量说明**。