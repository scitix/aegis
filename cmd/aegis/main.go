package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/scitix/aegis/api"
	"github.com/scitix/aegis/api/apis"
	"github.com/scitix/aegis/internal/controller"
	"github.com/scitix/aegis/internal/controller/nodepoller"
	"github.com/scitix/aegis/internal/k8s"
	"github.com/scitix/aegis/pkg/ai"
	"github.com/scitix/aegis/pkg/metrics"
	"github.com/scitix/aegis/tools"
	"github.com/scitix/aegis/version"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	metricsController := metrics.NewMetricsController()
	if port > 0 {
		go api.RunHttpServer(strconv.Itoa(port), routePrefix, aegisController.CreateOrUpdateAlert, metricsController)
	}

	// AIClient for parser
	if conf.AiBackend != "" {
		providerFactory := &ai.DefaultFactory{}
		client, _, err := providerFactory.Load(conf.AiBackend, nil)
		if err != nil {
			klog.Fatalf("Failed to load AI client: %v", err)
		}
		apis.SetAIAlertParser(ai.NewDefaultAIAlertParserWithClient(client))
	} else {
		klog.Infof("AI backend not configured, skip injecting AIAlertParser")
	}

	// run controller
	if err := aegisController.Run(ctx); err != nil {
		klog.Fatalf("Run Aegis Controller error: %v", err)
	}
}

// initConfig reads the config file (if set) and binds all pflags to viper so
// that config-file values fill in any flag that was not explicitly set on the
// CLI.  Priority: CLI flag > env var > config file > flag default.
func initConfig(flags *pflag.FlagSet) {
	viper.Set("kubeconfig", kubeConfigFile)
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err == nil {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		} else {
			klog.Fatalf("Error reading config file: %v", err)
		}
	}

	// Bind every pflag so config-file keys override the flag defaults.
	flags.VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})
}

func parse() (bool, *controller.Configuration, error) {
	flags := pflag.NewFlagSet("", pflag.ExitOnError)

	flag.StringVar(&cfgFile, "config", "", "Path to the configuration file.")
	flags.StringVar(&apiserverHost, "apiserver-host", "", "Address of the Kubernetes API server.")
	flags.StringVar(&kubeConfigFile, "kubeconfig", "", "Path to the kubeconfig file.")
	flags.IntVar(&port, "http-port", 80, "Port to use for http server")
	flags.StringVar(&routePrefix, "web.route-prefix", "/", "Prefix for API and UI endpoints")
	flags.IntVar(&gracePeriod, "grace-period", 5, "Graceful shutdown period")

	flags.Duration("sync-period", 30*time.Minute, "Period at which the controller forces the local object store.")
	flags.Int("workers", 2, "Workers for workqueue.")
	flags.Bool("enable-leader-election", false, "Enable leader election in a kubernetes cluster.")
	flags.String("election-id", "aegis-controller", "Election id to use for Aegis status update.")

	flags.Bool("alert.enable", true, "Enable alert operation")
	flags.String("alert.publish-namespace", "default", "Publish alert to the target namespace")
	flags.Int32("alert.ttl-after-succeed", 2*24*60*60, "clean ttl after alert ops succeed")
	flags.Int32("alert.ttl-after-failed", 4*24*60*60, "clean ttl after alert ops failed")
	flags.Int32("alert.ttl-after-noops", 1*24*60*60, "clean ttl after alert ops no-ops")

	// prometheus flags stay on stdlib flag so existing env-var / helm overrides work
	promEndpoint := flag.String("prometheus.endpoint", "", "Prometheus server endpoint, e.g. http://localhost:9090")
	promToken := flag.String("prometheus.token", "", "Prometheus API access token")

	flags.Bool("healthcheck.enable", true, "enable cluster/node healthcheck")
	flags.Bool("nodecheck.fireevent", false, "enable fire node event if check status true")

	flags.Bool("diagnosis.enable", true, "Enable diagnosis")
	flags.Bool("diagnosis.explain", false, "enable LLM based explanation")
	flags.Bool("diagnosis.cache", true, "enable cached data")
	flags.String("diagnosis.language", "chinese", "explain language, support chinese/english")
	flags.String("diagnosis.collector-image", "registry-ap-southeast.scitix.ai/k8s/aegis-collector:v1.0.0", "Container image of the Collector Pod")
	flags.Bool("diagnosis.enablePrometheus", true, "Whether use the prometheus to get events")

	flags.String("ai.provider", "openai", "backend AI provider name")

	flags.Bool("device-aware.enable", false, "enable device aware")

	flags.Bool("node-poller.enable", false, "enable node active polling")
	flags.Duration("node-poller.poll-interval", 0, "how often to query Prometheus (default 10s)")
	flags.Duration("node-poller.resync-interval", 0, "how often to resync critical nodes (default 1h)")
	flags.Duration("node-poller.cordon-resync-interval", 0, "how often to resync cordon-only nodes (default 10min)")
	flags.Int("node-poller.max-alerts-per-round", 0, "max alerts per poll round (default 20)")
	flags.String("node-poller.priority-configmap", "", "priority ConfigMap name (default \"aegis-priority\")")
	flags.String("node-poller.priority-namespace", "", "priority ConfigMap namespace (default \"monitoring\")")

	flags.Bool("version", false, "Show release info.")

	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)
	flag.CommandLine.Parse([]string{})

	pflag.VisitAll(func(f *pflag.Flag) {
		klog.V(2).InfoS("FLAG", f.Name, f.Value)
	})

	if v, _ := flags.GetBool("version"); v {
		return true, nil, nil
	}

	// Read config file and bind all flags to viper before reading values.
	initConfig(flags)

	// system-parameters: prefer config-file map; CLI not supported for this key
	// (operators set it directly in config.yaml as a YAML map).
	para := viper.GetStringMapString("alert.system-parameters")

	config := &controller.Configuration{
		PublishNamespace:          viper.GetString("alert.publish-namespace"),
		SystemParas:               para,
		ResyncPeriod:              viper.GetDuration("sync-period"),
		SyncWorkers:               viper.GetInt("workers"),
		EnableLeaderElection:      viper.GetBool("enable-leader-election"),
		ElectionID:                viper.GetString("election-id"),
		EnableAlert:               viper.GetBool("alert.enable"),
		DefaultTTLAfterOpsSucceed: viper.GetInt32("alert.ttl-after-succeed"),
		DefaultTTLAfterOpsFailed:  viper.GetInt32("alert.ttl-after-failed"),
		DefaultTTLAfterNoOps:      viper.GetInt32("alert.ttl-after-noops"),
		PromEndpoint:              *promEndpoint,
		PromToken:                 *promToken,
		EnableHealthcheck:         viper.GetBool("healthcheck.enable"),
		EnableFireNodeEvent:       viper.GetBool("nodecheck.fireevent"),
		EnableDiagnosis:           viper.GetBool("diagnosis.enable"),
		DiagnosisEnableExplain:    viper.GetBool("diagnosis.explain"),
		DiagnosisEnableCache:      viper.GetBool("diagnosis.cache"),
		DiagnosisLanguage:         viper.GetString("diagnosis.language"),
		CollectorImage:            viper.GetString("diagnosis.collector-image"),
		EnableProm:                viper.GetBool("diagnosis.enablePrometheus"),
		AiBackend:                 viper.GetString("ai.provider"),
		EnableDeviceAware:         viper.GetBool("device-aware.enable"),
		EnableNodePoller:          viper.GetBool("node-poller.enable"),
		NodePoller: nodepoller.PollerConfig{
			PollInterval:         viper.GetDuration("node-poller.poll-interval"),
			ResyncInterval:       viper.GetDuration("node-poller.resync-interval"),
			CordonResyncInterval: viper.GetDuration("node-poller.cordon-resync-interval"),
			MaxAlertsPerRound:    viper.GetInt("node-poller.max-alerts-per-round"),
			PriorityConfigMap:    viper.GetString("node-poller.priority-configmap"),
			PriorityNamespace:    viper.GetString("node-poller.priority-namespace"),
		},
	}

	return false, config, nil
}

func handleFatalInitError(err error) {
	klog.Fatalf("Error while initiating a connection to the Kubernetes API server: %v", err)
}
