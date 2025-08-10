package deviceaware

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/scitix/aegis/pkg/prom"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	AEGIS_DEVICE_ANNOTATION = "aegis.io/device-errors"
)

type DeviceAwareController struct {
	informer *NodeStatusInformer
	handler  *NodeStatusHandler
}

type NodeStatusHandler struct {
	kubeClient clientset.Interface
	nodeLister corelisters.NodeLister
	nodeSynced cache.InformerSynced
}

func (h *NodeStatusHandler) OnAdd(new *NodeStatus) {
	klog.Infof("[NODE ADD/UPDATE] %s status: %v", new.NodeName, new.StatusMap)

	node, err := h.nodeLister.Get(new.NodeName)
	if err != nil {
		klog.Errorf("failed to get node %s, %v", new.NodeName, err)
		return
	}

	value, err := json.Marshal(new.StatusMap)
	if err != nil {
		klog.Errorf("failed to marshal status map %v, %v", new.StatusMap, err)
		return
	}

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[AEGIS_DEVICE_ANNOTATION] = string(value)

	_, err = h.kubeClient.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update node %s, %v", new.NodeName, err)
		return
	}
}

func (h *NodeStatusHandler) OnUpdate(old, new *NodeStatus) {
	h.OnAdd(new)
}

func (h *NodeStatusHandler) OnDelete(old *NodeStatus) {
	klog.Infof("[NODE DELETE] %s status: %v", old.NodeName, old.StatusMap)
	node, err := h.nodeLister.Get(old.NodeName)
	if err != nil {
		klog.Errorf("failed to get node %s, %v", old.NodeName, err)
		return
	}

	if node.Annotations[AEGIS_DEVICE_ANNOTATION] != "" {
		delete(node.Annotations, AEGIS_DEVICE_ANNOTATION)
	}

	_, err = h.kubeClient.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update node %s, %v", old.NodeName, err)
		return
	}
}

func InitDeviceStatusCache(kubeclient clientset.Interface) (map[string]*NodeStatus, error) {
	statuses := make(map[string]*NodeStatus)
	nodes, err := kubeclient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	for _, node := range nodes.Items {
		annotations := node.Annotations
		if annotations == nil {
			continue
		}

		if status, ok := annotations[AEGIS_DEVICE_ANNOTATION]; ok {
			s := make(map[DeviceType]string)
			err := json.Unmarshal([]byte(status), &s)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal node %s device status: %v", node.Name, err)
			}

			vm := make(map[DeviceType]int64)
			vs := make([]int64, 0)
			for d, ds := range s {
				hash := hashStringToInt64(ds)
				vm[d] = hash
				vs = append(vs, hash)
			}

			statuses[node.Name] = &NodeStatus{
				NodeName:   node.Name,
				StatusMap:  s,
				Version:    hashInt64SliceToInt64(vs),
				VersionMap: vm,
				Timestamp:  time.Now(),
			}
		}
	}

	return statuses, nil
}

func NewController(kubeclient clientset.Interface,
	nodeInformer coreinformers.NodeInformer) (*DeviceAwareController, error) {

	cache, err := InitDeviceStatusCache(kubeclient)
	if err != nil {
		return nil, err
	}
	klog.V(4).Info("device status cache initialized:")
	for node, status := range cache {
		klog.V(4).Infof("	node %s: %+v", node, status)
	}

	handler := &NodeStatusHandler{
		kubeClient: kubeclient,
		nodeLister: nodeInformer.Lister(),
		nodeSynced: nodeInformer.Informer().HasSynced,
	}

	informer := NewNodeStatusInformer(
		prom.GetPromAPI(),
		handler,
		cache,
		[]DeviceType{
			DeviceTypeBaseboard,
			DeviceTypeCPU,
			DeviceTypeMemory,
			DeviceTypeDisk,
			DeviceTypeNetwork,
			DeviceTypeIB,
			DeviceTypeGPU,
		},
		10*time.Second,
	)

	controller := &DeviceAwareController{
		handler:  handler,
		informer: informer,
	}

	return controller, nil
}

func (d *DeviceAwareController) Run(ctx context.Context) error {
	klog.Info("Waiting for node informer caches to sync")
	if ok := cache.WaitForNamedCacheSync("node", ctx.Done(), d.handler.nodeSynced); !ok {
		return fmt.Errorf("failed to wait for node cache to sync")
	}

	go d.informer.Run(ctx)

	<-ctx.Done()
	klog.Info("Stopping device aware worker")
	return nil
}