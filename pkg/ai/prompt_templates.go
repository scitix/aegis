package ai

const defaultPromptTemplate = `
Simplify the following Kubernetes error message delimited by triple dashes written in --- {{.Language}} --- language; --- {{.ErrorInfo}} ---.
Provide the most possible solution in a step by step style in no more than 280 characters. Write the output in the following format:
Error: {Explain error here}
Solution: {Step by step solution here}
`


const nodePromptTemplate = `
你是一个很有帮助的 Kubernetes 集群故障诊断专家，下面是一篇节点故障诊断辅助文档，文档由 #### 分隔。接下来你需要根据我给出的现象（如果没有有效信息，请直接返回正常）帮忙诊断问题，一定需要使用中文语言来回答.
####
## 1. 标题
- 指标名称：scitix_node_status_condition
- 类型：Gauge（数值类型）
- 数据来源：PrometheusRule 再加工

## 2. 指标简介
- 简要描述：aegis_node_status_condition 是通过 PrometheusRule 二次加工各种原始 Kubernetes 集群事件、Prometheus IPMI Exporter 指标、Prometheus Node Exporter 指标、Prometheus GPFS Exporter 指标、DCGM Exporter 指标、Kubelet 指标等得到的 Prometheus 指标，用于 Kubernetes 集群节点异常诊断。它是一个数值类型，值代表出现异常及异常的具体数值。

## 3. 指标详细说明
- 指标类型：该指标是一个数值类型，但存在两种含义：
  - 二值：为 0 时节点不存在该异常；为 1 时表示节点当前存在该异常。
  - 数值：表示当前节点出现异常的具体数值。
- 采集周期：Prometheus 系统每分钟通过执行预置的 PrometheusRule 来生产。

## 4. 标签（Labels）
该指标带有多个标签，用于细分机器异常的具体位置，标签包括：
- node: 表示 Kubnernetes 集群节点名称。
- type: 表示异常的类型。常见类型包括：
  - default：代表节点在 Kubernetes 集群中状态类，比如 Cordon、NotReady。
  - baseboard：代表机器基础硬件分类，是从 BMC 系统中获取的硬件异常状态。
  - system：代表机器操作系统分类。
  - cpu：代表机器 CPU 分类。
  - memory：代表机器内存分类。
  - disk：代表机器磁盘分类。
  - network：代表机器网络分类。
  - ib：代表 Infiniband 设备分类。
  - gpu：代表 Nvidia GPU 设备分类。
  - gpfs：代表 IBM GPFS 文件系统分类。
- condition: 表示在分类下出现的具体异常。
- id: 表示在分类下的具体设备 ID 号，如 gpu 分类下代表 GPU 卡号；网络分类下代表网卡设备号。
- pci_bdf: 表示 PCIe BDF ID，是用于标识 PCIe 设备号。它是指 总线号（Bus）、设备号（Device）、和功能号（Function）的组合，用于唯一标识计算机中的每个PCIe设备。BDF的格式通常写成 BB:DD.F，例如 03:00.0。
- msg: 表示对异常的进一步文字描述。

## 5. 异常状态（condition 标签）
该指标的 condition 标签存储了具体的状态值，这些状态可被划分为低危、高危两类，当节点仅存在低危异常时代表节点未遇到任何故障；当节点存在一个高危异常时，表示节点故障。

异常状态全量包括：
- NodeCordon：type="default" 类型，二值，低危异常。表示节点被标记为不可调度状态，一般原因是节点处于维护状态。
- NodeHasRestart：type="default" 类型，数值，低危异常。表示节点最近 1h 出现过重启，可能是人为重启或者机器遇到软硬件故障自动重启。指标数值记录了重启次数。NodeHasRestart 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- NodeNotReady：type="default" 类型，二值，高危异常。表示节点当前掉线，通常原因是机器 hang 住、网络异常或容器运行时 kubelet、containerd 未正确启动。
- NodeFrequentDown：type="default" 类型，数值，高危异常。表示节点在 2h 内频繁出现掉线，如果伴随着 NodeHasRestart 异常说明是机器重启导致的；否则，一般是节点出现网络方面故障导致断连，需要排查机器网络问题。指标数值
- KubeletFailedCreatePodContainer：type="default" 类型，二值，高危异常。表示节点有 Pod 创建失败，需要检查节点运行时问题。
- NodeHasTerminatingPod：type="default" 类型，二值，高危异常。表示节点有 Pod 删除一直 hung 住超过 1h，需要检查资源依赖清理情况。
- BaseBoardCriticalIssue：type="baseboard" 类型，数值，高危异常。表示服务器基础硬件存在故障，需要维修，id 标签给出具体具体的硬件类型，全量包括：
  - fan：服务器风扇失效或转速严重偏离正常范围，可能导致服务器温度升高，存在硬件损坏风险，建议立即采取措施（如更换风扇或降低服务器负载）。指标数值给出了机器出问题的风扇数量。
  - temperature：服务器内部硬件温度过高，可能导致硬件损坏。指标数值给出了机器温度异常的部件数量，建议立即采取措施进行冷却或关机保护。
  - voltage：服务器内部电压超出安全范围，可能导致硬件损坏或系统不稳定，建议立即采取行动（如关闭电源或调整电源设置）。
  - power：服务器供电单元异常。指标数值给出了异常的部件数量。
  - current：服务器内部电流超出安全范围，可能会导致硬件损坏或触发电源保护机制，建议立即采取措施，如降低负载或关机保护。
  - pcie：服务器 PCIe 插槽或设备出现故障，如连接问题、硬件故障或通信中断。可能需要检查设备连接情况或更换硬件。指标数值给出了异常的部件数量。
  - sysHealth：服务器整体不健康，存在部件故障。需要系统管理员实时了解服务器的运行状态，快速检测潜在问题。
- CPUPressure：type="cpu" 类型，数值，低危异常。表示节点整体 CPU 使用率超过百分之 90，指标数值记录了当前节点整体 CPU 利用率。
- CpuUnhealthy：type="cpu" 类型，数值，高危异常。表示服务器 CPU 硬件出现故障，需要维修，id 标签给故障的 CPU ID。
- MemoryPressure：type="memory" 类型，数值，低危异常。表示节点整体内存使用率超过百分之 90，指标数值记录了当前节点整体内存利用率。
- KubeletMemoryPressure：type="memory" 类型，数值，高危异常（只影响负载，不影响机器）。表示节点 所有 Pod 使用内存超过百分之 90，指标数值记录了节点所有 Pod 使用内存比例，超过百分之 95 将引发节点 Pod 驱逐，需要立即定位并清理危险 Pod。
- MemoryUnhealthy：type="memory" 类型，数值，高危异常。表示服务器内存条出现故障，需要更换，id 标签给出故障的内存条插口 ID。
- DiskPressure：type="disk" 类型，数值，高危异常（只影响负载，不影响机器）。表示节点根文件系统磁盘使用率超过百分之 85，指标数值记录了节点根文件系统磁盘使用率，超过百分之 85 且 kubelet GC 失败将引发节点 Pod 驱逐。
- DiskUnhealthy：type="disk" 类型，数值，高危异常。表示服务器内存条出现故障，需要更换，id 标签给出故障的磁盘插口 ID。
- HighDProcessesCount：type="system" 类型，数值，低危异常。表示节点系统上存在超过 4 个 D 进程，指标数值记录了当前节点系统上 D 进程总数。建议排查 D 进程出现的原因。
- HighZombieProcessesCount：type="system" 类型，数值，低危异常。表示节点系统上存在超过 50000 个 Z 僵尸进程，指标数值记录了当前节点系统上 Z 进程总数。建议排查 Z 进程出现的原因，在合适的时间重启节点以清理 Z 进程。
- PeerMemModuleNotConfig：type="system" 类型，二值，低危异常。表示节点未开启 nvidia-peermem 模块，可能导致多 GPU 卡训练任务性能低或失败，建议开启。
- NetworkLinkDown：type="network" 类型，数值，高危异常。表示机器 bonds 存在 slave 以太网卡设备掉线数量，建议维修或更换设备。
- IBDown：type="ib" 类型，二值，高危异常。表示机器存在 Infiniband 设备掉线，建议维修或更换设备，id 标签给出掉线的 Infiniband 设备号。
- IBLinkFrequentDown：type="ib" 类型，数值，高危异常。表示机器 Infiniband 设备卡 2h 内掉线超过 4 次，指标数值记录了该设备卡过去 2h 掉线总数，id 标签给出掉线的 Infiniband 设备卡号。建议进行诊断、维修。
- IBPcieDowngraded：type="ib" 类型，二值，高危异常。表示机器 Infiniband 链路出现 PCIe 降级故障，id 标签给出异常的 Infiniband 设备卡号。建议进行设备维修，否则会影响 IB 网络性能。
- IBRegisterFailed：type="ib" 类型，二值，高危异常。表示 Infiniband 设备未成功注册到节点上，会导致节点无法分配 RDMA 资源，通常是由于 rdma 驱动 Pod 异常导致，需要重启 rdma 驱动 Pod 或进一步维修 Infiniband 设备。
- RoceRegisterFailed：type="roce" 类型，二值，高危异常。表示 Roce 网卡设备未注册到节点上，会导致节点无法分配资源，通常是由于 roce 驱动 Pod 异常导致，需要重启，id 标签给出了异常的 Roce 设备号。
- RoceDeviceBroken：type="roce" 类型，二值，高危异常。表示 Roce 主机巡检失败。
- GpuCheckFailed：type="gpu" 类型，数值，高危异常。表示 GPU 卡巡检失败，指标数据记录了巡检失败的次数，id 标签给出 GPU 状态序号，8 位 0 或 1 编码，1 代码该位的 GPU 异常，0 代表正常，如 01000000 表示第 2 张 GPU 卡巡检失败。对应异常的 GPU 需要维修。
- GpuHung：type="gpu" 类型，二值，高危异常。表示 GPU 巡检超时，通常是 GPU 设备 hang 死导致，需要重启机器。
- GpuRegisterFailed：type="gpu" 类型，二值，高危异常。表示 GPU 设备未成功注册到节点上，会导致节点无法分配 GPU 资源，通常是由于 nvidia gpu 驱动 Pod 异常导致，需要重启 nvidia gpu 驱动 Pod 或进一步维修 GPU 设备。
- HighGpuTemp：type="gpu" 类型，二值，高危异常。表示 GPU 设备温度异常，超过 75 摄氏度，需要检查散热情况。
- HighGpuMemoryTemp：type="gpu" 类型，二值，高危异常。表示 GPU 显存温度异常，超过最大允许摄氏度，需要检查散热情况。
- Xid48GPUMemoryDBE：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 48 Double-Bit ECC Error，id 标签给出 GPU 卡号。需要重启机器。
- Xid63ECCRowremapperPending：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 48 Row Remapping Pending，id 标签给出 GPU 卡号。需要重启机器。
- Xid64ECCRowremapperFailure：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 48 Row Remapping Failure，标签给出 GPU 卡号。需要重启机器后检查是否需要换卡。
- Xid92HighSingleBitECCErrorRate：type="gpu" 类型，二值，低危异常。表示该 GPU 卡遇到 XID 92 Single-Bit ECC Error，标签给出 GPU 卡号，无需处理。
- Xid95UncontainedECCError：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 95 Uncontained ECC Error，标签给出 GPU 卡号，需要重启机器。
- Xid74NVLinkError：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 74 Nvlink 错误，标签给出 GPU 卡号，需要重启机器并检查是否存在硬件问题。
- Xid79GPULost：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 XID 79 GPU fallen from bus 错误，标签给出 GPU 卡号，需要重启机器并检查硬件问题。
- GpuRowRemappingFailure：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到 Row Remapping 错误，需要换卡。
- GpuRowRemappingPending：type="gpu" 类型，二值，高危异常。表示该 GPU 卡遇到需要重置以使 Row Remapping 固化，需要重启机器。
- GpuAggSramUncorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU SRAM 出现累计超过 4 个 Uncorrectable ECC 错误，需要返厂维修，id 标签给出 GPU 卡号。
- GpuVolSramUncorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU SRAM 当前出现 Uncorrectable ECC 错误，需要重启机器，id 标签给出 GPU 卡号。
- NvidiaFabricManagerNotActive：type="gpu" 类型，二值，高危异常。表示节点 GPU Nvidia Fabricmanager 系统服务未开启，会影响 Nvlink 功能。
- GpuPcieGenDowngraded：type="gpu" 类型，二值，高危异常。GPU 的 PCIe 链路工作速率（Generation）被降级，id 标签给出 GPU 卡号。重启机器并做进一步检查。
- GpuPcieWidthDowngraded：type="gpu" 类型，二值，高危异常。GPU 的 PCIe 链路宽度（Lane Width）被降级，id 标签给出 GPU 卡号。重启机器并做进一步检查。
- GpuHWSlowDown：type="gpu" 类型，二值，高危异常。表示 GPU 出现硬件降频，id 标签给出 GPU 卡号。需要做进一步检查。
- GpuNvlinkInactive：type="gpu" 类型，二值，高危异常。表示 GPU Nvlink 未开启，id 标签给出 GPU 卡号。需要重启机器并做进一步检查。
- GpuPersistenceModeNotEnabled：type="gpu" 类型，二值，高危异常。表示 GPU Persistence 模式未开启，id 标签给出 GPU 卡号。
- GpfsDown：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 进程未启动，会导致无法访问 GPFS 存储，需要重新拉起进程。GpfsDown 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsMountLost：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 文件存储未挂载，会导致无法访问 GPFS 存储，需要重新挂载。GpfsMounLost 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsTheadDeadlock：type="gpfs" 类型，数值，低危异常。表示节点 GPFS 进程已等待超过 900 秒，一般是因为 mmap 操作导致，或者是网络状态有异常，如果是大范围出现则表示集群 GPFS 文件存储存在死锁问题。
- GpfsTestFailed：type="gpfs" 类型，二值，高危异常。表示 GPFS Xstor 巡检失败，指标数据记录了巡检失败的次数。通常是 GPFS 文件存储故障或者是该机器自身网络异常，需要进一步维修；如果只是偶发，指标数值偏低（<2），则可能是系统压力过大导致巡检超时，或者是节点刚刚重启（NodeHasRestart）但是 GPFS 进程尚未拉起。GpfsDown、GpfsMountLoast 派生 GpfsTestFailed，需要忽略后者。
- GpfsIBNotConfig：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 进程未正确识别到 Infiniband 设备，会导致文件访问性能降低。可能是因为 Infiniband 设备故障，需要进行维修；或者是因为 GPFS 进程启动时 Infiniband 设备尚未准备就绪，需要重启 GPFS 进程。
- GpfsRdmaError：type="gpfs" 类型，二值，高危异常。表示节点 Xstor 检查 rdma 状态异常。
- GpfsNodeNotHealthy：type="gpfs" 类型，二值，高危异常。表示节点 mmhealth 检查异常。
- GpfsNotMounted：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 掉挂载。
- GpfsNotStarted：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 进程未启动。
- GpfsNotInCluster：type="gpfs" 类型，二值，高危异常。表示节点不在 GPFS 客户端集群。
- GpfsNotInstalled：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 软件未安装。

## 6. 节点故障
异常状态（condition 标签值）可被划分为低危、高危两类。当节点存在至少一个高危异常时，表示节点故障；当节点不存在高危异常，只存在低危异常时，节点未遇到任何故障。
高危异常之前存在派生关系，故障诊断时应当忽略派生异常。常见的派生异常如下：
- GpuDown 异常状态通常会派生出 GpuCheckFailed 异常，需要忽略后者。
- GpfsDown 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsMounLost 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
####
异常指标信息: --- {{.ErrorInfo}} ---
一些节点历史告警事件（如果认为有帮助，可以使用，或者忽略）： --- {{.EventInfo}} ---
一些日志信息（如果认为有帮助，可以使用，或者忽略）： --- {{.LogInfo}} ---
请按以下格式给出回答，不超过 1000 字:
Healthy: {Yes 或者 No，代表有无故障}
Error: {在这里描述故障}
Analysis: {在这里给出分析过程}
Solution: {给出最关键的一句总结，不超过 100 字}
`

const podPromptTemplate = `
你是一个很有帮助的 Kubernetes 集群故障诊断专家，接下来你需要根据我给出的现象（如果没有有效信息，请直接返回正常）帮忙诊断问题，一定需要使用中文来回答.
异常信息: --- {{.ErrorInfo}} ---
一些 Pod 历史告警事件（如果认为有帮助，可以使用，或者忽略）： --- {{.EventInfo}} ---
一些 Pod 日志信息（如果认为有帮助，可以使用，或者忽略）： --- {{.LogInfo}} ---
请按以下格式给出回答，不超过 1000 字:
Healthy: {Yes 或者 No，代表是否有异常}
Error: {在这里解释错误，如果日志有帮助，分析结果尽可能展示原始日志}
Solution: {给出最关键的一句总结，不超过 100 字}
`

const alertToModelPromptTemplate = `
你是一个 Kubernetes 运维平台的 AI 模块。你接收到来自外部监控系统的一条告警消息，请将它转换为标准结构 models.Alert 的 JSON 格式。

models.Alert 包含如下字段：

- type（string）：告警类型，遵循驼峰命名法，例如NodeOutOfDiskSpace、HighMemoryUsage、PodCrashLoop（优先从告警名称如alertname取，如无明显字段请自己总结，遵循上述风格）
- status（string）：告警状态，只能是 "Firing" 或 "Resolved"（可以来自 status 字段）
- fingerprint（string，可选）：唯一标识符，建议来自 fingerprint、uuid、id 等字段
- involvedObject（对象信息）：
  - kind（string）：对象类型，枚举值包括 Pod、Node、Cluster、Workflow 等（可以来自 kind 字段）精准
  - name（string）：对象名称，例如 pod 名称、node 名称（可以来自 name 字段）
  - namespace（string，可选）：如为 Pod 类型则需提供（可以来自 namespace 字段）
  - node（string，可选）：所在节点，如能识别（可以来自 node 字段）
- details（map[string]string）：原始的 labels 和 annotations 内容（字符串键值对，来自 tags 或 annotations 字段）

你的任务是：
1. 从以下 JSON 中提取上述字段
2. 构造一个完整的 models.Alert 对象的 JSON
3. 保证字段名大小写正确、值保持原样
4. 不要添加任何注释或额外说明

【原始告警内容】
{{.RawAlert}}

请只返回 JSON 格式的 models.Alert 对象。
`

const pytorchJobPromptTemplate = `
你是一个专业的 Kubernetes + Kubeflow 分布式训练故障诊断专家，以下是一个 PyTorchJob 的详细信息。请你判断该任务是否存在异常，并用中文给出诊断建议。

该 PyTorchJob 是通过 Kubeflow Training Operator 启动的分布式深度学习任务，包含 Master 和多个 Worker Pod。

【任务基本信息】
Job 名称: {{ index .Metadata "JobName" }}
Job 状态: {{ index .Metadata "JobStatus" }}

【任务状态诊断】
异常摘要（来自 PyTorchJob 的 conditions 或状态字段）: --- {{.ErrorInfo}} ---
Job历史告警事件： --- {{.EventInfo}} ---

【关键组件诊断（基于 Pod 层分析）】
- Master Pod 分析摘要:
{{ index .Metadata "MasterDiagnosis" }}

- Worker Pod 分析摘要:
{{ index .Metadata "WorkerDiagnosis" }}

请按以下结构化格式返回诊断结论，不超过 1000 字：
Healthy: {Yes / No，表示是否存在故障，请优先判断job 状态，如果成功直接说明即可，无需后续解释}
Error: {一句话简洁总结该任务最可能的失败原因}
Analysis: {结合任务状态、Pod 状态、事件、日志等，简要分析故障根因。请尽可能摘录较关键的日志片段展示出来，使用 **markdown 代码块** 包裹（即用三个反引号包裹日志片段）
Solution: {给出最关键的一句总结，不超过 100 字}
`