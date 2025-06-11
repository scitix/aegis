package rule

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/scitix/aegis/cli/config"
	rulev1alpha1 "github.com/scitix/aegis/pkg/apis/rule/v1alpha1"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func NewGetCmd(config *config.AegisCliConfig, use string) *cobra.Command {
	o := &getOption{
		config: config,
	}

	c := &cobra.Command{
		Use:   use + " Name",
		Short: "Get aegis ops rules",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.complete(cmd, args); err != nil {
				klog.Fatalf("%v", err)
			}

			if err := o.validate(); err != nil {
				klog.Fatalf("Invalid get option: %v", err)
			}

			if err := o.run(); err != nil {
				klog.Fatalf("Get run failed: %v", err)
			}
		},
		Example: `aegiscli rule get testrule --namespace default`,
	}

	c.PersistentFlags().StringVarP(&o.namespace, "namespace", "n", "alert", "Rule namespace")
	c.PersistentFlags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace")

	return c
}

type getOption struct {
	name      string
	namespace string

	allNamespaces bool

	config *config.AegisCliConfig
}

type ruleObject struct {
	namespace         string
	name              string
	status            string
	templateNamespace string
	templateName      string
	templateStatus    string
}

// first args is rule name
func (o *getOption) complete(cmd *cobra.Command, args []string) error {
	argsLen := cmd.ArgsLenAtDash()
	if argsLen == -1 {
		argsLen = len(args)
	}

	if argsLen > 1 {
		return fmt.Errorf("at most one Name is required, got: %d", argsLen)
	} else if argsLen == 1 {
		o.name = args[0]
	} else {
		o.name = ""
	}

	return nil
}

func (o *getOption) validate() error {
	return nil
}

func (o *getOption) run() error {
	namespace := o.namespace
	if o.allNamespaces {
		namespace = metav1.NamespaceAll
	}

	rules := make([]rulev1alpha1.AegisAlertOpsRule, 0)

	if len(o.name) > 0 {
		rule, err := o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(namespace).Get(context.Background(), o.name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		rules = append(rules, *rule)
	} else {
		ruleList, err := o.config.RuleClient.AegisV1alpha1().AegisAlertOpsRules(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		rules = append(rules, ruleList.Items...)
	}

	ruleObjects := make([]ruleObject, 0)
	for _, rule := range rules {
		object := ruleObject{
			namespace:         rule.Namespace,
			name:              rule.Name,
			status:            rule.Status.Status,
			templateNamespace: rule.Spec.OpsTemplate.Namespace,
			templateName:      rule.Spec.OpsTemplate.Name,
		}

		template, err := o.config.TemplateClient.AegisV1alpha1().AegisOpsTemplates(object.templateNamespace).Get(context.Background(), object.templateName, metav1.GetOptions{})
		if err == nil {
			object.templateStatus = template.Status.Status
		} else if apierrors.IsNotFound(err) {
			object.templateStatus = "NotExists"
		} else {
			return err
		}

		ruleObjects = append(ruleObjects, object)
	}

	return o.printRules(ruleObjects)
}

func (o *getOption) printRules(objects []ruleObject) error {
	if len(objects) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No Rules found")
		return nil
	}

	o.printTable(objects)
	return nil
}

func (o *getOption) printTable(objects []ruleObject) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if o.allNamespaces {
		_, _ = fmt.Fprint(w, "NAMESPACE\t")
	}

	_, _ = fmt.Fprint(w, "NAME\tSTATUS\tTEMPLATE\tTEMPLATESTATUS")
	_, _ = fmt.Fprintf(w, "\n")
	for _, object := range objects {
		if o.allNamespaces {
			_, _ = fmt.Fprintf(w, "%s\t", object.namespace)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s/%s\t%s", object.name, object.status, object.templateNamespace, object.templateName, object.templateStatus)
		_, _ = fmt.Fprintf(w, "\n")
	}
	w.Flush()
}
