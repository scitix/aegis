package controller

import (
	"context"
	"os"
	"time"

	"gitlab.scitix-inner.ai/k8s/aegis/internal/k8s"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/generated/alert/clientset/versioned/scheme"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type leaderElectionConfig struct {
	Client clientset.Interface

	ElectionID string

	OnStartedLeading func(chan struct{})
	OnStoppedLeading func()
}

func setupLeaderElection(config *leaderElectionConfig) {
	var elector *leaderelection.LeaderElector

	// start a new context
	ctx := context.Background()

	var cancelContext context.CancelFunc

	var newLeaderCtx = func(ctx context.Context) context.CancelFunc {
		leaderCtx, cancel := context.WithCancel(ctx)
		go elector.Run(leaderCtx)
		return cancel
	}

	var stopCh chan struct{}
	callbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			klog.V(2).InfoS("I am a new leader")
			stopCh = make(chan struct{})

			if config.OnStartedLeading != nil {
				config.OnStartedLeading(stopCh)
			}
		},
		OnStoppedLeading: func() {
			klog.V(2).InfoS("I am not leader anymore")
			close(stopCh)

			cancelContext()

			cancelContext = newLeaderCtx(ctx)

			if config.OnStoppedLeading != nil {
				config.OnStoppedLeading()
			}
		},
		OnNewLeader: func(identity string) {
			klog.InfoS("New leader elected", "identity", identity)
		},
	}

	broadcaster := record.NewBroadcaster()
	hostname, _ := os.Hostname()

	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{
		Component: "aegis-leader-election",
		Host:      hostname,
	})
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Namespace: k8s.AegisPodDetail.Namespace,
			Name:      config.ElectionID,
		},
		Client: config.Client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      k8s.AegisPodDetail.Name,
			EventRecorder: recorder,
		},
	}

	ttl := 30 * time.Second
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: ttl,
		RenewDeadline: ttl / 2,
		RetryPeriod:   ttl / 4,
		Callbacks:     callbacks,
	})
	if err != nil {
		klog.Fatalf("unexpected error starting leader election: %v", err)
	}

	cancelContext = newLeaderCtx(ctx)
}
