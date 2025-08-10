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

var SleepWaitDuration = time.Minute * time.Duration(30)

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
	ComponentTypeRoceDevicePlugin   = "kube-sriov-device-plugin"
)

type ConditionType string

const (
	ConditionTypeNull                            ConditionType = "NULL"
	
	// baseboard
	ConditionTypeBaseBoardCriticalIssue ConditionType = "BaseBoardCriticalIssue"

	// cpu
	ConditionTypeCPUPressure  ConditionType = "CPUPressure"
	ConditionTypeCpuUnhealthy ConditionType = "CpuUnhealthy"

	// disk
	ConditionTypeDiskPressure  ConditionType = "DiskPressure"
	ConditionTypeDiskUnhealthy ConditionType = "DiskUnhealthy"

	// memory
	ConditionTypeMemoryPressure        ConditionType = "MemoryPressure"
	ConditionTypeKubeletMemoryPressure ConditionType = "KubeletMemoryPressure"
	ConditionTypeMemoryUnhealthy       ConditionType = "MemoryUnhealthy"

	// network
	ConditionTypeNetworkLinkDown ConditionType = "NetworkLinkDown"

	// system
	ConditionTypeHighZombieProcessesCount ConditionType = "HighZombieProcessesCount"

	// ib
	ConditionTypeIBLinkFrequentDown ConditionType = "IBLinkFrequentDown"
	ConditionTypeIBDown             ConditionType = "IBDown"
	ConditionTypeIBRegisterFailed   ConditionType = "IBRegisterFailed"
	ConditionTypeIBPcieDowngraded   ConditionType = "IBPcieDowngraded"
	ConditionTypeRoceRegisterFailed ConditionType = "RoceRegisterFailed"
	ConditionTypeRoceDeviceBroken   ConditionType = "RoceDeviceBroken"

	// gpfs
	ConditionTypeGpfsDown           ConditionType = "GpfsDown"
	ConditionTypeGpfsMountLost      ConditionType = "GpfsMountLost"
	ConditionTypeGpfsThreadDeadlock ConditionType = "GpfsThreadDeadlock"
	ConditionTypeGpfsTestFailed     ConditionType = "GpfsTestFailed"
	ConditionTypeGpfsRdmaError      ConditionType = "GpfsRdmaError"
	ConditionTypeGpfsNodeNotHealthy ConditionType = "GpfsNodeNotHealthy"
	ConditionTypeGpfsNotMounted     ConditionType = "GpfsNotMounted"
	ConditionTypeGpfsNotStarted     ConditionType = "GpfsNotStarted"
	ConditionTypeGpfsNotInCluster   ConditionType = "GpfsNotInCluster"
	ConditionTypeGpfsNotInstalled   ConditionType = "GpfsNotInstalled"
	ConditionTypeGpfsIBNotConfig    ConditionType = "GpfsIBNotConfig"

	// gpu
	ConditionTypeGpuHung                      ConditionType = "GpuHung"
	ConditionTypeGpuCheckFailed               ConditionType = "GpuCheckFailed"
	ConditionTypeGpuRegisterFailed            ConditionType = "GpuRegisterFailed"
	ConditionTypeHighGpuMemoryTemp            ConditionType = "HighGpuMemoryTemp"
	ConditionTypeHighGpuTemp                  ConditionType = "HighGpuTemp"
	ConditionTypeXIDECCMemoryErr              ConditionType = "XIDECCMemoryErr"
	ConditionTypeXIDHWSystemErr               ConditionType = "XIDHWSystemErr"
	ConditionTypeGpuRowRemappingPending       ConditionType = "GpuRowRemappingPending"
	ConditionTypeGpuRowRemappingFailure       ConditionType = "GpuRowRemappingFailure"
	ConditionTypeGpuTooManyPageRetired        ConditionType = "GpuTooManyPageRetired"
	ConditionTypeGpuAggSramUncorrectable      ConditionType = "GpuAggSramUncorrectable"
	ConditionTypeGpuVolSramUncorrectable      ConditionType = "GpuVolSramUncorrectable"
	ConditionTypeGpuVolDramUncorrectable      ConditionType = "GpuVolDramUncorrectable"
	ConditionTypeNvidiaFabricManagerNotActive ConditionType = "NvidiaFabricManagerNotActive"
	ConditionTypeGpuDown                      ConditionType = "GpuDown"
	ConditionTypeGpuPcieDowngraded            ConditionType = "GpuPcieDowngraded"
	ConditionTypeGpuGpuHWSlowdown             ConditionType = "GPUHWSlowdown"
	ConditionTypeGpuNvlinkInactive            ConditionType = "GPUNvlinkInactive"
	ConditionTypeGPUPersistenceModeNotEnabled ConditionType = "GPUPersistenceModeNotEnabled"
	ConditionTypeGpuMetricsHang               ConditionType = "GpuMetricsHang"

	// default
	ConditionTypeNodeCordon                      ConditionType = "NodeCordon"
	ConditionTypeNodeNotReady                    ConditionType = "NodeNotReady"
	ConditionTypeNodeHasRestart                  ConditionType = "NodeHasRestart"
	ConditionTypeKubeletFailedCreatePodContainer ConditionType = "KubeletFailedCreatePodContainer"
	ConditionTypeNodeFrequentDown                ConditionType = "NodeFrequentDown"
	ConditionTypeNodeInhibitAll                  ConditionType = "NodeInhibitAll"
	ConditionTypeNodeHasTerminatingPod           ConditionType = "NodeHasTerminatingPod"
)

type RemedyAction string

const (
	BreakDeadlockRemedyAction RemedyAction = "breakDeadlock"
	DropCacheRemedyAction     RemedyAction = "DropCache"
	PeerMemRemedyAction       RemedyAction = "ConfigPeerMem"
	RestartKubeletAction      RemedyAction = "RestartKubelet"
)
