package install

import (
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/clustercheck/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func Install(schme *runtime.Scheme) {
	utilruntime.Must(v1alpha1.AddToScheme(schme))
	utilruntime.Must(schme.SetVersionPriority(v1alpha1.SchemeGroupVersion))
}
