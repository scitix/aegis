package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gitlab.scitix-inner.ai/k8s/aegis/api"
	_ "gitlab.scitix-inner.ai/k8s/aegis/api/apis"
	"gitlab.scitix-inner.ai/k8s/aegis/internal/controller"
	"gitlab.scitix-inner.ai/k8s/aegis/internal/k8s"
	"gitlab.scitix-inner.ai/k8s/aegis/tools"
	"gitlab.scitix-inner.ai/k8s/aegis/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	cfgFile        string
	apiserverHost  string
	kubeConfigFile string
	routePrefix    string
	gracePeriod    int
	port           int
)

func main() {
	klog.InitFlags(nil)

	fmt.Println(version.String())

	showVersion, conf, err := parse()
	if showVersion {
		os.Exit(0)
	}

	if err != nil {
		klog.Fatal(err)
	}

	initConfig()

	cfg, kubeClient, err := k8s.CreateApiserverClient(apiserverHost, kubeConfigFile)
	if err != nil {
		handleFatalInitError(err)
	}
	conf.Client = kubeClient
	conf.Config = cfg

	// must run as pod
	if conf.EnableLeaderElection {
		err = k8s.InitAegisPodInfo(kubeClient)
		if err != nil {
			klog.Fatalf("Unexpected error obtaining aegis pod: %v", err)
		}
	}

	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), conf.PublishNamespace, metav1.GetOptions{})
	if err != nil {
		klog.Fatalf("Failed to get publish namespace(%s): %v", conf.PublishNamespace, err)
	}

	aegisController, err := controller.NewAegisController(conf)
	if err != nil {
		klog.Fatalf("Failed to create Aegis Controller: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go tools.HandlerSigterm(cancel, gracePeriod, func(code int) {
		os.Exit(code)
	})

	// run api
	if port > 0 {
		go api.RunHttpServer(strconv.Itoa(port), routePrefix, aegisController.CreateOrUpdateAlert)
	}

	// run controller
	if err := aegisController.Run(ctx); err != nil {
		klog.Fatalf("Run Aegis Controller error: %v", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.Set("kubeconfig", kubeConfigFile)

	viper.AutomaticEnv() // read in environment variables that match

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err == nil {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		} else {
			klog.Fatalf("Error reading config file: %v", err)
		}
	}
}

func parse() (bool, *controller.Configuration, error) {
	flags := pflag.NewFlagSet("", pflag.ExitOnError)

	flag.StringVar(&cfgFile, "config", "", "Path to the configuration file.")
	flags.StringVar(&apiserverHost, "apiserver-host", "", "Address of the Kubernetes API server.")
	flags.StringVar(&kubeConfigFile, "kubeconfig", "", "Path to the kubeconfig file.")
	flags.IntVar(&port, "http-port", 80, "Port to use for http server")
	flags.StringVar(&routePrefix, "web.route-prefix", "/", "Prefix for API and UI endpoints")
	flags.IntVar(&gracePeriod, "grace-period", 5, "Graceful shutdown period")

	resyncPeriod := flags.Duration("sync-period", 30*time.Duration(time.Minute), "Period at which the controller forces the local object store.")
	workers := flags.Int("workers", 2, "Workers for workqueue.")
	enableLeaderElection := flags.Bool("enable-leader-election", false, "Enable leader elelction in a kubernetes cluster.")
	electionID := flags.String("election-id", "aegis-controller", "Election id to use for Aegis status update.")

	enableAlert := flags.Bool("alert.enable", true, "Enable alert operation")
	publishNamespace := flags.String("alert.publish-namespace", "default", "Publish alert to the target namespace")
	systemParas := flags.String("alert.system-parameters", "", "system parameters")
	ttlAfterSucceed := flags.Int32("alert.ttl-after-succeed", 2*24*60*60, "clean ttl after alert ops succeed")
	ttlAfterFailed := flags.Int32("alert.ttl-after-failed", 4*24*60*60, "clean ttl after alert ops failed")
	ttlAfterNoOps := flags.Int32("alert.ttl-after-noops", 1*24*60*60, "clean ttl after alert ops failed")

	enableHealthcheck := flags.Bool("healthcheck.enable", true, "enable cluster/node healthcheck")
	enableFireNodeEvent := flags.Bool("nodecheck.fireevent", false, "enable fire node event if check status true")

	enableDiagnosis := flags.Bool("diagnosis.enable", true, "Enable diagnosis")
	explain := flags.Bool("diagnosis.explain", false, "enable LLM based explaination")
	cache := flags.Bool("diagnosis.cache", true, "enable cached data")
	language := flags.String("diagnosis.language", "chinese", "explain language, support chinese/english")
	collectorImage := flags.String("diagnosis.collectorImage", "registry-ap-southeast.scitix.ai/k8s/collector:v1.0.0", "Container image of the Collector Pod")
	enableProm := flags.Bool("diagnosis.enablePromtheus", false, "Whether use the promethus to get events")

	ai := flags.String("ai", "openai", "backend AI Provider")

	showVersion := flags.Bool("version", false, "Show release info.")

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	flag.CommandLine.Parse([]string{})

	pflag.VisitAll(func(flag *pflag.Flag) {
		klog.V(2).InfoS("FLAG", flag.Name, flag.Value)
	})

	if *showVersion {
		return true, nil, nil
	}

	var para map[string]string
	if len(*systemParas) > 0 {
		para = make(map[string]string)
		ps := strings.Split(*systemParas, ";")
		for _, p := range ps {
			kv := strings.Split(p, ":")
			if len(kv) != 2 {
				return false, nil, fmt.Errorf("Unexpected system param format: %s", p)
			}
			para[kv[0]] = kv[1]
		}
	}

	config := &controller.Configuration{
		PublishNamespace:          *publishNamespace,
		SystemParas:               para,
		ResyncPeriod:              *resyncPeriod,
		SyncWorkers:               *workers,
		EnableLeaderElection:      *enableLeaderElection,
		ElectionID:                *electionID,
		EnableAlert:               *enableAlert,
		DefaultTTLAfterOpsSucceed: *ttlAfterSucceed,
		DefaultTTLAfterOpsFailed:  *ttlAfterFailed,
		DefaultTTLAfterNoOps:      *ttlAfterNoOps,
		EnableHealthcheck:         *enableHealthcheck,
		EnableFireNodeEvent:       *enableFireNodeEvent,
		EnableDiagnosis:           *enableDiagnosis,
		DiagnosisEnableExplain:    *explain,
		DiagnosisEnableCache:      *cache,
		DiagnosisLanguage:         *language,
		CollectorImage:            *collectorImage,
		EnableProm:                *enableProm,
		AiBackend:                 *ai,
	}

	return false, config, nil
}

func handleFatalInitError(err error) {
	klog.Fatalf("Error while initiating a connecton to the Kubernetes API server", err)
}
