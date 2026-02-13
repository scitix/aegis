# 启动参数优化方案

## 现状问题

1. **CLI flags 过多**：当前 25+ 个 flag，全部在 Deployment args 里逐行列出。
2. **18 份 Deployment manifest 几乎雷同**：每个集群只有 4-5 个字段不同（`system-parameters`、`prometheus.endpoint/token`、`collector-image`、`route-prefix`），其余 15+ 行完全重复。
3. **viper 形同虚设**：`--config` flag 和 viper 已经存在，但 `initConfig()` 读进来之后没有绑定任何 flag，config.yaml 只存了 AI provider 配置。
4. **`--alert.system-parameters=region:gx;cluster:gemini`**：自定义 kv 格式，在 YAML 里更自然。

## 方案：flag → viper binding

viper 支持 `viper.BindPFlag(key, flag)`，绑定后优先级为：**CLI arg > 环境变量 > config file > 默认值**。不破坏任何现有行为。

### 第一步：在 `initConfig()` 里绑定所有 flag

```go
func initConfig(flags *pflag.FlagSet) {
    // ... 现有逻辑不变 ...
    viper.BindPFlag("alert.publish-namespace", flags.Lookup("alert.publish-namespace"))
    viper.BindPFlag("alert.ttl-after-succeed",  flags.Lookup("alert.ttl-after-succeed"))
    // ... 其他 flag 同理
}
```

### 第二步：把固定值挪进 config.yaml（各集群通用部分）

```yaml
alert:
  enable: true
  publish-namespace: monitoring
  ttl-after-succeed: 86400
  ttl-after-failed:  259200
  ttl-after-noops:   86400

healthcheck:
  enable: true

diagnosis:
  enable: true
  explain: true
  cache: true
  language: chinese

device-aware:
  enable: true
```

### 第三步：Deployment args 只保留集群差异项

```yaml
args:
  - --config=/aegis/config/config.yaml
  - --alert.system-parameters=cluster:gemini   # 集群专属，或移入 config.yaml
  - --prometheus.endpoint=http://prometheus-k8s.monitoring:9090
  - --prometheus.token=xxx
  - --diagnosis.collector-image=harbor.xxx/aegis-collector:v1.0.1
  - --web.route-prefix=/hercules/aegis
  - --http-port=8080
  - --enable-leader-election=true
  - --v=4
```

18 份 manifest 的 args 从 ~20 行缩减到 ~9 行，且每份只有实际差异。

## `system-parameters` 改格式

当前：`--alert.system-parameters=region:gx;cluster:gemini`（自定义解析）

改为 config.yaml：
```yaml
alert:
  system-parameters:
    region: gx
    cluster: gemini
```

去掉 `parse()` 里手写的 split 逻辑，直接用 `viper.GetStringMapString("alert.system-parameters")`。
CLI flag 保留作为覆盖入口（逗号分隔 `key=value` 格式），config file 优先读 map。

## 改动范围

| 文件 | 改动 |
|---|---|
| `cmd/aegis/main.go` | `initConfig()` 接收 flagset，加 `viper.BindPFlag`；system-parameters 支持 viper map |
| `deploy/online/framework/*.yaml` | 18 份 manifest 的 args 精简至集群差异项 |
| 各集群 ConfigMap 的 `config.yaml` | 补充通用配置项 |
