package rule

import (
	"context"
	"fmt"

	"github.com/scitix/aegis/cli/config"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func NewDeleteCmd(config *config.AegisCliConfig, use string) *cobra.Command {
	o := &deleteOption{
		config: config,
	}

	c := &cobra.Command{
		Use:   use + " Name",
		Short: "Delete a aegis ops rule",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.complete(cmd, args); err != nil {
				klog.Fatalf("%v", err)
			}

			if err := o.validate(); err != nil {
				klog.Fatalf("Invalid delete option: %v", err)
			}

			if err := o.run(); err != nil {
				klog.Fatalf("Delete run failed: %v", err)
			}
		},
		Example: `aegiscli rule delete testrule --namespace default`,
	}

	c.PersistentFlags().StringVarP(&o.namespace, "namespace", "n", "alert", "rule namespace")

	return c
}

type deleteOption struct {
	name      string
	namespace string

	config *config.AegisCliConfig
}

// first args is rule name
func (o *deleteOption) complete(cmd *cobra.Command, args []string) error {
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

func (o *deleteOption) validate() error {
	if len(o.name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

func (o *deleteOption) run() error {
	rule, err := o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(o.namespace).Get(context.Background(), o.name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	ref := rule.Spec.OpsTemplate

	if err := o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(rule.Namespace).Delete(context.Background(), rule.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}
	klog.Infof("Ops Rule %s/%s has been deleted", rule.Namespace, rule.Name)

	if err := o.config.TemplateClient.AegisV1alpha1().AegisOpsTemplates(ref.Namespace).Delete(context.Background(), ref.Name, metav1.DeleteOptions{}); apierrors.IsNotFound(err) {
		klog.Warningf("Ops Template %s/%s not found, skip", ref.Namespace, ref.Name)
	} else if err != nil {
		return err
	} else {
		klog.Infof("Ops Template %s/%s has been deleted", ref.Namespace, ref.Name)
	}

	return nil
}
