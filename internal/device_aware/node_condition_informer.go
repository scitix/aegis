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

// ConditionLookup is the subset of PriorityWatcher used by device-aware.
// Defining it here keeps device_aware free of a direct nodepoller dependency.
type ConditionLookup interface {
	IsLoadAffecting(condition string) bool
	GetIDMode(condition string) string
}

// 节点状态集合
type NodeStatus struct {
	NodeName    string
	StatusMap   map[DeviceType]string // 存储各维度状态（例如：gpu、ib、cpu）
	Version     int64                 // 全局版本号
	VersionMap  map[DeviceType]int64
	Timestamp   time.Time
	AffectsLoad bool // 当前节点是否存在影响负载的故障
}

// 事件处理器接口
type NodeStatusEventHandler interface {
	OnAdd(status *NodeStatus)
	OnUpdate(old, new *NodeStatus)
	OnDelete(status *NodeStatus)
}

// 复合型Informer
type NodeStatusInformer struct {
	client          *prom.PromAPI
	cache           map[string]*NodeStatus // key: node name
	cacheLock       sync.RWMutex
	handler         NodeStatusEventHandler
	resyncPeriod    time.Duration
	types           []DeviceType // 监控的设备类型
	conditionLookup ConditionLookup
}

func NewNodeStatusInformer(client *prom.PromAPI, handler NodeStatusEventHandler, cache map[string]*NodeStatus, types []DeviceType, resyncPeriod time.Duration, conditionLookup ConditionLookup) *NodeStatusInformer {
	return &NodeStatusInformer{
		client:          client,
		cache:           cache,
		handler:         handler,
		resyncPeriod:    resyncPeriod,
		types:           types,
		conditionLookup: conditionLookup,
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

			data, err := i.queryMetric(ctx, t)
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
						existing.VersionMap[k] = status.VersionMap[k]
					}
					// AffectsLoad: OR 聚合
					existing.AffectsLoad = existing.AffectsLoad || status.AffectsLoad
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
	for idx, result := range results {
		node := result.Name
		nodeStatuses[node] = append(nodeStatuses[node], results[idx])
	}

	statusMap := make(map[string]*NodeStatus)

	for node, statuses := range nodeStatuses {
		// 计算 AffectsLoad：遍历原始条件做 OR 聚合
		affectsLoad := false
		for _, s := range statuses {
			if i.conditionLookup.IsLoadAffecting(s.Condition) {
				affectsLoad = true
				break
			}
		}

		// 根据指标类型解析 StatusMap（数据驱动）
		var s string
		switch deviceType {
		case DeviceTypeBaseboard:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeCPU:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeMemory:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeDisk:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeNetwork:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeGPU:
			s = parseGPUStatus(statuses, i.conditionLookup)
		case DeviceTypeGPFS:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeIB:
			s = parseByIDMode(statuses, i.conditionLookup)
		case DeviceTypeRoce:
			s = parseByIDMode(statuses, i.conditionLookup)
		default:
			klog.Warningf("unsupported device type: %s", deviceType)
		}

		if s != "" || affectsLoad {
			if _, exists := statusMap[node]; !exists {
				statusMap[node] = &NodeStatus{
					NodeName:   node,
					StatusMap:  make(map[DeviceType]string),
					VersionMap: make(map[DeviceType]int64),
					Timestamp:  time.Now(),
				}
			}
			if s != "" {
				statusMap[node].StatusMap[deviceType] = s
				statusMap[node].VersionMap[deviceType] = hashStringToInt64(s)
			}
			statusMap[node].AffectsLoad = affectsLoad
		}
	}

	return statusMap, nil
}

// parseByIDMode handles all device types whose ID logic follows the standard
// "all" / "id" / "-" modes (Baseboard, CPU, Memory, Disk, Network, GPFS, IB, RoCE).
// For "id" mode: if status.ID is empty the entry falls back to defaultDeviceId,
// preserving the behaviour of the original per-device parse functions.
func parseByIDMode(statuses []prom.AegisNodeStatus, lookup ConditionLookup) string {
	disabledMap := make(map[string]bool)
	for _, status := range statuses {
		switch lookup.GetIDMode(status.Condition) {
		case "all":
			disabledMap[defaultDeviceId] = true
		case "id":
			if status.ID != "" {
				disabledMap[status.ID] = true
			} else {
				disabledMap[defaultDeviceId] = true
			}
		case "-", "":
			// explicitly ignored
		default:
			klog.Warningf("unexpected DeviceIDMode %q for condition %s in non-GPU device",
				lookup.GetIDMode(status.Condition), status.Condition)
		}
	}

	disabled := make([]string, 0, len(disabledMap))
	for id := range disabledMap {
		disabled = append(disabled, id)
	}
	sort.Strings(disabled)
	return strings.Join(disabled, ",")
}

// parseGPUStatus handles GPU-specific ID modes: "all", "mask", "index", "-".
func parseGPUStatus(statuses []prom.AegisNodeStatus, lookup ConditionLookup) string {
	const gpuCount = 8
	disabled := make([]bool, gpuCount)

	for _, status := range statuses {
		switch lookup.GetIDMode(status.Condition) {
		case "all":
			for i := range disabled {
				disabled[i] = true
			}
		case "mask":
			for i, ch := range status.ID {
				if i >= gpuCount {
					break
				}
				b, err := strconv.ParseBool(string(ch))
				if err != nil {
					klog.Warningf("parse gpu mask index %d char %c for condition %s: %v",
						i, ch, status.Condition, err)
					continue
				}
				disabled[i] = b
			}
		case "index":
			if status.ID != "" {
				id, err := strconv.Atoi(status.ID)
				if err != nil || id >= gpuCount {
					klog.Warningf("parse gpu index %q for condition %s failed or out of range: %v",
						status.ID, status.Condition, err)
				} else {
					disabled[id] = true
				}
			}
		case "-", "":
			// explicitly ignored
		default:
			klog.Warningf("unexpected DeviceIDMode %q for GPU condition %s",
				lookup.GetIDMode(status.Condition), status.Condition)
		}
	}

	indexes := make([]string, 0, gpuCount)
	for i, d := range disabled {
		if d {
			indexes = append(indexes, strconv.Itoa(i))
		}
	}
	return strings.Join(indexes, ",")
}
