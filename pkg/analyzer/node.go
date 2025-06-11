package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	kcommon "github.com/k8sgpt-ai/k8sgpt/pkg/common"
	"github.com/k8sgpt-ai/k8sgpt/pkg/util"
	"github.com/scitix/aegis/pkg/analyzer/common"
	"github.com/scitix/aegis/pkg/prom"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	elastic "github.com/olivere/elastic/v7"
)

type NodeAnalyzer struct {
	prometheus *prom.PromAPI
}

func NewNodeAnalyzer() NodeAnalyzer {
	return NodeAnalyzer{
		prometheus: prom.GetPromAPI(),
	}
}

func (n NodeAnalyzer) Analyze(a common.Analyzer) (*common.Result, error) {
	kind := "Node"

	analyzer.AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	// selector, err := labels.Parse(a.LabelSelector)
	// if err != nil {
	// 	return nil, err
	// }

	// hostname, found := selector.RequiresExactMatch("kubernetes.io/hostname")
	// if !found {
	// 	return nil, fmt.Errorf("label selector must have hostname")
	// }
	hostname := a.Name

	// check node exists
	node, err := a.Client.GetClient().CoreV1().Nodes().Get(a.Context, hostname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var failures []kcommon.Failure
	// node condition
	nConditions, err := n.prometheus.GetNodeStatuses(a.Context, node.Name, "")
	if err != nil {
		return nil, fmt.Errorf("error get node conditions from prometheus: %s", err)
	}
	for _, condition := range nConditions {
		failures = append(failures, nodeStatusFailure(node.Name, condition))
	}

	var warnings []common.Warning
	// event
	events, err := n.prometheus.GetEvent(a.Context, "Node", "", node.Name, "Warning")
	if err != nil {
		klog.Warningf("error get node events from prometheus: %s", err)
	} else {
		for _, event := range events {
			warnings = append(warnings, nodeEventWarning(node.Name, event))
		}
	}

	var infos []common.Info

	if len(failures) > 0 {
		analyzer.AnalyzerErrorsMetric.WithLabelValues(kind, node.Name, "").Set(float64(len(failures)))
	}

	return &common.Result{
		Result: kcommon.Result{
			Kind:  kind,
			Name:  node.Name,
			Error: failures,
		},
		Warning: warnings,
		Info:    infos,
	}, nil
}

func fetchLogs(res *elastic.SearchResult) ([]string, error) {
	results := make([]string, 0)
	for _, hit := range res.Hits.Hits {
		type Log struct {
			Log string `json:"log"`
		}

		var log Log
		err := json.Unmarshal(hit.Source, &log)
		if err != nil {
			return nil, err
		}
		results = append(results, log.Log)
	}

	return results, nil
}

func nodeStatusFailure(nodeName string, status prom.AegisNodeStatus) kcommon.Failure {
	return kcommon.Failure{
		Text: fmt.Sprintf("condition %s type=%s id=%s value=%d", status.Condition, status.Type, status.ID, status.Value),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func nodeEventWarning(nodeName string, event prom.Event) common.Warning {
	return common.Warning{
		Text: fmt.Sprintf("has %s event at %s %s(%s) count %d", event.Type, event.TimeStamps, event.Reason, event.Message, event.Count),
		Sensitive: []kcommon.Sensitive{
			{
				Unmasked: nodeName,
				Masked:   util.MaskString(nodeName),
			},
		},
	}
}

func (NodeAnalyzer) Prompt(result *common.Result) (prompt string) {
	if result == nil || (len(result.Error) == 0 && len(result.Warning) == 0) {
		return
	}

	prompt = `你是一个很有帮助的 Kubernetes 集群故障诊断专家，下面是一篇节点故障诊断辅助文档，文档由 #### 分隔。接下来你需要根据我给出的现象（如果没有有效信息，请直接返回正常）帮忙诊断问题，一定需要使用中文语言来回答.
  ####
  ## 1. 标题

- 指标名称：scitix_node_status_condition
- 类型：Gauge（数值类型）
- 数据来源：PrometheusRule 再加工

## 2. 指标简介

- 简要描述：scitix_node_status_condition 是通过 PrometheusRule  二次加工各种原始 Kubernetes 集群事件、Prometheus IPMI Exporter 指标、Prometheus Node Exporter 指标、Prometheus GPFS Exporter 指标、DCGM Exporter 指标、Kubelet 指标等得到的 Prometheus 指标，用于 Kubernetes 集群节点异常诊断。它是一个数值类型，值代表出现异常及异常的具体数值。

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
- NodeFrequentDown：type="default" 类型，数值，高危异常。表示节点在 2h 内频繁出现掉线，如果伴随着 NodeHasRestart 异常说明是机器重启导致的；否则，一般是节点出现网络方面故障导致断连，需要排查机器网络问题。指标数值表现掉线次数。
- KubeletFailedCreatePodContainer: type="default" 类型，数值，高危异常。表示节点 kubelet 运行时创建 Pod 出现未预期异常，通常重启 kubelet 可以解决。指标数据表示该节点创建失败的 Pod 数量。
- BaseBoardCriticalIssue：type="baseboard" 类型，数值，高危异常。表示服务器基础硬件存在故障，需要维修，id 标签给出具体具体的硬件类型，全量包括：

  - fan：服务器风扇失效或转速严重偏离正常范围，可能导致服务器温度升高，存在硬件损坏风险，建议立即采取措施（如更换风扇或降低服务器负载）。指标数值给出了机器出问题的风扇数量。
  - temperature：服务器内部硬件温度过高，可能导致硬件损坏。指标数值给出了机器温度异常的部件数量，建议立即采取措施进行冷却或关机保护。
  - voltage：服务器内部电压超出安全范围，可能导致硬件损坏或系统不稳定，建议立即采取行动（如关闭电源或调整电源设置）。
  - power：服务器供电单元异常。指标数值给出了异常的部件数量。
  - current：服务器内部电流超出安全范围，可能会导致硬件损坏或触发电源保护机制，建议立即采取措施，如降低负载或关机保护。
  - pcie：服务器 PCIe 插槽或设备出现故障，如连接问题、硬件故障或通信中断。可能需要检查设备连接情况或更换硬件。指标数值给出了异常的部件数量。
  - sysHealth：服务器整体不健康，存在部件故障。需要系统管理员实时了解服务器的运行状态，快速检测潜在问题。
- CPUPressure：type="cpu" 类型，数值，低危异常。表示节点整体 CPU 使用率超过 90%，指标数值记录了当前节点整体 CPU 利用率。
- CpuUnhealthy：type="cpu" 类型，数值，高危异常。表示服务器 CPU 硬件出现故障，需要维修，id 标签给故障的 CPU ID。
- MemoryPressure：type="memory" 类型，数值，低危异常。表示节点整体内存使用率超过 90%，指标数值记录了当前节点整体内存利用率。
- KubeletMemoryPressure：type="memory" 类型，数值，高危异常（只影响负载，不影响机器）。表示节点 所有 Pod 使用内存超过 90%，指标数值记录了节点所有 Pod 使用内存比例，超过 95% 将引发节点 Pod 驱逐，需要立即定位并清理危险 Pod。
- MemoryUnhealthy：type="memory" 类型，数值，高危异常。表示服务器内存条出现故障，需要更换，id 标签给出故障的内存条插口 ID。
- DiskPressure：type="disk" 类型，数值，高危异常（只影响负载，不影响机器）。表示节点根文件系统磁盘使用率超过 84%，指标数值记录了节点根文件系统磁盘使用率，超过 85% 将引发节点 Pod 驱逐。
- DiskUnhealthy：type="disk" 类型，数值，高危异常。表示服务器内存条出现故障，需要更换，id 标签给出故障的磁盘插口 ID。
- HighDProcessesCount：type="system" 类型，数值，低危异常。表示节点系统上存在超过 4 个 D 进程，指标数值记录了当前节点系统上 D 进程总数。建议排查 D 进程出现的原因。
- HighZombieProcessesCount：type="system" 类型，数值，低危异常。表示节点系统上存在超过 50000 个 Z 僵尸进程，指标数值记录了当前节点系统上 Z 进程总数。建议排查 Z 进程出现的原因，在合适的时间重启节点以清理 Z 进程。
- PeerMemModuleNotConfig：type="system" 类型，二值，低危异常。表示节点未开启 nvidia-peermem 模块，可能导致多 GPU 卡训练任务性能低或失败，建议开启。
- NetworkLinkDown：type="network" 类型，二值，高危异常。表示机器存在以太网卡设备掉线，建议维修或更换设备，id 标签给出掉线的网卡号。
- NetworkLinkFrequentDown：type="network" 类型，数值，高危异常。表示机器以太网卡设备 2h 内掉线超过 5 次，指标数值记录了该网卡设备过去 2h 掉线总数，id 标签给出掉线的网卡号。建议进行诊断、维修。
- NetworkLinkTooManyDown：type="network" 类型，数值，高危异常。表示机器以太网卡设备掉线总数超过 100 次，指标数值记录了该网卡设备掉线总数，id 标签给出掉线的网卡号。建议进行诊断、维修，并重启机器以重置计数。
- IBDown：type="ib" 类型，二值，高危异常。表示机器存在 Infiniband 设备掉线，建议维修或更换设备，id 标签给出掉线的 Infiniband 设备号。
- IBBroken：type="ib" 类型，数值，高危异常。表示在机器上执行的 ibping 巡检失败，指标数值给出了巡检失败次数。建议排查 Infiniband 设备或 IB 网络链路问题。
- IBLinkFrequentDown：type="ib" 类型，数值，高危异常。表示机器 Infiniband 设备卡 2h 内掉线超过 5 次，指标数值记录了该设备卡过去 2h 掉线总数，id 标签给出掉线的 Infiniband 设备卡号。建议进行诊断、维修。
- IBPcieDowngraded：type="ib" 类型，二值，高危异常。表示机器 Infiniband 链路出现 PCIe 降级故障，id 标签给出异常的 Infiniband 设备卡号，pci_bdf 标签给出异常的 PCIe 设备号，msg 字段给出详细说明。建议进行设备维修，否则会影响 IB 网络性能。
- IBModuleNotInstalled：type="ib" 类型，二值，高危异常。表示机器 Infiniband 设备驱动有模块未安装，Infiniband 设备将不能正常工作，建议重装正确版本的驱动。
- IBRegisterFailed：type="ib" 类型，二值，高危异常。表示 Infiniband 设备未成功注册到节点上，会导致节点无法分配 RDMA 资源，通常是由于 rdma 驱动 Pod 异常导致，需要重启 rdma 驱动 Pod 或进一步维修 Infiniband 设备。
- RoceRegisterFailed：type="ib" 类型，数值，高危异常。表示 Roce 以太网某设备未成功注册到节点上，指标数值给出了当前少注册的数量，会导致节点无法分配网卡资源，通常是由于驱动层面的异常，id 标签给出异常的网卡号。
- RoceDeviceBroken: type="ib" 类型，数值，高危异常。表示 Roce 以太网某设备初始化异常。
- IBSymbolError：type="ib" 类型，数值，低危异常。指的是 Infiniband 网络中在数据链路层发生的一种通信错误，通常表示链路上接收到的符号（Symbol）与预期不符，导致传输数据无法正确解析。这些错误可能由物理链路上的问题（如干扰、连接不良）或硬件故障导致。指标数值记录了当前总出错次数，id 标签给出了出问题的 Infiniband 设备号。
- IBReceivedError：type="ib" 类型，数值，低危异常。表示机器 Infiniband 设备端口最近 24h 收到超过 1000 次错误，指标数值记录了当前总出错次数，id 标签给出了出问题的 Infiniband 设备号。建议进行诊断。
- IBTransmitError：type="ib" 类型，数值，低危异常。表示机器 Infiniband 设备端口最近 24h 传输发生超过 1000 次错误，指标数值记录了当前总出错次数，id 标签给出了出问题的 Infiniband 设备号。建议进行诊断。
- GpuApplicationFrequentError：type="gpu"类型，数值，低危异常。表示 GPU 卡上最近 1h 出现超过 6 次应用类错误，指标数据记录了当前出错次数，id 标签给出发生异常的 GPU 卡号。该异常通常是用户应用出错导致的，但如果应用本身正常就需要进一步诊断 GPU 硬件。
- GpuCheckFailed：type="gpu" 类型，数值，高危异常。表示 GPU 卡巡检失败，指标数据记录了巡检失败的次数，id 标签给出 GPU 卡号异常编码（如 10010000 表示 8 张 GPU 卡，其中第 0 号与第 3 号卡异常）。
- GpuHung：type="gpu" 类型，数值，高危异常。表示 GPU 卡巡检 hung 死，如果只是偶发，指标数值偏低（<3），则可能是系统压力过大导致巡检超时。
- GpuDown：type="gpu" 类型，数值，高危异常。表示节点 GPU 掉卡，指标数值记录了当前节点可用卡数（正常情况下机器卡数是 8）。通常是 GPU PCIe 插槽或连接问题，或是本身硬件故障。GpuDown 异常状态通常会派生出 GpuCheckFailed 异常，需要忽略后者。
- XIDECCMemoryErr：type="gpu" 类型，数值，高危异常。表示节点 GPU 出现 ECC 错误，指标数据记录了 GPU XID 错误码，id 标签给出 GPU 卡号。已失败的 GPU 任务需要重新发起，出现异常的 GPU 需要适时重置。
- XIDHWSystemErr：type="gpu" 类型，数值，高危异常。表示节点 GPU 出现硬件故障，需要进行维修。指标数据记录了 GPU XID 错误码，id 标签给出 GPU 卡号。
- XIDApplicationErr：type="gpu" 类型，数值，无异常。表示节点 GPU 上应用本身出现异常，如地址访问越界。指标数据记录了 GPU XID 错误码，id 标签给出 GPU 卡号。建议应用开发者自查问题。
- XIDUnclassifiedErr：type="gpu" 类型，数值，高危异常。表示节点 GPU 上出现非 XIDECCMemoryErr、XIDHWSystemErr、XIDApplicationErr 错误，指标数据记录了 GPU XID 错误码，id 标签给出 GPU 卡号。建议进一步诊断。
- GpuTooManyPageRetired：type="gpu" 类型，二值，高危异常。表示节点 GPU 显存页退休机制出现错误，需要返厂维修，id 标签给出 GPU 卡号。
- GpuPcieDowngraded：type="gpu" 类型，二值，高危异常。表示机器 GPU PCIe 链路出现降级故障，id 标签给出异常的 GPU 卡号，pci_bdf 标签给出异常的 PCIe 设备号，msg 字段给出详细说明。建议进行设备维修，否则会影响 GPU 训练速度。
- GpuRegisterFailed：type="gpu" 类型，二值，高危异常。表示 GPU 设备未成功注册到节点上，会导致节点无法分配 GPU 资源，通常是由于 nvidia gpu 驱动 Pod 异常导致，需要重启 nvidia gpu 驱动 Pod 或进一步维修 GPU 设备。
- GpuRowRemappingFailure：type="gpu" 类型，二值，高危异常。表示节点 GPU 显存重映射机制出现错误，需要返厂维修，id 标签给出 GPU 卡号。
- GpuSramUncorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU SRAM 累计出现超过 4 个 Uncorrectable ECC 错误，需要返厂维修，id 标签给出 GPU 卡号。
- GpuAggSramUncorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU SRAM 累计出现超过 4 个 Uncorrectable ECC 错误，需要返厂维修，id 标签给出 GPU 卡号。
- GpuVolSramUncorrectable: type="gpu" 类型，二值，高危异常。表示节点 GPU SRAM 临时性出现 Uncorrectable ECC 错误，需要人工查看，id 标签给出 GPU 卡号。
- GpuVolDramUncorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU DRAM 临时性出现超过 60 次 Uncorrectable ECC 错误，需要重置 GPU，id 标签给出 GPU 卡号。
- GpuVolDramCorrectable：type="gpu" 类型，二值，高危异常。表示节点 GPU DRAM 临时性出现超过 1000 次 Correctable ECC 错误，需要重置 GPU，id 标签给出 GPU 卡号。
- HighGpuTemp：type="gpu" 类型，二值，低危异常。表示节点 GPU 温度超过安全阈值，可能原因是散热或者硬件问题，id 标签给出 GPU 卡号。
- HighGpuMemoryTemp：type="gpu" 类型，二值，低危异常。表示节点 GPU 显存温度超过安全阈值，可能原因是散热或者硬件问题，id 标签给出 GPU 卡号。
- GPUNvlinkInactive：type="gpu" 类型，二值，高危异常。表示节点 GPU NVLINK 状态未激活，id 给出了卡号。
- GPUHWSlowdown：type="gpu" 类型，二值，高危异常。表示节点 GPU 出现硬件降频，可能是因为温度过高或供电不足等硬件问题，id 给出了卡号。
- GPUPersistenceModeNotEnabled: type="gpu" 类型，二值，高危异常。表示节点 GPU 未开启持久性模式，会影响 GPU 性能，id 给出了卡号。
- GpfsDown：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 进程未启动，会导致无法访问 GPFS 存储，需要重新拉起进程。GpfsDown 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsMountLost：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 文件存储未挂载，会导致无法访问 GPFS 存储，需要重新挂载。GpfsMounLost 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsTheadDeadlock：type="gpfs" 类型，数值，低危异常。表示节点 GPFS 进程已等待超过 900 秒，一般是因为 mmap 操作导致，或者是网络状态有异常，如果是大范围出现则表示集群 GPFS 文件存储存在死锁问题。
- GpfsTestFailed：type="gpfs" 类型，二值，高危异常。表示 GPFS 文件系统读写巡检失败，指标数据记录了巡检失败的次数，id 标签给出 GPFS 文件系统名。通常是 GPFS 文件存储故障或者是该机器自身网络异常，需要进一步维修；如果只是偶发，指标数值偏低（<10），则可能是系统压力过大导致巡检超时，或者是节点刚刚重启（NodeHasRestart）但是 GPFS 进程尚未拉起。GpfsDown、GpfsMountLoast 派生 GpfsTestFailed，需要忽略后者。
- GpfsIBNotConfig：type="gpfs" 类型，二值，高危异常。表示节点 GPFS 进程未正确识别到 Infiniband 设备，会导致文件访问性能降低。可能是因为 Infiniband 设备故障，需要进行维修；或者是因为 GPFS 进程启动时 Infiniband 设备尚未准备就绪，需要重启 GPFS 进程。

## 6. 节点故障

异常状态（condition 标签值）可被划分为低危、高危两类。当节点存在至少一个高危异常时，表示节点故障；当节点不存在高危异常，只存在低危异常时，节点未遇到任何故障。

高危异常之前存在派生关系，故障诊断时应当忽略派生异常。常见的派生异常如下：

- GpuDown 异常状态通常会派生出 GpuCheckFailed 异常，需要忽略后者。
- GpfsDown 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
- GpfsMounLost 异常状态通常会派生 GpfsTestFailed，需要忽略后者。
  ####`

	if len(result.Error) > 0 {
		failureText := ""
		for _, e := range result.Error {
			failureText = failureText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n异常指标信息：%s", failureText)
	}

	if len(result.Warning) > 0 {
		warningText := ""
		for _, e := range result.Warning {
			warningText = warningText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n一些节点历史告警事件（如果认为有帮助，可以使用，或者忽略）：%s", warningText)
	}

	if len(result.Info) > 0 {
		infoText := ""
		for _, e := range result.Info {
			infoText = infoText + e.Text + "\n"
		}
		prompt += fmt.Sprintf("\n以下日志只是辅助信息，可以参考日志对异常指标进行解释：%s", infoText)
	}

	prompt += `
请按以下格式给出回答，不超过 512 字:
Healthy: {Yes 或者 No，代表是否有异常}
Error: {在这里解释错误}
Analysis: {在这里给出分析过程}
Solution: {在这里给出分步骤的解决方案}`

	return
}
