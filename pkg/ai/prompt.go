package ai

import (
	"bytes"
	"fmt"
	"text/template"
)

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
}

func GetRenderedPrompt(kind string, data PromptData) (string, error) {
	tpl, ok := PromptRegistry[kind]
	if !ok {
		return "", fmt.Errorf("unknown prompt type: %s", kind)
	}
	return tpl.Render(tpl.Content, data)
}
