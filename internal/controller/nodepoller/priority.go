package nodepoller

import (
	"context"
	"sync"

	"github.com/scitix/aegis/internal/selfhealing/analysis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// PriorityWatcher watches the aegis-priority ConfigMap and hot-reloads priority config.
type PriorityWatcher struct {
	configs map[string]analysis.ConditionConfig
	mu      sync.RWMutex
}

// NewPriorityWatcher creates a PriorityWatcher. The instance should be created
// once in the top-level controller and injected into both NodeStatusPoller and
// DeviceAwareController so they share a single hot-reloaded config.
func NewPriorityWatcher() *PriorityWatcher {
	return &PriorityWatcher{
		configs: make(map[string]analysis.ConditionConfig),
	}
}

// IsCritical returns true if the condition has priority in [0, Emergency].
func (w *PriorityWatcher) IsCritical(condition string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.configs[condition]
	if !ok {
		return false // unknown condition: conservative, do not trigger
	}
	return c.Priority <= analysis.Emergency
}

// IsCordon returns true if the condition is exactly NodeCordon priority.
func (w *PriorityWatcher) IsCordon(condition string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.configs[condition]
	return ok && c.Priority == analysis.NodeCordon
}

// IsLoadAffecting returns true if the condition is configured to affect node load.
func (w *PriorityWatcher) IsLoadAffecting(condition string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.configs[condition]
	return ok && c.AffectsLoad
}

// GetIDMode returns the DeviceIDMode for the condition ("all"/"index"/"mask"/"id"/"-").
// Returns "-" for unknown conditions (conservative: do not mark any device).
func (w *PriorityWatcher) GetIDMode(condition string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.configs[condition]
	if !ok {
		return "-"
	}
	return c.DeviceIDMode
}

func (w *PriorityWatcher) reload(data map[string]string) {
	const key = "priority"
	content, ok := data[key]
	if !ok {
		klog.Warningf("nodepoller: priority ConfigMap has no key %q, skipping reload", key)
		return
	}

	parsed, err := analysis.ParseConditionConfig(content)
	if err != nil {
		klog.Errorf("nodepoller: failed to parse priority config: %v", err)
		return
	}

	w.mu.Lock()
	w.configs = parsed
	w.mu.Unlock()
	klog.V(4).Infof("nodepoller: priority config reloaded (%d entries)", len(parsed))
}

// RunConfigMapWatcher starts watching the priority ConfigMap and reloads on change.
func (w *PriorityWatcher) RunConfigMapWatcher(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", name).String()
		}),
	)

	cmInformer := factory.Core().V1().ConfigMaps().Informer()

	cmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm, ok := obj.(*corev1.ConfigMap)
			if !ok || cm.Name != name {
				return
			}
			klog.V(4).Infof("nodepoller: priority ConfigMap added, reloading")
			w.reload(cm.Data)
		},
		UpdateFunc: func(_, newObj interface{}) {
			cm, ok := newObj.(*corev1.ConfigMap)
			if !ok || cm.Name != name {
				return
			}
			klog.V(4).Infof("nodepoller: priority ConfigMap updated, reloading")
			w.reload(cm.Data)
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
}
