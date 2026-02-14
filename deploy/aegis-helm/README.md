# Aegis Helm Chart

AIOps framework for Kubernetes self-healing and diagnostics.

## 功能概述

| 模块 | 说明 | 默认状态 |
|---|---|---|
| **Alert** | 监听 Prometheus 告警，分发到对应 OpsTemplate 执行 | 始终启用 |
| **Healthcheck** | 定期对节点/集群执行健康检查 | `aegis.healthcheck.enable` |
| **Diagnosis** | 基于 LLM 对节点/Pod/PyTorchJob 进行故障诊断 | `aegis.diagnosis.enable` |
| **Device-Aware** | 设备级故障感知（GPU 卡、IB 端口等粒度） | `aegis.deviceAware.enable` |
| **Node Poller** | 主动轮询节点状态，结合 priority ConfigMap 触发自愈 | `aegis.nodePoller.enable` |
| **Self-Healing** | 节点自愈工作流（Argo Workflow + selfhealing 二进制） | `selfhealing.enable` |

---

## 前置依赖

- Kubernetes >= 1.19
- [Argo Workflows](https://argoproj.github.io/argo-workflows/)（自愈模块需要）
- Prometheus + [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)（告警、健康检查、Node Poller 需要）
- Prometheus Operator（`ServiceMonitor` / `PrometheusRule` 需要）

---

## 安装

```bash
# 首次安装
helm install aegis ./deploy/aegis-helm \
  -n monitoring --create-namespace \
  -f my-values.yaml

# 升级
helm upgrade aegis ./deploy/aegis-helm \
  -n monitoring \
  -f my-values.yaml
```

---

## 参数说明

### 全局参数

| 参数 | 说明 | 默认值 |
|---|---|---|
| `registry` | 所有镜像的 registry 地址 | `registry-ap-southeast.scitix.ai` |
| `cluster` | 集群名称，写入告警 system-parameters | `arise` |
| `region` | 云区域标识，传给自愈二进制 | `""` |
| `orgname` | 租户/组织标识，传给 ticket 系统 | `""` |

### Ingress

| 参数 | 说明 | 默认值 |
|---|---|---|
| `ingress.enabled` | 是否创建 Ingress | `true` |
| `ingress.host` | Ingress 域名 | `console.scitix.ai` |
| `ingress.path` | Ingress 路径前缀 | `/scitix/arise/aegis` |

### Aegis 主框架（`aegis.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.image.repository` | 主框架镜像 repository | `k8s/aegis` |
| `aegis.image.tag` | 主框架镜像 tag | `v3.0.0-6c7698e` |
| `aegis.replicas` | Deployment 副本数 | `2` |
| `aegis.tolerations` | Pod 调度容忍 | control-plane NoSchedule |
| `aegis.nodeSelector` | Pod 节点选择器 | `{}` |
| `aegis.affinity` | Pod 亲和性（默认反亲和，避免两副本同节点） | podAntiAffinity |

#### 告警（`aegis.alert.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.alert.publishNamespace` | AegisAlert 对象发布到哪个 namespace | `monitoring` |
| `aegis.alert.ttlAfterSucceed` | 成功后 TTL（秒） | `86400` |
| `aegis.alert.ttlAfterFailed` | 失败后 TTL（秒） | `259200` |
| `aegis.alert.ttlAfterNoOps` | 无操作后 TTL（秒） | `86400` |

#### 健康检查（`aegis.healthcheck.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.healthcheck.enable` | 启用健康检查 | `true` |

#### 诊断（`aegis.diagnosis.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.diagnosis.enable` | 启用 LLM 诊断 | `true` |
| `aegis.diagnosis.explain` | 是否输出诊断解释 | `true` |
| `aegis.diagnosis.cache` | 是否缓存诊断结果 | `true` |
| `aegis.diagnosis.language` | 诊断输出语言（`chinese` / `english`） | `chinese` |
| `aegis.diagnosis.collector.repository` | 日志采集器镜像 repository | `k8s/aegis-collector` |
| `aegis.diagnosis.collector.tag` | 日志采集器镜像 tag | `v1.0.1` |
| `aegis.diagnosis.podLog.fetchLines` | 每容器拉取日志行数（0 = 默认 1000） | `0` |
| `aegis.diagnosis.podLog.keywords` | 日志过滤关键词列表（空 = 不过滤） | `[]` |
| `aegis.diagnosis.podLog.maxOutputLines` | 传给 LLM 的最大行数（0 = 默认 60） | `0` |

#### 设备感知（`aegis.deviceAware.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.deviceAware.enable` | 启用设备级故障感知 | `false` |

#### Node Poller（`aegis.nodePoller.*`）

主动轮询节点 Prometheus 状态，结合 `aegis-priority` ConfigMap 确定自愈优先级。

| 参数 | 说明 | 默认值 |
|---|---|---|
| `aegis.nodePoller.enable` | 启用 Node Poller | `false` |
| `aegis.nodePoller.priorityConfigmap` | priority 配置的 ConfigMap 名称 | `aegis-priority` |
| `aegis.nodePoller.priorityNamespace` | priority ConfigMap 所在 namespace | `monitoring` |

> 启用 Node Poller 时，建议同时启用 `selfhealing.enable`，以便自动创建 `aegis-priority` ConfigMap。

### Prometheus（`prometheus.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `prometheus.endpoint` | Prometheus server 地址 | `""` |
| `prometheus.token` | Prometheus API token | `""` |

### AI（`ai.*`）

用于 Diagnosis 模块的 LLM 后端配置。

| 参数 | 说明 | 默认值 |
|---|---|---|
| `ai.enabled` | 启用 AI 诊断 | `true` |
| `ai.provider` | LLM 提供商（`openai` / `anthropic` 等） | `openai` |
| `ai.model` | 模型名称 | `""` |
| `ai.password` | API Key | `""` |
| `ai.baseUrl` | API Base URL（自建/代理时使用） | `""` |
| `ai.temperature` | 生成温度 | `0.7` |
| `ai.topp` | Top-P | `0.5` |
| `ai.topk` | Top-K | `50` |
| `ai.maxtokens` | 最大生成 token 数 | `32768` |

### 节点自愈（`selfhealing.*`）

自愈模块默认关闭（`selfhealing.enable: false`），启用后会额外渲染以下资源：

- `ConfigMap/aegis-priority`：条件优先级配置
- `AegisAlertOpsRule/selfhealing-node-critical-issue`：监听 `NodeCriticalIssue` 告警
- `AegisOpsTemplate/selfhealing-node`：拉起自愈 Job 的 Argo Workflow 模板
- `AegisAlertOpsRule/todo-nodecheck` + `AegisOpsTemplate/todo-nodecheck`：处理节点 precheck 污点
- `PrometheusRule/todo-nodecheck-rulers`：监测 `scitix.ai/nodecheck` 污点的告警规则
- `Secret/aegis-selfhealing`（仅当 `selfhealing.ticket.enabled: true`）：ticket 系统凭据

#### 基础配置

| 参数 | 说明 | 默认值 |
|---|---|---|
| `selfhealing.enable` | 总开关 | `false` |
| `selfhealing.image.repository` | 自愈二进制镜像 repository | `k8s/aegis-selfhealing` |
| `selfhealing.image.tag` | 自愈二进制镜像 tag | `""` |
| `selfhealing.opsImage.repository` | Ops Job 执行镜像 repository（registry 复用全局） | `k8s/aegis` |
| `selfhealing.opsImage.tag` | Ops Job 执行镜像 tag | `""` |
| `selfhealing.level` | 自愈激进度（0 = 保守，>0 = 激进） | `0` |
| `selfhealing.hostNetwork` | 自愈 Job 是否使用 hostNetwork | `false` |
| `selfhealing.dnsPolicy` | 自愈 Job DNS 策略 | `ClusterFirst` |
| `selfhealing.precheckTaints` | 逗号分隔的 taint key 列表，命中则先 cordon 并移除污点 | `scitix.ai/nodecheck` |

#### 硬件类型开关

控制 `aegis-priority` ConfigMap 中写入哪些 condition 条目，关闭后对应类型的故障不参与自愈调度。

| 参数 | 涵盖条件 | 默认值 |
|---|---|---|
| `selfhealing.node` | NodeNotReady / NodeCordon / NodeFrequentDown 等 | `true` |
| `selfhealing.cpu` | CPUPressure / CpuUnhealthy | `true` |
| `selfhealing.memory` | MemoryPressure / MemoryUnhealthy / KubeletMemoryPressure | `true` |
| `selfhealing.disk` | DiskPressure / DiskUnhealthy | `true` |
| `selfhealing.network` | NetworkLinkDown | `true` |
| `selfhealing.baseboard` | BaseBoardCriticalIssue | `true` |
| `selfhealing.gpu` | GpuHung / GpuCheckFailed / Xid79GPULost 等 20+ 条件 | `true` |
| `selfhealing.ib` | IBLost / IBModuleLost / IBLinkAbnormal 等 | `true` |
| `selfhealing.roce` | RoceDeviceBroken / RoceHostOffline 等 | `true` |
| `selfhealing.gpfs` | GpfsDown / GpfsMountLost / GpfsNotMounted 等 | `true` |

#### 设备插件选择器（`selfhealing.pluginSelector.*`）

用于定位设备插件 DaemonSet Pod，以判断设备就绪状态。

| 参数 | 说明 | 默认值 |
|---|---|---|
| `selfhealing.pluginSelector.gpu` | GPU 设备插件 Pod 名称匹配 | `nvidia-device-plugin-ds` |
| `selfhealing.pluginSelector.roce` | RoCE 设备插件 Pod 名称匹配 | `sriovdp` |
| `selfhealing.pluginSelector.rdma` | RDMA 设备插件 Pod 名称匹配 | `rdma-shared-dp-ds` |

#### Ticket 系统（`selfhealing.ticket.*`）

| 参数 | 说明 | 默认值 |
|---|---|---|
| `selfhealing.ticket.enabled` | 是否创建 ticket 凭据 Secret | `true` |
| `selfhealing.ticket.type` | Ticket 系统类型（如 `Scitix`，留空则使用 Node annotation） | `""` |
| `selfhealing.ticket.endpoint` | Ticket API 地址 | `""` |
| `selfhealing.ticket.appid` | Ticket 应用 ID | `""` |
| `selfhealing.ticket.token` | Ticket API Token | `""` |
| `selfhealing.ticket.ranstr` | Ticket 随机盐值 | `""` |

#### SRE 分派（`selfhealing.sre.*`）

自愈失败时将工单派发给对应 SRE 组。

| 参数 | 说明 | 默认值 |
|---|---|---|
| `selfhealing.sre.default` | 默认 SRE 组 | `""` |
| `selfhealing.sre.hardware` | 硬件故障 SRE 组 | `""` |
| `selfhealing.sre.nonhardware` | 非硬件故障 SRE 组 | `""` |

---

## priority.conf 格式说明

`aegis-priority` ConfigMap 中的 `priority.conf` 采用四字段格式：

```
Condition:Priority:AffectsLoad:DeviceIDMode
```

| 字段 | 说明 |
|---|---|
| `Priority` | 数值越小优先级越高；`0` = NodeNotReady，`1` = NodeCordon，`≤99` = Emergency，`≤999` = CanIgnore |
| `AffectsLoad` | `true` = 影响节点可用负载（调度器/外部系统可感知）；`false` = 仅监控告警 |
| `DeviceIDMode` | `all` 全设备 / `index` 整数编号 / `mask` 位掩码 / `id` 标识符字符串 / `-` 不标记设备 |

---

## 典型部署示例

### 最小化（仅 AI 诊断）

```yaml
registry: my-registry.example.com
cluster: prod-cluster

aegis:
  image:
    tag: v3.0.0-6c7698e
  diagnosis:
    enable: true
    collector:
      tag: v1.0.1

ai:
  enabled: true
  provider: openai
  model: gpt-4o
  password: sk-xxx
  baseUrl: https://api.openai.com/v1

prometheus:
  endpoint: http://prometheus-k8s.monitoring:9090
```

### 启用节点自愈（含 GPU 故障处理）

```yaml
registry: my-registry.example.com
cluster: prod-cluster
region: ap-southeast
orgname: my-org

prometheus:
  endpoint: http://prometheus-k8s.monitoring:9090
  token: ""

aegis:
  image:
    tag: v3.0.0-6c7698e
  nodePoller:
    enable: true

selfhealing:
  enable: true
  image:
    tag: v3.0.0-xxx
  opsImage:
    tag: ops-xxx
  level: 0

  # 仅启用 GPU 和节点基础自愈
  node: true
  gpu: true
  baseboard: false
  cpu: false
  memory: false
  disk: false
  network: false
  ib: false
  roce: false
  gpfs: false

  ticket:
    enabled: false

  sre:
    default: sre-on-call
    hardware: sre-hardware
    nonhardware: sre-software
```
