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
	config map[string]analysis.Priority
	mu     sync.RWMutex
}

func newPriorityWatcher() *PriorityWatcher {
	return &PriorityWatcher{
		config: make(map[string]analysis.Priority),
	}
}

// IsCritical returns true if the condition has priority in [0, Emergency].
func (w *PriorityWatcher) IsCritical(condition string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p, ok := w.config[condition]
	if !ok {
		return false // unknown condition: conservative, do not trigger
	}
	return p <= analysis.Emergency // priority 0~99
}

// IsCordon returns true if the condition is exactly NodeCordon priority.
func (w *PriorityWatcher) IsCordon(condition string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p, ok := w.config[condition]
	return ok && p == analysis.NodeCordon
}

func (w *PriorityWatcher) reload(data map[string]string) {
	const key = "priority"
	content, ok := data[key]
	if !ok {
		klog.Warningf("nodepoller: priority ConfigMap has no key %q, skipping reload", key)
		return
	}

	parsed, err := analysis.ParsePriorityConfig(content)
	if err != nil {
		klog.Errorf("nodepoller: failed to parse priority config: %v", err)
		return
	}

	w.mu.Lock()
	w.config = parsed
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
