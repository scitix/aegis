package ai

import (
	"strings"
	"testing"
)

func TestGetRenderedPrompt_Default(t *testing.T) {
	data := PromptData{
		ErrorInfo: "Pod crashloop due to OOM",
		EventInfo: "BackOff restarting failed container",
		LogInfo:   "OOMKilled",
		Language:  "English",
	}

	output, err := GetRenderedPrompt("default", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Error:") || !strings.Contains(output, "Solution:") {
		t.Errorf("output format invalid:\n%s", output)
	}
}

func TestGetRenderedPrompt_Node(t *testing.T) {
	data := PromptData{
		ErrorInfo: "NodeNotReady with type=default",
		EventInfo: "Kubelet stopped sending heartbeats",
		LogInfo:   "kubelet failed to start container runtime",
	}

	output, err := GetRenderedPrompt("Node", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Healthy:") || !strings.Contains(output, "Error:") || !strings.Contains(output, "Solution:") {
		t.Errorf("Node template output missing expected fields:\n%s", output)
	}
}

func TestGetRenderedPrompt_Unknown(t *testing.T) {
	data := PromptData{
		ErrorInfo: "Some error",
	}

	_, err := GetRenderedPrompt("UnknownType", data)
	if err == nil {
		t.Errorf("expected error for unknown prompt type, got nil")
	}
}
