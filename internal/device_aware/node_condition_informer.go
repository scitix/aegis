package deviceaware

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/binary"

	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"k8s.io/klog/v2"
)

type DeviceType string

const (
	DeviceTypeGPU       DeviceType = "gpu"
	DeviceTypeGPFS      DeviceType = "gpfs"
	DeviceTypeIB        DeviceType = "ib"
	DeviceTypeRoce      DeviceType = "roce"
	DeviceTypeCPU       DeviceType = "cpu"
	DeviceTypeMemory    DeviceType = "memory"
	DeviceTypeNetwork   DeviceType = "network"
	DeviceTypeSystem    DeviceType = "system"
	DeviceTypeDisk      DeviceType = "disk"
	DeviceTypeDefault   DeviceType = "default"
	DeviceTypeBaseboard DeviceType = "baseboard"
)

var defaultDeviceId = "all"

// 节点状态集合
type NodeStatus struct {
	NodeName   string
	StatusMap  map[DeviceType]string // 存储各维度状态（例如：gpu、ib、cpu）
	Version    int64                 // 全局版本号
	VersionMap map[DeviceType]int64
	Timestamp  time.Time
}

// 事件处理器接口
type NodeStatusEventHandler interface {
	OnAdd(status *NodeStatus)
	OnUpdate(old, new *NodeStatus)
	OnDelete(status *NodeStatus)
}

// 复合型Informer
type NodeStatusInformer struct {
	client       *prom.PromAPI
	cache        map[string]*NodeStatus // key: node name
	cacheLock    sync.RWMutex
	handler      NodeStatusEventHandler
	resyncPeriod time.Duration
	types        []DeviceType // 监控的设备类型
}

func NewNodeStatusInformer(client *prom.PromAPI, handler NodeStatusEventHandler, cache map[string]*NodeStatus, types []DeviceType, resyncPeriod time.Duration) *NodeStatusInformer {
	return &NodeStatusInformer{
		client:       client,
		cache:        cache,
		handler:      handler,
		resyncPeriod: resyncPeriod,
		types:        types,
	}
}

// 主运行循环
func (i *NodeStatusInformer) Run(ctx context.Context) {
	ticker := time.NewTicker(i.resyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.fullSync(ctx)
		}
	}
}

// 全量同步
func (i *NodeStatusInformer) fullSync(ctx context.Context) {
	current, err := i.fetchAllStatus(ctx)
	if err != nil {
		klog.Errorf("Error fetching status: %v\n", err)
		return
	}

	i.cacheLock.Lock()
	defer i.cacheLock.Unlock()

	// 预防性阻塞
	if len(i.cache) > 2 && len(current) == 0 {
		klog.Warningf("存量节点设备异常数：%d， 当前所有节点状态为空, 等待下次同步", len(i.cache))
		return
	}

	// 处理删除和更新
	for nodeName, oldStatus := range i.cache {
		if newStatus, exists := current[nodeName]; exists {
			if newStatus.Version != oldStatus.Version {
				i.handler.OnUpdate(oldStatus, newStatus)
				i.cache[nodeName] = newStatus
			}
		} else {
			// 创建删除标记状态
			i.handler.OnDelete(oldStatus)
			delete(i.cache, nodeName)
		}
	}

	// 处理新增
	for nodeName, newStatus := range current {
		if _, exists := i.cache[nodeName]; !exists {
			i.handler.OnAdd(newStatus)
			i.cache[nodeName] = newStatus
		}
	}
}

// 获取所有监控指标的状态
func (i *NodeStatusInformer) fetchAllStatus(ctx context.Context) (map[string]*NodeStatus, error) {
	result := make(map[string]*NodeStatus)

	// 并发获取所有指标
	var wg sync.WaitGroup
	var mutex sync.Mutex
	errChan := make(chan error, len(i.types))

	for _, deviceType := range i.types {
		wg.Add(1)
		go func(t DeviceType) {
			defer wg.Done()

			data, err := i.queryMetric(ctx, deviceType)
			if err != nil {
				errChan <- err
				return
			}

			mutex.Lock()
			defer mutex.Unlock()
			for node, status := range data {
				if existing, ok := result[node]; ok {
					// 合并状态
					for k, v := range status.StatusMap {
						existing.StatusMap[k] = v
						existing.VersionMap[k] = status.Version
					}
					// 取最新时间戳
					if status.Timestamp.After(existing.Timestamp) {
						existing.Timestamp = status.Timestamp
					}
				} else {
					result[node] = status
				}
			}
		}(deviceType)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return nil, fmt.Errorf("error fetching metrics: %v", <-errChan)
	}

	for node, status := range result {
		vs := make([]int64, 0)
		for _, version := range status.VersionMap {
			vs = append(vs, version)
		}

		result[node].Version = hashInt64SliceToInt64(vs)
	}

	return result, nil
}

func hashInt64SliceToInt64(vs []int64) int64 {
	// 排序
	sort.Slice(vs, func(i, j int) bool {
		return vs[i] < vs[j]
	})

	data := make([]byte, 0, 8*len(vs))
	for _, v := range vs {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(v))
		data = append(data, b...)
	}

	return hashStringToInt64(string(data))
}

func hashStringToInt64(s string) int64 {
	h := sha256.Sum256([]byte(s))
	// 取前8字节转成 int64
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// 查询单个类型指标
func (i *NodeStatusInformer) queryMetric(ctx context.Context, deviceType DeviceType) (map[string]*NodeStatus, error) {
	query := fmt.Sprintf("aegis_node_status_condition{type=\"%s\"}", deviceType)
	results, err := i.client.ListNodeStatusesWithQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	nodeStatuses := make(map[string][]prom.AegisNodeStatus)
	for i, result := range results {
		node := result.Name
		if len(nodeStatuses[node]) > 0 {
			nodeStatuses[node] = append(nodeStatuses[node], results[i])
		} else {
			nodeStatuses[node] = []prom.AegisNodeStatus{results[i]}
		}
	}

	statusMap := make(map[string]*NodeStatus)

	for node, statuses := range nodeStatuses {
		// 根据指标类型解析值
		var s string
		switch deviceType {
		case DeviceTypeBaseboard:
			s = parseBaseboardStatus(statuses)
		case DeviceTypeCPU:
			s = parseCPUStatus(statuses)
		case DeviceTypeMemory:
			s = parseMemoryStatus(statuses)
		case DeviceTypeDisk:
			s = parseDiskStatus(statuses)
		case DeviceTypeGPU:
			s = parseGPUStatus(statuses)
		case DeviceTypeGPFS:
			s = parseGPFSDeviceStatus(statuses)
		case DeviceTypeIB:
			s = parseIBDeviceStatus(statuses)
		case DeviceTypeRoce:
			s = parseRoceDeviceStatus(statuses)
		case DeviceTypeNetwork:
			s = parseNetworkStatus(statuses)
		default:
			klog.Warningf("unsupported device type: %s", deviceType)
		}

		if s != "" {
			if _, exists := statusMap[node]; !exists {
				statusMap[node] = &NodeStatus{
					NodeName:  node,
					StatusMap: make(map[DeviceType]string),
					Timestamp: time.Now(),
				}
			}
			statusMap[node].StatusMap[deviceType] = s

			// 版本签名
			statusMap[node].VersionMap = map[DeviceType]int64{
				deviceType: hashStringToInt64(s),
			}
		}
	}

	return statusMap, nil
}

// baseboard 设备状态解析
func parseBaseboardStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]string, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeBaseBoardCriticalIssue):
			if status.ID != "" {
				disabled = append(disabled, status.ID)
			}
		default:
			continue
		}
	}

	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// cpu 设备状态解析
func parseCPUStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]string, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeCpuUnhealthy):
			if status.ID != "" {
				disabled = append(disabled, status.ID)
			}
		default:
			continue
		}
	}

	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// memory 设备状态解析
func parseMemoryStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]string, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeMemoryUnhealthy):
			if status.ID != "" {
				disabled = append(disabled, status.ID)
			}
		default:
			continue
		}
	}

	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// disk 设备状态解析
func parseDiskStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]string, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeDiskPressure):
			fallthrough
		case string(basic.ConditionTypeDiskUnhealthy):
			if status.ID != "" {
				disabled = append(disabled, status.ID)
			}
		default:
			continue
		}
	}

	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// network 设备状态解析
func parseNetworkStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]string, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeNetworkLinkDown):
			if status.ID != "" {
				disabled = append(disabled, status.ID)
			}
		default:
			continue
		}
	}

	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// gpu 设备状态解析
func parseGPUStatus(statuses []prom.AegisNodeStatus) string {
	disabled := make([]bool, 8)

	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeGpuHung):
			for i, _ := range disabled {
				disabled[i] = true
			}
		case string(basic.ConditionTypeGpuCheckFailed):
			if status.ID != "" {
				for i, ch := range status.ID {
					num, err := strconv.ParseBool(string(ch))
					if err != nil {
						klog.Warningf("parse gpu index %d status %c failed: %s", i, ch, err)
						continue
					}

					disabled[i] = num
				}
			}
		case string(basic.ConditionTypeGpuNvlinkInactive):
			fallthrough
		case string(basic.ConditionTypeGpuTooManyPageRetired):
			fallthrough
		case string(basic.ConditionTypeGpuAggSramUncorrectable):
			fallthrough
		case string(basic.ConditionTypeGpuVolSramUncorrectable):
			fallthrough
		case string(basic.ConditionTypeGpuGpuHWSlowdown):
			fallthrough
		case string(basic.ConditionTypeGpuPcieGenDowngraded):
			fallthrough
		case string(basic.ConditionTypeGpuPcieWidthDowngraded):
			fallthrough
		case string(basic.ConditionTypeHighGpuTemp):
			fallthrough
		case string(basic.ConditionTypeHighGpuMemoryTemp):
			fallthrough
		case string(basic.ConditionTypeXid64ECCRowremapperFailure):
			fallthrough
		case string(basic.ConditionTypeXid74NVLinkError):
			fallthrough
		case string(basic.ConditionTypeXid79GPULost):
			if status.ID != "" {
				id, err := strconv.Atoi(status.ID)
				if err != nil || id > 7 {
					klog.Warningf("parse gpu index %s failed or invalid: %s", status.ID, err)
				} else {
					disabled[id] = true
				}
			}
		case string(basic.ConditionTypeXid48GPUMemoryDBE):
			fallthrough
		case string(basic.ConditionTypeXid63ECCRowremapperPending):
			fallthrough
		case string(basic.ConditionTypeGpuRegisterFailed):
			fallthrough
		case string(basic.ConditionTypeGpuMetricsHang):
			fallthrough
		case string(basic.ConditionTypeGpuRowRemappingFailure):
			continue
		default:
			klog.Warningf("unsupported condition type %s", status.Type)
		}
	}

	indexs := make([]string, 0)
	for i, disable := range disabled {
		if disable {
			indexs = append(indexs, fmt.Sprintf("%d", i))
		}
	}

	return strings.Join(indexs, ",")
}

// gpfs 状态解析
func parseGPFSDeviceStatus(statuses []prom.AegisNodeStatus) string {
	disabledMap := make(map[string]bool, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeGpfsMountLost):
			if status.ID != "" {
				disabledMap[status.ID] = true
			}
		default:
			disabledMap[defaultDeviceId] = true
		}
	}

	disabled := make([]string, 0)
	for id, _ := range disabledMap {
		disabled = append(disabled, id)
	}
	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// ib 设备状态解析
func parseIBDeviceStatus(statuses []prom.AegisNodeStatus) string {
	disabledMap := make(map[string]bool, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeIBDown):
			fallthrough
		case string(basic.ConditionTypeIBPcieDowngraded):
			fallthrough
		case string(basic.ConditionTypeIBLinkFrequentDown):
			if status.ID != "" {
				disabledMap[status.ID] = true
			}
		default:
			continue
		}
	}

	disabled := make([]string, 0)
	for id, _ := range disabledMap {
		disabled = append(disabled, id)
	}
	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// roce 设备状态解析
func parseRoceDeviceStatus(statuses []prom.AegisNodeStatus) string {
	disabledMap := make(map[string]bool, 0)
	for _, status := range statuses {
		switch status.Condition {
		case string(basic.ConditionTypeRoceRegisterFailed):
			if status.ID != "" {
				disabledMap[status.ID] = true
			}
		case string(basic.ConditionTypeRoceDeviceBroken):
			disabledMap[defaultDeviceId] = true
		default:
			continue
		}
	}

	disabled := make([]string, 0)
	for id, _ := range disabledMap {
		disabled = append(disabled, id)
	}
	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}
