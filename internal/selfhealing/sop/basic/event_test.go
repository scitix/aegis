package basic

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/aegis/internal/k8s"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/scheme"
)

func TestFireEvent(t *testing.T) {
	_, client, err := k8s.CreateApiserverClient("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		t.Errorf(err.Error())
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "Aegis"})

	bridge := &sop.ApiBridge{
		KubeClient:    client,
		EventRecorder: recorder,
	}

	err = FireEventForNodePod(context.Background(), bridge, "cygnus170", EventReasonPotentialInfluence, "gpfs break deadlock")
	if err != nil {
		t.Errorf(err.Error())
	}
}
