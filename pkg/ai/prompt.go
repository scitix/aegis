package ai

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"k8s.io/klog/v2"
)

const promptOverridePath = "/aegis/prompt"

type PromptData struct {
	ErrorInfo string
	EventInfo string
	LogInfo   string
	Metadata  map[string]string // 扩展

	Language string
	RawAlert string
}

type PromptTemplate struct {
	Name    string
	Content string
	Render  func(tmpl string, data PromptData) (string, error)
}

func RenderTextTemplate(tmpl string, data PromptData) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

var PromptRegistry = map[string]PromptTemplate{
	"default": {
		Name:    "default",
		Content: defaultPromptTemplate,
		Render:  RenderTextTemplate,
	},
	"Node": {
		Name:    "Node",
		Content: nodePromptTemplate,
		Render:  RenderTextTemplate,
	},
	"Pod": {
		Name:    "Pod",
		Content: podPromptTemplate,
		Render:  RenderTextTemplate,
	},
	"AlertParse": {
		Name:    "AlertParse",
		Content: alertToModelPromptTemplate,
		Render:  RenderTextTemplate,
	},
	"PytorchJob": {
		Name:    "PytorchJob",
		Content: pytorchJobPromptTemplate,
		Render:  RenderTextTemplate,
	},
}

func GetRenderedPrompt(kind string, data PromptData) (string, error) {
	tpl, ok := PromptRegistry[kind]
	if !ok {
		return "", fmt.Errorf("unknown prompt type: %s", kind)
	}

	var content string
	overrideContent, err := LoadPromptOverride(kind)
	if err == nil {
		klog.Infof("using override prompt for kind %s's AI diagnosis", kind)
		content = overrideContent
	} else {
		content = tpl.Content
	}

	return tpl.Render(content, data)
}

func LoadPromptOverride(kind string) (string, error) {
	filename := filepath.Join(promptOverridePath, strings.ToLower(kind) + ".tmpl")
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
