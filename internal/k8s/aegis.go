package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gitlab.scitix-inner.ai/k8s/aegis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	discovery "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	AegisPodDetail *PodInfo
)

type PodInfo struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func InitAegisPodInfo(kubeClient clientset.Interface) error {
	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAMESPACE")

	if len(podNamespace) == 0 || len(podNamespace) == 0 {
		return fmt.Errorf("unable to get Pod information (missing POD_NAME or POD_NAMESPACE environment variable)")
	}

	pod, err := kubeClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get POD information: %v", err)
	}

	AegisPodDetail = &PodInfo{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
	}

	pod.ObjectMeta.DeepCopyInto(&AegisPodDetail.ObjectMeta)
	AegisPodDetail.SetLabels(pod.GetLabels())

	return nil
}

func CreateApiserverClient(apiserverHost, kubeConfig string) (*rest.Config, *kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(apiserverHost, kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	cfg.UserAgent = fmt.Sprintf(
		"%s%s (%s%s) aegis%s",
		filepath.Base(os.Args[0]),
		version.RELEASE,
		runtime.GOOS,
		runtime.GOARCH,
		version.COMMIT,
	)

	klog.V(2).InfoS("Creating API client", "host", cfg.Host)

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	var v *discovery.Info
	defaultRetry := wait.Backoff{
		Steps:    10,
		Duration: 1 * time.Second,
		Factor:   1.5,
		Jitter:   0.1,
	}

	var lastErr error
	retries := 0
	klog.V(2).InfoS("Trying to discover Kubernetes version")
	err = wait.ExponentialBackoff(defaultRetry, func() (bool, error) {
		v, err = client.DiscoveryClient.ServerVersion()

		if err == nil {
			return true, nil
		}

		lastErr = err
		klog.V(2).ErrorS(err, "Unexpected error discovering Kubernetes version", "attempt", retries)
		retries++
		return false, nil
	})

	if err != nil {
		return nil, nil, lastErr
	}

	if retries > 0 {
		klog.Warningf("Initial connection to the Kubernetes API server was retried %d times", retries)
	}

	klog.V(2).InfoS("Running in Kubernetes cluster",
		"major", v.Major,
		"minor", v.Minor,
		"git", v.GitVersion,
		"state", v.GitTreeState,
		"commit", v.GitCommit,
		"platform", v.Platform,
	)

	return cfg, client, nil
}
