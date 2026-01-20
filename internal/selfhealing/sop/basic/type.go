package basic

import (
	"context"
	"os"
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

var SleepWaitDuration = time.Minute * time.Duration(10)

var GPUPluginPodSelector = os.Getenv("GPU_DEVICE_PLUGIN_SELECTOR")
var InfinibandPluginPodSelector = os.Getenv("INFINIBAND_DEVICE_PLUGIN_SELECTOR")
var RocePluginPodSelector = os.Getenv("ROCE_DEVICE_PLUGIN_SELECTOR")

const (
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
)

const (
	ModelTypeHardware = "hardware"
	ModelTypeKubelet  = "kubelet"
)

const (
	ComponentTypeKebelet            = "kubelet"
	ComponentTypeDcgmExporter       = "dcgm-exporter"
	ComponentTypeNvidiaDevicePlugin = "nvidia-device-plugin"
	ComponentTypeRdmaDevicePlugin   = "rdma-device-plugin"
	ComponentTypeRoceDevicePlugin   = "kube-sriov-device-plugin"
)

type ConditionType string

const (
	ConditionTypeNull ConditionType = "NULL"

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
	ConditionTypeIBLinkFrequentDown    ConditionType = "IBLinkFrequentDown"
	ConditionTypeIBLost                ConditionType = "IBLost"
	ConditionTypeIBRegisterFailed      ConditionType = "IBRegisterFailed"
	ConditionTypeIBModuleLost          ConditionType = "IBModuleLost"
	ConditionTypeIBNetDriverFailedLoad ConditionType = "IBNetDriverFailedLoad"
	ConditionTypeIBPCIeMRRNotAlign     ConditionType = "IBPCIeMRRNotAlign"
	ConditionTypeIBPortSpeedAbnormal   ConditionType = "IBPortSpeedAbnormal"
	ConditionTypeIBPCIeSpeedAbnormal   ConditionType = "IBPCIeSpeedAbnormal"
	ConditionTypeIBPCIeWidthAbnormal   ConditionType = "IBPCIeWidthAbnormal"
	ConditionTypeIBLinkAbnormal        ConditionType = "IBLinkAbnormal"
	ConditionTypeIBProtoclAbnormal     ConditionType = "IBProtoclAbnormal"

	// roce
	ConditionTypeRoceRegisterFailed        ConditionType = "RoceRegisterFailed"
	ConditionTypeRoceDeviceBroken          ConditionType = "RoceDeviceBroken"
	ConditionTypeRoceHostOffline           ConditionType = "RoceHostOffline"
	ConditionTypeRoceHostGatewayNotMatch   ConditionType = "RoceHostGatewayNotMatch"
	ConditionTypeRoceHostRouteMiss         ConditionType = "RoceHostRouteMiss"
	ConditionTypeRocePodOffline            ConditionType = "RocePodOffline"
	ConditionTypeRocePodGatewayNotMatch    ConditionType = "RocePodGatewayNotMatch"
	ConditionTypeRocePodRouteMiss          ConditionType = "RocePodRouteMiss"
	ConditionTypeRoceNodeLabelMiss         ConditionType = "RoceNodeLabelMiss"
	ConditionTypeRocePodDeviceMiss         ConditionType = "RocePodDeviceMiss"
	ConditionTypeRoceNodeResourceMiss      ConditionType = "RoceNodeResourceMiss"
	ConditionTypeRoceVfDeviceMiss          ConditionType = "RoceVfDeviceMiss"
	ConditionTypeRoceSriovInitError        ConditionType = "RoceSriovInitError"
	ConditionTypeRoceNodeUnitLabelMiss     ConditionType = "RoceNodeUnitLabelMiss"
	ConditionTypeRoceNodePfNamesLabelMiss  ConditionType = "RoceNodePfNamesLabelMiss"
	ConditionTypeRoceNodeResourceLabelMiss ConditionType = "RoceNodeResourceLabelMiss"
	ConditionTypeRoceNodeNetworkLabelMiss  ConditionType = "RoceNodeNetworkLabelMiss"

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
	ConditionTypeGpuHung                        ConditionType = "GpuHung"
	ConditionTypeGpuErrResetRequired            ConditionType = "GpuErrResetRequired"
	ConditionTypeGpuCheckFailed                 ConditionType = "GpuCheckFailed"
	ConditionTypeGpuRegisterFailed              ConditionType = "GpuRegisterFailed"
	ConditionTypeHighGpuMemoryTemp              ConditionType = "HighGpuMemoryTemp"
	ConditionTypeHighGpuTemp                    ConditionType = "HighGpuTemp"
	ConditionTypeXid48GPUMemoryDBE              ConditionType = "Xid48GPUMemoryDBE"
	ConditionTypeXid63ECCRowremapperPending     ConditionType = "Xid63ECCRowremapperPending"
	ConditionTypeXid64ECCRowremapperFailure     ConditionType = "Xid64ECCRowremapperFailure"
	ConditionTypeXid92HighSingleBitECCErrorRate ConditionType = "Xid92HighSingleBitECCErrorRate"
	ConditionTypeXid95UncontainedECCError       ConditionType = "Xid95UncontainedECCError"
	ConditionTypeXid74NVLinkError               ConditionType = "Xid74NVLinkError"
	ConditionTypeXid79GPULost                   ConditionType = "Xid79GPULost"
	ConditionTypeGpuRowRemappingPending         ConditionType = "GpuRowRemappingPending"
	ConditionTypeGpuRowRemappingFailure         ConditionType = "GpuRowRemappingFailure"
	ConditionTypeGpuTooManyPageRetired          ConditionType = "GpuTooManyPageRetired"
	ConditionTypeGpuAggSramUncorrectable        ConditionType = "GpuAggSramUncorrectable"
	ConditionTypeGpuVolSramUncorrectable        ConditionType = "GpuVolSramUncorrectable"
	ConditionTypeGpuSmClkSlowDown               ConditionType = "GpuSmClkSlowDown"
	ConditionTypeNvidiaFabricManagerNotActive   ConditionType = "NvidiaFabricManagerNotActive"
	ConditionTypeGpuPcieLinkDegraded            ConditionType = "GpuPcieLinkDegraded"
	// Deprecated: use ConditionTypeGpuPcieLinkDegraded instead
	ConditionTypeGpuGpuHWSlowdown             ConditionType = "GpuHWSlowdown"
	ConditionTypeGpuNvlinkInactive            ConditionType = "GpuNvlinkInactive"
	ConditionTypeGpuNvlinkError               ConditionType = "GpuNvlinkError"
	ConditionTypeGPUPersistenceModeNotEnabled ConditionType = "GpuPersistenceModeNotEnabled"
	ConditionTypeGpuMetricsHang               ConditionType = "GpuMetricsHang"
	ConditionTypeGpuP2PNotSupported           ConditionType = "GpuP2PNotSupported"
	ConditionTypeGPUIbgdaNotEnabled           ConditionType = "GPUIbgdaNotEnabled"

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
	BreakDeadlockRemedyAction  RemedyAction = "breakDeadlock"
	DropCacheRemedyAction      RemedyAction = "DropCache"
	PeerMemRemedyAction        RemedyAction = "ConfigPeerMem"
	RestartKubeletAction       RemedyAction = "RestartKubelet"
	RestartFabricmanagerAction RemedyAction = "RestartFabricmanager"
	EnableGpuPersistenceAction RemedyAction = "EnableGpuPersistence"
)
