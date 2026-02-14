# Selfhealing Helm Chart 合并方案

## 现状问题

当前 selfhealing 功能由两个独立 Helm Chart 分别管理：

| Chart | 目录 | 职责 |
|---|---|---|
| `aegis` | `deploy/aegis-helm` | 主框架：控制器、诊断、健康检查、告警分发 |
| `aegis-selfhealing` | `deploy/aegis-selfhealing-cks` | 节点自愈：OpsRule/OpsTemplate、priority 配置、ticket 凭据 |

这带来以下问题：

1. **参数冗余**：`registry`、`cluster`、`prometheus.endpoint/token` 在两个 chart 里各写一份，部署时需保持同步。
2. **拆分部署复杂**：用户需同时维护两套 `values.yaml`，且安装顺序有依赖（主框架先装，selfhealing 后装）。
3. **死参数残留**：`aegis-selfhealing` 中传给 selfhealing 二进制的 `--registry`/`--repository` 两个 flag 是死参数——所有 Job 模板（`healthcheck_node.yaml`、`repair_node.yaml` 等共 6 个）只使用 `{{.image}}`，从不引用这两个变量，且 `values.yaml` 里也没有顶层 `repository` 字段（渲染为空）。
4. **命名风格不统一**：`selfhealing-cks` 使用 `plugin_selector`（snake_case），`aegis-helm` 全部使用 camelCase。

## 方案

将 `aegis-selfhealing-cks` 的全部功能合并进 `aegis-helm`，以 `selfhealing.enable` 作为总开关，默认关闭，保持对未启用自愈功能用户的零影响。

### 新增顶层参数

`registry` 和 `cluster` 已存在于 `aegis-helm/values.yaml`，直接复用。新增两个环境标识参数（与 `registry`、`cluster` 并列）：

```yaml
region: ""
orgname: ""
```

### 新增 selfhealing 块

```yaml
selfhealing:
  enable: false                        # 总开关；false 时不渲染任何 selfhealing 资源

  image:
    repository: k8s/aegis-selfhealing  # selfhealing 二进制镜像（独立于 aegis 主镜像）
    tag: ""

  opsImage:
    repository: k8s/aegis              # ops Job 执行镜像；registry 复用顶层
    tag: ""

  level: 0                             # 自愈激进度，0 = 保守
  hostNetwork: false
  dnsPolicy: ClusterFirst

  # 各硬件类型开关，控制 aegis-priority ConfigMap 中写入哪些 condition 条目
  baseboard: true
  node: true
  system: true
  cpu: true
  memory: true
  disk: true
  network: true
  gpu: true
  ib: true
  roce: true
  gpfs: true

  pluginSelector:                      # 命名由 snake_case 统一改为 camelCase
    gpu: nvidia-device-plugin-ds
    roce: sriovdp
    rdma: rdma-shared-dp-ds

  ticket:
    enabled: true                      # false 时不渲染 aegis-selfhealing Secret
    type: ""
    endpoint: ""
    appid: ""
    token: ""
    ranstr: ""

  sre:
    default: ""
    hardware: ""
    nonhardware: ""
```

### 新增 templates

所有新文件均放在 `templates/selfhealing/` 子目录，外层以 `{{- if .Values.selfhealing.enable }}` 包裹。

| 新文件 | 来源 | 说明 |
|---|---|---|
| `templates/selfhealing/aegis-priority-config.yaml` | `aegis-selfhealing-cks/templates/aegis-priority-config.yaml` | 直接迁移 |
| `templates/selfhealing/selfhealing-node.yaml` | `aegis-selfhealing-cks/templates/selfhealing-node.yaml` | 迁移并去掉 `--registry`/`--repository` 两行 |
| `templates/selfhealing/aegis-secret.yaml` | `aegis-selfhealing-cks/templates/aegis-secret.yaml` | 增加 `selfhealing.ticket.enabled` 内层条件 |

### 改造已有 templates

`templates/workflow/selfcheck-node.yaml` 和 `templates/alerts/todo-check-node.yaml` 属于 precheck 流程（检测 `scitix.ai/nodecheck` 污点、去除污点并 cordon 节点），也是自愈能力的组成部分，同样加 `selfhealing.enable` 开关。

### 参数映射关系

下表列出 cks chart 的参数在合并后的对应位置：

| 原参数（cks） | 合并后（aegis-helm） | 变化 |
|---|---|---|
| `registry` | `registry` | 复用，无变化 |
| `cluster` | `cluster` | 复用，无变化 |
| `region` | `region` | 提升为顶层 |
| `orgname` | `orgname` | 提升为顶层 |
| `prometheus.endpoint/token` | `prometheus.endpoint/token` | 复用，无变化 |
| `selfhealing.image` | `selfhealing.image` | 直接迁移 |
| `selfhealing.opsimage` | `selfhealing.opsImage` | 迁移，改 camelCase |
| `selfhealing.level` | `selfhealing.level` | 直接迁移 |
| `selfhealing.hostNetwork` | `selfhealing.hostNetwork` | 直接迁移 |
| `selfhealing.dnsPolicy` | `selfhealing.dnsPolicy` | 直接迁移 |
| `selfhealing.baseboard` … `gpfs` | `selfhealing.baseboard` … `gpfs` | 直接迁移 |
| `selfhealing.plugin_selector` | `selfhealing.pluginSelector` | 迁移，改 camelCase |
| `ticket.*` | `selfhealing.ticket.*` | 下移到 selfhealing 命名空间 |
| `sre.*` | `selfhealing.sre.*` | 下移到 selfhealing 命名空间 |
| `--registry` flag（命令行） | **删除** | 死参数，模板从未引用 |
| `--repository` flag（命令行） | **删除** | 死参数，且原 values.yaml 中无对应字段 |

### SelfHealingConfig / ApiBridge 清理（联动）

Helm 合并的同时，Go 侧同步清理对应死字段：

- `selfhealing/config/SelfHealingConfig`：删除 `Registry`、`Repository` 字段
- `root.go`：删除 `--registry`、`--repository` PersistentFlags
- `internal/selfhealing/sop/ApiBridge`：删除 `Registry`、`Repository` 字段
- `basic/node_remedy.go` 等 4 个文件：删除 parameters map 中的 `"registry"` 和 `"repository"` 条目

## 改动范围

| 文件 | 改动 |
|---|---|
| `deploy/aegis-helm/values.yaml` | 新增 `region`、`orgname`、`selfhealing.*` 块 |
| `deploy/aegis-helm/templates/selfhealing/aegis-priority-config.yaml` | 新增（迁移自 cks） |
| `deploy/aegis-helm/templates/selfhealing/selfhealing-node.yaml` | 新增（迁移自 cks，去掉死参数） |
| `deploy/aegis-helm/templates/selfhealing/aegis-secret.yaml` | 新增（迁移自 cks） |
| `deploy/aegis-helm/templates/workflow/selfcheck-node.yaml` | 加 `selfhealing.enable` 开关 |
| `deploy/aegis-helm/templates/alerts/todo-check-node.yaml` | 加 `selfhealing.enable` 开关 |
| `deploy/aegis-selfhealing-cks/` | 整个目录废弃删除 |
| `selfhealing/config/config.go` | 删除 `Registry`、`Repository` 字段 |
| `selfhealing/root.go` | 删除 `--registry`、`--repository` flag |
| `internal/selfhealing/sop/registry.go` | 删除 `ApiBridge.Registry`、`ApiBridge.Repository` 字段 |
| `internal/selfhealing/sop/basic/node_remedy.go` 等 4 个文件 | 删除 parameters 中 `registry`/`repository` 条目 |
