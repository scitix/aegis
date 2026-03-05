package nodepoller

import (
	"context"
	"sync"
	"time"

	pkgcontroller "github.com/scitix/aegis/pkg/controller"
	"github.com/scitix/aegis/pkg/prom"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const nodecheckTaintKey = "scitix.ai/nodecheck"

// PollerConfig holds all tunables for NodeStatusPoller.
type PollerConfig struct {
	Enabled bool

	// How often to query Prometheus and run edge detection (default 10s).
	PollInterval time.Duration

	// How often to re-trigger all critical-cache entries (default 4h).
	ResyncInterval time.Duration

	// How often to re-trigger all cordon-only-cache entries (default 10min).
	CordonResyncInterval time.Duration

	// Maximum number of rising-edge alerts per poll round (default 20).
	MaxAlertsPerRound int

	// ConfigMap name for priority config (default "aegis-priority").
	PriorityConfigMap string

	// Namespace of the priority ConfigMap (default "monitoring").
	PriorityNamespace string

	// ConfigMap data key for priority config (default "priority").
	PriorityConfigKey string

	// Alert creation fields passed through from main controller config.
	PublishNamespace        string
	SystemParas             map[string]string
	DefaultTTLAfterOpsSucceed int32
	DefaultTTLAfterOpsFailed  int32
	DefaultTTLAfterNoOps      int32
}

func (c *PollerConfig) applyDefaults() {
	if c.PollInterval == 0 {
		c.PollInterval = 10 * time.Second
	}
	if c.ResyncInterval == 0 {
		c.ResyncInterval = 4 * time.Hour
	}
	if c.CordonResyncInterval == 0 {
		c.CordonResyncInterval = 10 * time.Minute
	}
	if c.MaxAlertsPerRound == 0 {
		c.MaxAlertsPerRound = 20
	}
	if c.PriorityConfigMap == "" {
		c.PriorityConfigMap = "aegis-priority"
	}
	if c.PriorityNamespace == "" {
		c.PriorityNamespace = "monitoring"
	}
	if c.PriorityConfigKey == "" {
		c.PriorityConfigKey = "priority.conf"
	}
}

// NodeStatusPoller periodically polls Prometheus, classifies nodes, and
// creates AegisAlert CRDs to drive self-healing workflows.
type NodeStatusPoller struct {
	promClient     *prom.PromAPI
	cfg            PollerConfig
	priority       *PriorityWatcher
	alertInterface pkgcontroller.AlertControllerInterface

	criticalCache   map[string]*criticalEntry // node → entry
	cordonOnlyCache map[string]struct{}        // node → present (Prometheus-driven)
	nodecheckCache  map[string]struct{}        // node → present (nodecheck-taint-driven)
	cacheLock       sync.RWMutex

	nodeLister corelisters.NodeLister
}

// NewNodeStatusPoller constructs a NodeStatusPoller.
// priority must be the shared PriorityWatcher instance created by the top-level
// controller; its ConfigMap watch is started inside Run().
func NewNodeStatusPoller(
	promClient *prom.PromAPI,
	alertInterface pkgcontroller.AlertControllerInterface,
	nodeLister corelisters.NodeLister,
	cfg PollerConfig,
	priority *PriorityWatcher,
) *NodeStatusPoller {
	cfg.applyDefaults()

	return &NodeStatusPoller{
		promClient:      promClient,
		cfg:             cfg,
		priority:        priority,
		alertInterface:  alertInterface,
		criticalCache:   make(map[string]*criticalEntry),
		cordonOnlyCache: make(map[string]struct{}),
		nodecheckCache:  make(map[string]struct{}),
		nodeLister:      nodeLister,
	}
}

// Run starts the poller loops. Blocks until ctx is cancelled.
func (p *NodeStatusPoller) Run(ctx context.Context, kubeClient kubernetes.Interface) error {
	klog.Info("nodepoller: starting")

	// Start priority ConfigMap watcher.
	p.priority.RunConfigMapWatcher(ctx, kubeClient, p.cfg.PriorityNamespace, p.cfg.PriorityConfigMap)

	// Start node taint watcher (for nodecheck taint → down-edge-B).
	go p.runNodeTaintWatcher(ctx, kubeClient)

	pollTicker := time.NewTicker(p.cfg.PollInterval)
	resyncTicker := time.NewTicker(p.cfg.ResyncInterval)
	cordonTicker := time.NewTicker(p.cfg.CordonResyncInterval)
	defer pollTicker.Stop()
	defer resyncTicker.Stop()
	defer cordonTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("nodepoller: stopping")
			return nil
		case <-pollTicker.C:
			p.fullSync(ctx)
		case <-resyncTicker.C:
			p.criticalResync(ctx)
		case <-cordonTicker.C:
			p.cordonResync(ctx)
		}
	}
}

// fullSync queries Prometheus, classifies nodes, and fires rising-edge alerts.
func (p *NodeStatusPoller) fullSync(ctx context.Context) {
	klog.V(4).Info("nodepoller: starting fullSync")

	statuses, err := p.promClient.ListNodeStatusesWithQuery(ctx, "aegis_node_status_condition")
	if err != nil {
		klog.Errorf("nodepoller: prometheus query failed: %v", err)
		return
	}

	klog.V(4).Infof("nodepoller: fetched %d raw statuses from Prometheus", len(statuses))

	result := classify(statuses, p.nodeLister, p.priority)
	klog.Infof("nodepoller: classification complete - criticalSet=%d, cordonOnlySet=%d", len(result.criticalSet), len(result.cordonOnlySet))

	p.cacheLock.Lock()
	defer p.cacheLock.Unlock()

	// --- criticalSet edge detection ---
	// Rising edges (new nodes in criticalSet not in criticalCache)
	triggered := 0
	skipped := 0
	for node, nodeStatuses := range result.criticalSet {
		ver := statusVersion(nodeStatuses)
		if existing, ok := p.criticalCache[node]; ok {
			if existing.version == ver {
				skipped++
				continue // no change, noop
			}
			klog.V(4).Infof("nodepoller: node %s status version changed (%d -> %d), updating cache", node, existing.version, ver)
			// version changed: update entry but do NOT re-trigger
			existing.lastStatuses = nodeStatuses
			existing.version = ver
			continue
		}

		if triggered >= p.cfg.MaxAlertsPerRound {
			klog.Warningf("nodepoller: MaxAlertsPerRound (%d) reached, deferring remaining rising edges", p.cfg.MaxAlertsPerRound)
			break
		}

		klog.V(2).Infof("nodepoller: critical rising edge detected for node %s with %d conditions", node, len(nodeStatuses))
		id, err := p.onCriticalRisingEdge(ctx, node, nodeStatuses)
		if err != nil {
			klog.Errorf("nodepoller: failed to create NodeCriticalIssue alert for node %s: %v", node, err)
			continue
		}
		p.criticalCache[node] = &criticalEntry{
			alertName:    id,
			lastStatuses: nodeStatuses,
			version:      ver,
			since:        time.Now(),
		}
		triggered++
	}
	klog.V(4).Infof("nodepoller: criticalSet processing - triggered=%d, skipped=%d, total=%d", triggered, skipped, len(result.criticalSet))

	// Falling edges (nodes that disappeared from criticalSet)
	criticalFalling := 0
	for node := range p.criticalCache {
		if _, inCritical := result.criticalSet[node]; !inCritical {
			klog.V(4).Infof("nodepoller: critical falling edge for node %s", node)
			delete(p.criticalCache, node)
			criticalFalling++
		}
	}
	klog.V(4).Infof("nodepoller: criticalSet falling edges: %d removed from cache", criticalFalling)

	// --- cordonOnlySet edge detection ---
	cordonTriggered := 0
	for node := range result.cordonOnlySet {
		if _, ok := p.cordonOnlyCache[node]; !ok {
			klog.V(2).Infof("nodepoller: cordon-only rising edge detected for node %s", node)
			if _, err := p.onCordonOnlyRisingEdge(ctx, node); err != nil {
				klog.Errorf("nodepoller: failed to create NodeCriticalIssueDisappeared alert for node %s: %v", node, err)
				continue
			}
			p.cordonOnlyCache[node] = struct{}{}
			cordonTriggered++
		}
	}

	// Falling edges for cordon-only
	cordonFalling := 0
	for node := range p.cordonOnlyCache {
		if _, inCordon := result.cordonOnlySet[node]; !inCordon {
			klog.V(4).Infof("nodepoller: cordon-only falling edge for node %s", node)
			delete(p.cordonOnlyCache, node)
			cordonFalling++
		}
	}
	klog.V(4).Infof("nodepoller: cordonOnlySet processing - triggered=%d, falling=%d", cordonTriggered, cordonFalling)
	klog.V(4).Info("nodepoller: fullSync complete")
}

// criticalResync re-triggers NodeCriticalIssue alerts for nodes whose alert
// may have been TTL-cleaned but the condition is still present.
func (p *NodeStatusPoller) criticalResync(ctx context.Context) {
	p.cacheLock.Lock()
	defer p.cacheLock.Unlock()

	resyncCount := 0
	for node, entry := range p.criticalCache {
		exists := p.activeAlertExists(ctx, node, alertTypeNodeCriticalIssue)
		klog.V(4).Infof("nodepoller: criticalResync checking node %s - active alert exists: %v", node, exists)
		if exists {
			continue
		}
		klog.Infof("nodepoller: resync: active alert for node %s gone, recreating", node)
		id, err := p.onCriticalRisingEdge(ctx, node, entry.lastStatuses)
		if err != nil {
			klog.Errorf("nodepoller: resync: failed to recreate alert for node %s: %v", node, err)
			continue
		}
		entry.alertName = id
		resyncCount++
	}
	if resyncCount == 0 {
		klog.V(4).Infof("nodepoller: criticalResync complete - no alerts need resyncing (cache size: %d)", len(p.criticalCache))
	} else {
		klog.Infof("nodepoller: criticalResync complete - recreated %d alerts", resyncCount)
	}
}

// cordonResync re-triggers NodeCriticalIssueDisappeared for all cordon-only
// nodes (both Prometheus-driven and nodecheck-taint-driven).
func (p *NodeStatusPoller) cordonResync(ctx context.Context) {
	p.cacheLock.RLock()
	nodes := make([]string, 0, len(p.cordonOnlyCache)+len(p.nodecheckCache))
	for node := range p.cordonOnlyCache {
		nodes = append(nodes, node)
	}
	for node := range p.nodecheckCache {
		if _, alreadyAdded := p.cordonOnlyCache[node]; !alreadyAdded {
			nodes = append(nodes, node)
		}
	}
	p.cacheLock.RUnlock()

	klog.V(4).Infof("nodepoller: cordonResync processing %d nodes (cordonOnly: %d, nodecheck: %d)",
		len(nodes), len(p.cordonOnlyCache), len(p.nodecheckCache))

	successCount := 0
	for _, node := range nodes {
		if _, err := p.onCordonOnlyRisingEdge(ctx, node); err != nil {
			klog.Errorf("nodepoller: cordonResync: failed to retrigger for node %s: %v", node, err)
		} else {
			successCount++
		}
	}
	klog.V(4).Infof("nodepoller: cordonResync complete - %d/%d nodes processed successfully", successCount, len(nodes))
}

// runNodeTaintWatcher watches node spec changes and fires NodeCheck alerts
// when the scitix.ai/nodecheck taint appears.
func (p *NodeStatusPoller) runNodeTaintWatcher(ctx context.Context, kubeClient kubernetes.Interface) {
	klog.Info("nodepoller: starting node taint watcher")

	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	seenTaints := make(map[string]bool) // node → has nodecheck taint

	addCount, updateCount, deleteCount := 0, 0, 0

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			addCount++
			if addCount%100 == 1 {
				klog.V(4).Infof("nodepoller: taint watcher add handler called %d times", addCount)
			}
			node, ok := obj.(*corev1.Node)
			if !ok {
				klog.Warningf("nodepoller: add handler received non-node object: %T", obj)
				return
			}
			p.checkNodecheckTaint(ctx, node, seenTaints)
		},
		UpdateFunc: func(_, newObj any) {
			updateCount++
			if updateCount%100 == 1 {
				klog.V(4).Infof("nodepoller: taint watcher update handler called %d times", updateCount)
			}
			node, ok := newObj.(*corev1.Node)
			if !ok {
				klog.Warningf("nodepoller: update handler received non-node object: %T", newObj)
				return
			}
			p.checkNodecheckTaint(ctx, node, seenTaints)
		},
		DeleteFunc: func(obj any) {
			deleteCount++
			if deleteCount%100 == 1 {
				klog.V(4).Infof("nodepoller: taint watcher delete handler called %d times", deleteCount)
			}
			node, ok := obj.(*corev1.Node)
			if !ok {
				klog.Warningf("nodepoller: delete handler received non-node object: %T", obj)
				return
			}
			delete(seenTaints, node.Name)
			klog.V(4).Infof("nodepoller: removed node %s from taint cache (deleted)", node.Name)
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	klog.Info("nodepoller: node taint watcher cache synced")
	<-ctx.Done()
	klog.Info("nodepoller: node taint watcher stopped")
}

func (p *NodeStatusPoller) checkNodecheckTaint(ctx context.Context, node *corev1.Node, seenTaints map[string]bool) {
	hasTaint := false
	for _, t := range node.Spec.Taints {
		if t.Key == nodecheckTaintKey {
			hasTaint = true
			break
		}
	}

	wasPresent := seenTaints[node.Name]
	if hasTaint && !wasPresent {
		klog.V(2).Infof("nodepoller: detected nodecheck taint on node %s (rising edge)", node.Name)
		seenTaints[node.Name] = true

		// Only act when aegis.io/disable label is absent.
		if node.Labels["aegis.io/disable"] == "true" {
			klog.V(4).Infof("nodepoller: node %s has aegis.io/disable=true, skipping alert creation", node.Name)
			return
		}

		p.cacheLock.Lock()
		p.nodecheckCache[node.Name] = struct{}{}
		p.cacheLock.Unlock()
		klog.V(4).Infof("nodepoller: added node %s to nodecheckCache", node.Name)

		if _, err := p.onCordonOnlyRisingEdge(ctx, node.Name); err != nil {
			klog.Errorf("nodepoller: failed to create NodeCriticalIssueDisappeared alert for node %s: %v", node.Name, err)
		}
	} else if !hasTaint && wasPresent {
		klog.V(2).Infof("nodepoller: nodecheck taint removed from node %s (falling edge)", node.Name)
		seenTaints[node.Name] = false

		p.cacheLock.Lock()
		delete(p.nodecheckCache, node.Name)
		p.cacheLock.Unlock()
		klog.V(4).Infof("nodepoller: removed node %s from nodecheckCache", node.Name)
	}
}
