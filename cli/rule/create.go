package rule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	_name "github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"gitlab.scitix-inner.ai/k8s/aegis/cli/config"
	"gitlab.scitix-inner.ai/k8s/aegis/tools"
	"k8s.io/klog/v2"

	rulev1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/rule/v1alpha1"
	templatev1alpha1 "gitlab.scitix-inner.ai/k8s/aegis/pkg/apis/template/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewCreateCommand(config *config.AegisCliConfig, use string) *cobra.Command {
	o := &createOptions{
		config: config,
	}

	c := &cobra.Command{
		Use:   use + " Name",
		Short: "Create a aegis ops rule",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.complete(cmd, args); err != nil {
				klog.Fatalf("%v", err)
			}

			if err := o.validate(); err != nil {
				klog.Fatalf("Invalid create option: %v", err)
			}

			if err := o.run(); err != nil {
				klog.Fatalf("Create run failed: %v", err)
			}
		},
		Example: `aegiscli rule create clean-disk-v1 \
--alert-status Firing \
--alert-type NodeOutOfDiskSpace \
--base-ops-image k8s/aegis:test \
--ops-file-path ./clean-disk.sh \
--namespace alert \
--ops-target Node`,
	}

	c.PersistentFlags().StringVarP(&o.namespace, "namespace", "n", "alert", "rule namespace")
	c.PersistentFlags().StringVar(&o.alertType, "alert-type", "", "associcated alert type")
	c.PersistentFlags().StringVar(&o.alertStatus, "alert-status", "Firing", "associcated alert status")
	c.PersistentFlags().StringVar(&o.alertSelector, "label-selector", "", "selector alert by labels")
	c.PersistentFlags().StringVar(&o.opsFilePath, "ops-file-path", "", "ops file path in host machine or in ops image rootfs")
	c.PersistentFlags().BoolVar(&o.enablePreStepWebhook, "enable-pre-step-webhook", false, "enable pre-step notify webhook")
	c.PersistentFlags().StringVar((*string)(&o.opsTarget), "ops-target", "Node", "ops target, Node or Pod")
	c.PersistentFlags().StringVar(&o.baseImage, "base-ops-image", "library/centos:latest", "The base image used to create new ops image")
	c.PersistentFlags().StringVar(&o.baseImage, "ops-image", "", "The ops image used to run workflow")
	c.PersistentFlags().BoolVar(&o.enableOverrideOpsImage, "enable-override-ops-image", false, "Override ops image if exists")
	c.PersistentFlags().StringVar(&o.serviceaccount, "service-account", "aegis-workflow", "The ops workflow service account")
	c.PersistentFlags().StringVar(&o.template, "template", getDefaultTemplatePath(), "workflow template file path")

	return c
}

type createOptions struct {
	name           string
	namespace      string
	serviceaccount string

	// match alert
	alertType     string
	alertStatus   string
	alertSelector string
	selectors     map[string]string

	opsFilePath string
	opsFileName string
	opsTarget   OpsTarget
	template    string

	baseImage              string
	opsImage               string
	enableOverrideOpsImage bool

	// notify step, can enable when OpsTarget = Node
	enablePreStepWebhook bool

	config *config.AegisCliConfig
}

type OpsTarget string

const (
	NodeTarget OpsTarget = "Node"
	PodTarget  OpsTarget = "Pod"
)

// first args is rule name
func (o *createOptions) complete(cmd *cobra.Command, args []string) error {
	argsLen := cmd.ArgsLenAtDash()
	if argsLen == -1 {
		argsLen = len(args)
	}

	if argsLen != 1 {
		return fmt.Errorf("exactly one Name is required, got: %d", argsLen)
	}

	o.name = args[0]
	return nil
}

func (o *createOptions) validate() (err error) {
	if len(o.name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}

	if len(o.alertType) == 0 {
		return fmt.Errorf("alert-type cannot be empty")
	}

	if len(o.opsFilePath) == 0 {
		return fmt.Errorf("ops-file-path cannot be empty")
	}

	if len(o.alertSelector) > 0 {
		o.selectors = make(map[string]string)
		ps := strings.Split(o.alertSelector, ";")
		for _, p := range ps {
			kv := strings.Split(p, ":")
			if len(kv) != 2 {
				return fmt.Errorf("invalid label selector format: %s", p)
			}
			o.selectors[kv[0]] = kv[1]
		}
	}

	// dest image
	ref, err := _name.ParseReference(o.baseImage)
	if err != nil {
		return err
	}

	if len(o.opsImage) == 0 {
		o.opsImage = fmt.Sprintf("%s:%s", ref.Context().String(), o.name)
	}

	// cannot override image
	// _, err = crane.Head(o.opsImage)
	// if err == nil {
	// 	return fmt.Errorf("Cannot override ops image %s", o.opsImage)
	// }

	if o.opsTarget != NodeTarget && o.opsTarget != PodTarget {
		return fmt.Errorf("Invalid ops target: %s, should be Node/Pod", o.opsTarget)
	}

	// ops file
	o.opsFileName = filepath.Base(o.opsFilePath)

	_, err = o.config.TemplateClient.AegisV1alpha1().AegisOpsTemplates(o.namespace).Get(context.Background(), o.name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Cannot override ops template %s/%s", o.namespace, o.name)
	}

	_, err = o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(o.namespace).Get(context.Background(), o.name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Cannot override ops rule %s/%s", o.namespace, o.name)
	}

	return nil
}

func (o *createOptions) run() (err error) {
	// create ops image
	base := o.baseImage
	registry := o.config.GetRegistryAddress()
	if registry != "" {
		base = fmt.Sprintf("%s/%s", registry, base)
	}

	// if target image exist, just use it
	_, err = crane.Head(o.opsImage)
	if err != nil || o.enableOverrideOpsImage {
		// targz file
		new_layer := o.opsFilePath + ".tar"
		if err := tools.CompressTargz(o.opsFilePath, new_layer); err != nil {
			return err
		}
		defer os.Remove(new_layer)

		if err := o.createOpsImage(base, new_layer, o.opsImage); err != nil {
			return err
		}
	} else {
		klog.Infof("ops image %s already exists, skip create new image", o.opsImage)
	}

	// create ops template
	if err := o.createTemplate(); err != nil {
		return err
	}

	// create ops rule
	if err := o.createRule(); err != nil {
		return err
	}
	return nil
}

func (o *createOptions) createOpsImage(baseRef, new_layer string, newImage string) (err error) {
	klog.Infof("--- Starting create ops image: %s ---", newImage)
	defer func() {
		if err != nil {
			klog.Errorf("--- Create ops image failed: %v ---", err)
		} else {
			klog.Info("--- Succeeded creating ops image ---")
		}
	}()
	base, err := crane.Pull(baseRef)
	if err != nil {
		return fmt.Errorf("pulling %s: %v", baseRef, err)
	}

	img, err := crane.Append(base, new_layer)
	if err != nil {
		return fmt.Errorf("appending %v: %v", new_layer, err)
	}

	if err := crane.Push(img, newImage); err != nil {
		return fmt.Errorf("pushing %s: %v", newImage, err)
	}
	return nil
}

func (o *createOptions) createTemplate() (err error) {
	klog.Infof("--- Starting create ops template: %s/%s ---", o.namespace, o.name)
	defer func() {
		if err != nil {
			klog.Errorf("--- Create ops template failed: %v ---", err)
		} else {
			klog.Info("--- Succeeded creating ops template ---")
		}
	}()

	// create manifest from tpl
	templateStr, err := tools.LoadFromFile(o.template)
	if err != nil {
		return fmt.Errorf("Load template file %s failed: %v", o.template, err)
	}

	parameters := o.getParameters()
	yamlContent, err := tools.RenderWorkflowTemplate(templateStr, parameters)
	if err != nil {
		return fmt.Errorf("Render template failed: %v", err)
	}

	template := &templatev1alpha1.AegisOpsTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: templatev1alpha1.AegisOpsTemplateSpec{
			Manifest: yamlContent,
		},
	}

	_, err = o.config.TemplateClient.AegisV1alpha1().AegisOpsTemplates(o.namespace).Create(context.Background(), template, metav1.CreateOptions{})
	return err
}

func (o *createOptions) createRule() (err error) {
	klog.Infof("--- Starting create ops rule: %s/%s ---", o.namespace, o.name)
	defer func() {
		if err != nil {
			klog.Errorf("--- Create ops rule failed: %v ---", err)
		} else {
			klog.Info("--- Succeeded creating ops rule ---")
		}
	}()

	var selector *metav1.LabelSelector
	if len(o.selectors) > 0 {
		selector = &metav1.LabelSelector{
			MatchLabels: o.selectors,
		}
	}

	rule := &rulev1alpha1.AegisAlertOpsRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: rulev1alpha1.AegisAlertOpsRuleSpec{
			AlertConditions: []rulev1alpha1.AegisAlertCondition{
				{
					Type:   o.alertType,
					Status: o.alertStatus,
				},
			},
			Selector: selector,
			OpsTemplate: &corev1.ObjectReference{
				Kind:       "AegisOpsTemplate",
				APIVersion: "aegis.io/v1alpha1",
				Namespace:  o.namespace,
				Name:       o.name,
			},
		},
	}

	_, err = o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(o.namespace).Create(context.Background(), rule, metav1.CreateOptions{})
	return err
}

func (o *createOptions) getParameters() map[string]interface{} {
	return map[string]interface{}{
		"OpsImage":       o.opsImage,
		"OpsTarget":      o.opsTarget,
		"OpsFileName":    o.opsFileName,
		"EnableWebhook":  o.enablePreStepWebhook,
		"ServiceAccount": o.serviceaccount,
	}
}

func getDefaultTemplatePath() string {
	return filepath.Join(os.Getenv("HOME"), ".aegis", "workflow-template.yaml")
}
