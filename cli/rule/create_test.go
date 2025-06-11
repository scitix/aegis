package rule

import (
	"testing"

	_name "github.com/google/go-containerregistry/pkg/name"
	"github.com/scitix/aegis/tools"
)

func TestRenderWorkflowTemplate(t *testing.T) {
	o := &createOptions{
		template:             getDefaultTemplatePath(),
		serviceaccount:       "aegis-test",
		opsFileName:          "script.sh",
		opsTarget:            "Node",
		opsImage:             "nginx:latest",
		enablePreStepWebhook: false,
	}

	parameters := o.getParameters()
	templateStr, err := tools.LoadFromFile(o.template)
	if err != nil {
		t.Errorf("Load template file %s failed: %v", o.template, err)
	}

	yamlContent, err := tools.RenderWorkflowTemplate(templateStr, parameters)
	if err != nil {
		t.Errorf("Render template failed: %v", err)
	}

	t.Logf("yaml: %s", yamlContent)
}

func TestParseImage(t *testing.T) {
	image := "k8s/aegis:test"
	expectedContext := "k8s/aegis"
	ref, err := _name.ParseReference(image)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ref.Context().String() != expectedContext {
		t.Logf("expected: %s, got: %s", ref.Context(), ref.Context().String())
	}
}
