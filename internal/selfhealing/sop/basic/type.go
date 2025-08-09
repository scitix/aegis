package basic

import (
	"context"
	"path/filepath"
	"time"
)

var job_namespace = "monitoring"
var job_dir string = "/selfhealing/job"
var reboot_job_file string = filepath.Join(job_dir, "restart_node.yaml")
var shutdown_job_file string = filepath.Join(job_dir, "shutdown_node.yaml")
var healthcheck_job_file string = filepath.Join(job_dir, "healthcheck_node.yaml")
var diagnose_job_file string = filepath.Join(job_dir, "diagnose_node.yaml")
var diagnose_gpfs_job_file string = filepath.Join(job_dir, "diagnose_gpfs.yaml")
var repair_job_file string = filepath.Join(job_dir, "repair_node.yaml")
var remedy_job_file string = filepath.Join(job_dir, "remedy_node.yaml")
var perf_job_file string = filepath.Join(job_dir, "perf_node.yaml")

var SleepWaitDuration = time.Hour * time.Duration(1)

const (
	SystemNamespace = "kube-system"

	GPU_RESOURCE_TYPE = "nvidia.com/gpu"

	NodeGpfsUnavailableLabelKey   = "aegis.io/gpfs-unavailable"
	NodeGpfsUnavailableLabelValue = "true"

	NodeIBUnavailableLabelKey   = "aegis.io/ib-unavailable"
	NodeIBUnavailableLabelValue = "true"

	NodeCordonReasonKey = "aegis.io/node-cordon-reason"
	NodeRebootCountKey  = "aegis.io/node-reboot"
	NodeRepairCountKey  = "aegis.io/node-repair"
)

type PatchStringValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

type WaitCancelFunc func(context.Context) bool

type HardwareType string

const (
	HardwareTypeBaseBoard = "baseboard"
	HardwareTypeGpu       = "gpu"
	HardwareTypeMemory    = "memory"
	HardwareTypeNetwork   = "network"
	HardwareTypeIB        = "ib"
	HardwareTypeCpu       = "cpu"
	HardwareTypeGpfs      = "gpfs"
	HardwareTypeDisk      = "disk"
	HardwareTypeUnknown   = "unknown"
	HardwareTypeNone      = "none"

	ComponentTypeDcgmExporter       = "dcgm-exporter"
	ComponentTypeNvidiaDevicePlugin = "nvidia-device-plugin"
	ComponentTypeRdmaDevicePlugin   = "rdma-device-plugin"
)

type ConditionType string

const (
	ConditionTypeNull                            ConditionType = "NULL"
	ConditionTypeBaseBoardCriticalIssue          ConditionType = "BaseBoardCriticalIssue"
	ConditionTypeCPUPressure                     ConditionType = "CPUPressure"
	ConditionTypeCpuUnhealthy                    ConditionType = "CpuUnhealthy"
	ConditionTypeDiskUnhealthy                   ConditionType = "DiskUnhealthy"
	ConditionTypeGpfsTestFailed                  ConditionType = "GpfsTestFailed"
	ConditionTypeGpfsIBNotConfig                 ConditionType = "GpfsIBNotConfig"
	ConditionTypeGpfsThreadDeadlock              ConditionType = "GpfsThreadDeadlock"
	ConditionTypeGpfsDown                        ConditionType = "GpfsDown"
	ConditionTypeGpfsMountLost                   ConditionType = "GpfsMountLost"
	ConditionTypeGpfsInactive                    ConditionType = "GpfsInactive"
	ConditionTypeGpfsRdmaStatusError             ConditionType = "GpfsRdmaStatusError"
	ConditionTypeGpfsQuorumConnectionDown        ConditionType = "GpfsQuorumConnectionDown"
	ConditionTypeGpfsExpelledFromCluster         ConditionType = "GpfsExpelledFromCluster"
	ConditionTypeGpfsTimeClockError              ConditionType = "GpfsTimeClockError"
	ConditionTypeGpfsOsLockup                    ConditionType = "GpfsOsLockup"
	ConditionTypeGpfsBadTcpState                 ConditionType = "GpfsBadTcpState"
	ConditionTypeGpfsUnauthorized                ConditionType = "GpfsUnauthorized"
	ConditionTypeGpfsBond0Lost                   ConditionType = "GpfsBond0Lost"
	ConditionTypeGpuApplicationFrequentError     ConditionType = "GpuApplicationFrequentError"
	ConditionTypeGpuHung                         ConditionType = "GpuHung"
	ConditionTypeGpuCheckFailed                  ConditionType = "GpuCheckFailed"
	ConditionTypeGpuDown                         ConditionType = "GpuDown"
	ConditionTypeXIDECCMemoryErr                 ConditionType = "XIDECCMemoryErr"
	ConditionTypeXIDHWSystemErr                  ConditionType = "XIDHWSystemErr"
	ConditionTypeXIDUnclassifiedErr              ConditionType = "XIDUnclassifiedErr"
	ConditionTypeGpuMetricsHang                  ConditionType = "GpuMetricsHang"
	ConditionTypeGpuTooManyPageRetired           ConditionType = "GpuTooManyPageRetired"
	ConditionTypeGpuPcieDowngraded               ConditionType = "GpuPcieDowngraded"
	ConditionTypeGpuRegisterFailed               ConditionType = "GpuRegisterFailed"
	ConditionTypeGpuRowRemappingPending          ConditionType = "GpuRowRemappingPending"
	ConditionTypeGpuRowRemappingFailure          ConditionType = "GpuRowRemappingFailure"
	ConditionTypeGpuSramUncorrectable            ConditionType = "GpuSramUncorrectable"
	ConditionTypeGpuAggSramUncorrectable         ConditionType = "GpuAggSramUncorrectable"
	ConditionTypeGpuVolSramUncorrectable         ConditionType = "GpuVolSramUncorrectable"
	ConditionTypeGpuVolDramUncorrectable         ConditionType = "GpuVolDramUncorrectable"
	ConditionTypeGpuVolDramCorrectable           ConditionType = "GpuVolDramCorrectable"
	ConditionTypeGpuNvlinkInactive               ConditionType = "GPUNvlinkInactive"
	ConditionTypeGpuGpuHWSlowdown                ConditionType = "GPUHWSlowdown"
	ConditionTypeGPUPersistenceModeNotEnabled    ConditionType = "GPUPersistenceModeNotEnabled"
	ConditionTypeHighGpuMemoryTemp               ConditionType = "HighGpuMemoryTemp"
	ConditionTypeHighGpuTemp                     ConditionType = "HighGpuTemp"
	ConditionTypeIBDown                          ConditionType = "IBDown"
	ConditionTypeRoceDeviceBroken                ConditionType = "RoceDeviceBroken"
	ConditionTypeIBLinkFrequentDown              ConditionType = "IBLinkFrequentDown"
	ConditionTypeIBPcieDowngraded                ConditionType = "IBPcieDowngraded"
	ConditionTypeIBModuleNotInstalled            ConditionType = "IBModuleNotInstalled"
	ConditionTypeIBRegisterFailed                ConditionType = "IBRegisterFailed"
	ConditionTypeRoceRegisterFailed              ConditionType = "RoceRegisterFailed"
	ConditionTypeIBSymbolError                   ConditionType = "IBSymbolError"
	ConditionTypeMemoryPressure                  ConditionType = "MemoryPressure"
	ConditionTypeKubeletMemoryPressure           ConditionType = "KubeletMemoryPressure"
	ConditionTypeMemoryUnhealthy                 ConditionType = "MemoryUnhealthy"
	ConditionTypeNetworkLinkFrequentDown         ConditionType = "NetworkLinkFrequentDown"
	ConditionTypeNetworkLinkTooManyDown          ConditionType = "NetworkLinkTooManyDown"
	ConditionTypeNetworkICETXTimeout             ConditionType = "ICETxTimeout"
	ConditionTypeNodeCordon                      ConditionType = "NodeCordon"
	ConditionTypeKubeletFailedCreatePodContainer ConditionType = "KubeletFailedCreatePodContainer"
	ConditionTypeNodeFrequentDown                ConditionType = "NodeFrequentDown"
	ConditionTypeNodeInhibitAll                  ConditionType = "NodeInhibitAll"
	ConditionTypeNodeNotReady                    ConditionType = "NodeNotReady"
	ConditionTypeHighDProcessesCount             ConditionType = "HighDProcessesCount"
	ConditionTypeHighZombieProcessesCount        ConditionType = "HighZombieProcessesCount"
	ConditionTypePeerMemModuleNotReady           ConditionType = "PeerMemModuleNotReady"
	ConditionTypePeerMemModuleNotConfig          ConditionType = "PeerMemModuleNotConfig"
)

type RemedyAction string

const (
	BreakDeadlockRemedyAction RemedyAction = "breakDeadlock"
	DropCacheRemedyAction     RemedyAction = "DropCache"
	PeerMemRemedyAction       RemedyAction = "ConfigPeerMem"
)
