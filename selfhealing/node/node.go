package node

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/aegis/internal/selfhealing"
	"github.com/scitix/aegis/internal/selfhealing/analysis"
	nodesop "github.com/scitix/aegis/internal/selfhealing/node_sop"
	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/internal/selfhealing/sop/basic"
	"github.com/scitix/aegis/pkg/prom"
	"github.com/scitix/aegis/selfhealing/config"
	"github.com/scitix/aegis/tools"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"

	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/baseboard"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/cpu"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/disk"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/gpfs"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/gpu"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/ib"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/memory"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/network"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/node"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/roce"
	_ "github.com/scitix/aegis/internal/selfhealing/node_sop/system"
	selfticket "github.com/scitix/aegis/internal/selfhealing/ticket"
	"github.com/scitix/aegis/pkg/ticketmodel"

	"github.com/scitix/aegis/internal/selfhealing/node_sop/gatekeeper"
)

var (
	AlertType = os.Getenv("AlertType")
	Alert     = os.Getenv("Alert")
	Object    = os.Getenv("Object")
)

func NewCommand(config *config.SelfHealingConfig, use string) *cobra.Command {
	o := &nodeOptions{
		config: config,
	}

	c := &cobra.Command{
		Use:   use + " Name",
		Short: "self-healing node",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.complete(cmd, args); err != nil {
				klog.Fatalf("Invalid node selfhealing startup option: %s", err)
			}

			if err := o.validate(); err != nil {
				klog.Fatalf("Invalid node selfhealing startup option: %s", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			go tools.HandlerSigterm(cancel, 10, func(code int) {
				os.Exit(code)
			})

			// just return
			if !o.precheck(ctx) {
				os.Exit(0)
			}

			if err := o.catchLockAndRun(ctx); err != nil {
				klog.Fatalf("Selfhealing failed: %v", err)
			}
		},
		Example: `selfhealing node scorp123`,
	}

	c.PersistentFlags().StringVar(&o.tpe, "type", "", "node issue type.")
	c.PersistentFlags().StringVar(&o.priorityConfig, "priority-config", "/selfhealing/config/priority.conf", "node status priority config")

	c.PersistentFlags().IntVar(&o.level, "level", 0, "node issue selfhealing level")
	c.PersistentFlags().BoolVar(&o.onlyTicket, "ticket.only", false, "only create ticket record for issue, no operation actions")
	c.PersistentFlags().StringVar(&o.ticketSystem, "ticket.system", "Node", "ticket system for record issue")
	c.PersistentFlags().BoolVar(&o.claimTicket, "ticket.claim", false, "allow to claim ticket for handling the issue")
	c.PersistentFlags().StringVar(&o.promEndpoint, "prometheus.endpoint", "", "Prometheus server endpoint, e.g. http://localhost:9090")
	c.PersistentFlags().StringVar(&o.promToken, "prometheus.token", "", "Prometheus API access token")
	c.PersistentFlags().StringVar(&o.opsImage, "ops.image", "", "selfhealing ops image")
	return c
}

type TicketSystem string

const (
	// use node annotation for record issue
	TicketSystemNode TicketSystem = "Node"

	// use scitix ticket system for record
	TicketSystemScitix TicketSystem = "Scitix"
)

type nodeOptions struct {
	name           string
	ip             string
	tpe            string
	namespace      string
	podName        string
	priorityConfig string
	level          int

	onlyTicket   bool
	ticketSystem string
	claimTicket  bool

	promEndpoint string
	promToken    string

	opsImage string

	node   *v1.Node
	pod    *v1.Pod
	bridge *sop.ApiBridge

	gatekeeper *gatekeeper.GateKeeper

	promApi *prom.PromAPI
	config  *config.SelfHealingConfig
}

func (o *nodeOptions) complete(cmd *cobra.Command, args []string) (err error) {
	o.podName = os.Getenv("POD_NAME")
	o.namespace = os.Getenv("POD_NAMESPACE")
	if len(o.namespace) == 0 || len(o.podName) == 0 {
		return fmt.Errorf("unable to get Pod information (missing POD_NAME or POD_NAMESPACE environment variable)")
	}

	o.pod, err = o.config.KubeClient.CoreV1().Pods(o.namespace).Get(context.Background(), o.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("fail to get pod object: %s", err)
	}

	argsLen := cmd.ArgsLenAtDash()
	if argsLen == -1 {
		argsLen = len(args)
	}

	if argsLen != 1 {
		return fmt.Errorf("exactly one Name is required, got: %d", argsLen)
	}

	o.name = args[0]
	if strings.Contains(o.name, ".") {
		o.name = strings.Split(o.name, ".")[0]
	}

	// nodeStatus := &sop.NodeStatus{}
	o.node, err = o.config.KubeClient.CoreV1().Nodes().Get(context.Background(), o.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("fail to get node object: %s", err)
	} else {
		o.ip = o.node.Status.Addresses[0].Address
	}

	o.promApi = prom.CreatePromClient(o.promEndpoint, o.promToken)

	system := selfticket.TicketSystem(o.ticketSystem)

	managerArgs := &ticketmodel.TicketManagerArgs{
		Client:      o.config.KubeClient,
		Node:        o.node,
		Region:      o.config.Region,
		ClusterName: o.config.ClusterName,
		OrgName:     o.config.OrgName,
		NodeName:    o.name,
		Ip:          o.ip,
		User:        config.TicketSupervisorAegis,
	}

	ticketManager, err := selfticket.NewTicketManagerBySystem(context.Background(), system, managerArgs)
	if err != nil {
		return fmt.Errorf("fail to create ticket manager: %s", err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: o.config.KubeClient.CoreV1().Events(v1.NamespaceAll)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "Aegis"})

	var alertName string
	if strings.Contains(Alert, "/") {
		parts := strings.Split(Alert, "/")
		if len(parts) > 1 {
			alertName = parts[1]
		} else {
			alertName = "" // Default value if split result is invalid
		}
	} else {
		alertName = "" // Default value if Alert does not contain "/"
	}

	o.bridge = &sop.ApiBridge{
		ClusterName:     o.config.ClusterName,
		Region:          o.config.Region,
		AlertName:       alertName,
		Aggressive:      o.level > 0,
		AggressiveLevel: o.level,
		Registry:        o.config.Registry,
		Repository:      o.config.Repository,
		OpsImage:        o.opsImage,
		Owner: metav1.NewControllerRef(o.pod, schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		}),
		KubeClient:    o.config.KubeClient,
		PromClient:    o.promApi,
		TicketManager: ticketManager,
		EventRecorder: recorder,
	}

	gatekeeper, err := gatekeeper.CreateGateKeeper(context.Background(), o.bridge)
	if err != nil {
		return fmt.Errorf("fail to create gatekeeper: %s", err)
	}
	o.gatekeeper = gatekeeper

	return err
}

func (o *nodeOptions) validate() error {
	return nil
}

const (
	queueCMName = "aegis-selfhealing-leader-election-waiting-list"
)

func (o *nodeOptions) shouldCompete(ctx context.Context, max int) (bool, error) {
	cm, err := o.config.KubeClient.CoreV1().ConfigMaps(o.namespace).Get(ctx, queueCMName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			cm = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: queueCMName,
				},
				Data: map[string]string{},
			}
			_, err = o.config.KubeClient.CoreV1().ConfigMaps(o.namespace).Create(ctx, cm, metav1.CreateOptions{})
			if err != nil {
				return false, err
			}
			cm.Data = map[string]string{}
		} else {
			return false, err
		}
	}

	data := cm.Data
	if data == nil {
		data = make(map[string]string)
	}

	waiting := cm.Data[o.name]
	cnt, err := strconv.Atoi(waiting)
	if err != nil {
		return false, err
	}

	if cnt >= max {
		return false, nil
	}

	cm.Data[o.name] = strconv.Itoa(cnt + 1)
	cm.Data = data

	_, err = o.config.KubeClient.CoreV1().ConfigMaps(o.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return true, err
}

func (o *nodeOptions) catchLockAndRun(ctx context.Context) error {
	// ok, err := o.shouldCompete(ctx, 2)
	// if err != nil {
	// 	return fmt.Errorf("Failed to determine competition: %v", err)
	// }

	// if !ok {
	// 	klog.Infof("Too many competitors, exiting.")
	// 	return nil
	// }
	var err error
	ctx, cancel := context.WithCancel(ctx)
	if o.config.EnableLeaderElection {
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      o.name,
				Namespace: o.namespace,
			},
			Client: o.config.KubeClient.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: o.podName,
			},
		}

		ttl := 30 * time.Second
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   ttl,
			RenewDeadline:   ttl / 2,
			RetryPeriod:     ttl / 4,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					klog.InfoS("I am a new leader")
					err = o.run(ctx)
					cancel()
				},
				OnStoppedLeading: func() {
					klog.InfoS("I am not leader anymore")
				},
				OnNewLeader: func(identity string) {
					klog.Infof("Not the leader, Current leader: %s\n", identity)
				},
			},
			Name: o.name,
		})
	} else {
		err = o.run(ctx)
		cancel()
	}

	<-ctx.Done()

	return err
}

func (o *nodeOptions) run(ctx context.Context) error {
	o.bridge.TicketManager.Reset(ctx)

	// if disable selfhealing, dispatch ticket to sre.
	if o.isDiableSelfHealing(ctx) {
		klog.Infof("node has disabled selfhealing, will give up")
		err := o.bridge.TicketManager.DispatchTicketToSRE(ctx)
		if err == ticketmodel.TicketNotFoundErr {
			return nil
		} else if err != nil {
			klog.Errorf("error dispatch ticket to sre: %s", err)
		} else {
			o.bridge.TicketManager.AddConclusion(ctx, "aegis cannot deal with disabled selfhealing node, so dispatch to sre.")
		}
		return nil
	}

	nodestatus, err := o.promApi.GetNodeStatuses(ctx, o.name, o.tpe)
	if err != nil {
		return fmt.Errorf("Error get node status: %s", err)
	}
	klog.V(4).Infof("Node %s status: %v", o.name, nodestatus)

	err = analysis.InitAnalysisConfig(o.priorityConfig)
	if err != nil {
		return fmt.Errorf("Error init priority config: %s", err)
	}

	result := analysis.AnalysisNodeStatus(nodestatus)
	klog.V(4).Infof("Analysis result: %+v", result)

	status := o.findProperOne(result)
	if status == nil {
		klog.Infof("No issue to deal with, exit.")
		return nil
	}

	if !o.bridge.TicketManager.CanDealWithTicket(ctx) {
		existingCondition := o.bridge.TicketManager.GetTicketCondition(ctx)
		existingSOP, err := nodesop.GetSOP(existingCondition)
		canPreempt := err == nil
		if canPreempt {
			if ps, ok := existingSOP.(nodesop.PreemptableSOP); ok {
				canPreempt = ps.IsPreemptable()
			} else {
				canPreempt = false
			}
		}

		// Do not preempt if the incoming condition is also preemptable (same tier).
		// Preemption is only allowed when the new condition is non-preemptable (i.e. higher severity).
		if canPreempt {
			newSOP, err := nodesop.GetSOP(status.Condition)
			if err == nil {
				if ps, ok := newSOP.(nodesop.PreemptableSOP); ok && ps.IsPreemptable() {
					canPreempt = false
					klog.Infof("both existing condition %q and new condition %q are preemptable (same tier), skip preemption", existingCondition, status.Condition)
				}
			}
		}

		if canPreempt {
			klog.Infof("existing ticket condition %q is preemptable, deleting ticket and proceeding", existingCondition)
			if err := o.bridge.TicketManager.DeleteTicket(ctx); err != nil {
				klog.Warningf("failed to delete preemptable ticket: %s", err)
			}
		} else if !o.claimTicket {
			klog.Warningf("cannot deal with ticket (condition: %q), give up", existingCondition)
			return nil
		} else {
			klog.Warning("aegis try to claim ticket.")
			if err := o.bridge.TicketManager.AdoptTicket(ctx); err != nil {
				return fmt.Errorf("aegis failed to claim ticket: %s", err)
			}
			klog.Infof("aegis succeed claim ticket.")
		}
	}

	klog.Infof("Start to deal with issue: %v", status)
	sop, err := nodesop.GetSOP(status.Condition)
	if err != nil {
		klog.Warningf("No sop found, give up.")
		return nil
	}

	if err := sop.CreateInstance(ctx, o.bridge); err != nil {
		return fmt.Errorf("Error create sop instance: %s", err)
	}

	if !sop.Evaluate(ctx, o.name, status) {
		klog.Warningf("Evaluate sop failed, give up.")
		return nil
	}

	// gatekeeper: skip if SOP does not need to cordon the node
	needCordon := true
	if cs, ok := sop.(nodesop.CordonSOP); ok {
		needCordon = cs.NeedCordon(ctx, o.name, status)
	}
	if needCordon && status.Condition != selfhealing.NodeCordonCondition {
		pass, reason := o.gatekeeper.Pass(ctx)
		if !pass {
			klog.Warningf("GateKeeper refuse workflow, reason: %s", reason)
			return nil
		}
	}

	if basic.CheckNodeIsMaster(ctx, o.bridge, o.name) {
		return nil
	}

	err = sop.Execute(ctx, o.name, status)
	if err != nil {
		klog.Errorf("Error execute sop: %s", err)
		return fmt.Errorf("Error execute sop: %s", err)
	}

	return nil
}

// find proper issue
// fisrt: Node not ready
// second: Node cordon with no emergency issues
// third: emergency issues
func (o *nodeOptions) findProperOne(result *analysis.NodeStatusAnalysisResult) *prom.AegisNodeStatus {
	var status prom.AegisNodeStatus
	var find bool
	// debug info
	klog.V(4).Infof("Node %s issue list:", o.name)
	if result.NotReady != nil {
		if !find {
			find = true
			status = *result.NotReady
		}
		klog.V(4).Infof("Node NotReady issues: %v", result.NotReady)
	}

	if result.Cordon != nil && len(result.EmergencyList) == 0 {
		if !find {
			find = true
			status = *result.Cordon
		}
		klog.V(4).Infof("Node Cordon issues: %v", result.Cordon)
	}

	if len(result.EmergencyList) > 0 {
		if !find {
			find = true
			status = result.EmergencyList[0]
		}
		klog.V(4).Infof("Emergency issues: %v", result.EmergencyList)
	}

	if len(result.CanIgnoreList) > 0 {
		klog.V(4).Infof("CanIgnore issues: %v", result.CanIgnoreList)
	}
	if len(result.MustIgnoreList) > 0 {
		klog.V(4).Infof("MustIgnore issues: %v", result.MustIgnoreList)
	}

	if find {
		return &status
	}

	return nil
}

func (o *nodeOptions) precheck(ctx context.Context) bool {
	// 检查是否有指定的污点，如果有则移除并将节点置为 cordon 状态
	if o.hasSpecifiedTaint(ctx) {
		klog.Infof("Node %s has specified taint, removing taint and cordoning node", o.name)
		if err := o.removeTaintAndCordon(ctx); err != nil {
			klog.Errorf("Failed to remove taint and cordon node %s: %v", o.name, err)
			return false
		}
		klog.Infof("Successfully removed taint and cordoned node %s", o.name)
		return false // 不继续执行后续的自愈逻辑
	}
	return true
}

func (o *nodeOptions) isDiableSelfHealing(ctx context.Context) bool {
	for key, value := range o.node.Labels {
		if key == tools.AEGIS_DISABLE_LABEL_KEY && value == tools.AEGIS_DISABLE_LABEL_VALUE {
			klog.V(4).Infof("node %s disable selfhealing, give up.", o.name)
			return true
		}
	}

	return false
}

// hasSpecifiedTaint 检查节点是否有指定的污点
func (o *nodeOptions) hasSpecifiedTaint(ctx context.Context) bool {
	// 从环境变量获取需要检查的污点 key，格式：key1,key2,key3
	taintEnv := os.Getenv("AEGIS_PRECHECK_TAINTS")
	if taintEnv == "" {
		// 如果没有设置环境变量，直接返回 false，不进行检查
		return false
	}

	// 解析污点 key 配置
	taintKeysToCheck := parseTaintKeys(taintEnv)

	for _, taint := range o.node.Spec.Taints {
		for _, key := range taintKeysToCheck {
			if taint.Key == key {
				klog.V(4).Infof("Node %s has specified taint: %s=%s:%s", o.name, taint.Key, taint.Value, taint.Effect)
				return true
			}
		}
	}

	return false
}

// parseTaintKeys 解析污点 key 配置字符串，格式：key1,key2,key3
func parseTaintKeys(config string) []string {
	var keys []string
	parts := strings.Split(config, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			keys = append(keys, part)
		}
	}

	return keys
}

// parseTaintConfig 解析污点配置字符串，格式：key1=effect1,key2=effect2（保留兼容性）
func parseTaintConfig(config string) []v1.Taint {
	var taints []v1.Taint
	parts := strings.Split(config, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			klog.Warningf("Invalid taint config format: %s, expected key=effect", part)
			continue
		}

		key := strings.TrimSpace(kv[0])
		effect := strings.TrimSpace(kv[1])

		var taintEffect v1.TaintEffect
		switch effect {
		case "NoSchedule":
			taintEffect = v1.TaintEffectNoSchedule
		case "PreferNoSchedule":
			taintEffect = v1.TaintEffectPreferNoSchedule
		case "NoExecute":
			taintEffect = v1.TaintEffectNoExecute
		default:
			klog.Warningf("Unknown taint effect: %s, skipping", effect)
			continue
		}

		taints = append(taints, v1.Taint{
			Key:    key,
			Effect: taintEffect,
		})
	}

	return taints
}

// removeTaintAndCordon 移除指定的污点并将节点置为 cordon 状态
func (o *nodeOptions) removeTaintAndCordon(ctx context.Context) error {
	// 从环境变量获取需要移除的污点 key
	taintEnv := os.Getenv("AEGIS_PRECHECK_TAINTS")
	if taintEnv == "" {
		// 如果没有设置环境变量，不应该到达这里，因为 hasSpecifiedTaint 已经返回 false 了
		return fmt.Errorf("no taints configured for removal")
	}
	taintKeysToRemove := parseTaintKeys(taintEnv)

	// 移除指定的污点
	for _, key := range taintKeysToRemove {
		if err := basic.RemoveNodeTaint(ctx, o.bridge, o.name, key, "Precheck: remove specified taints"); err != nil {
			return fmt.Errorf("failed to remove taint with key %s: %v", key, err)
		}
	}

	// 将节点置为 cordon 状态
	return basic.CordonNode(ctx, o.bridge, o.name, "Precheck: removed specified taints", "aegis")
}

