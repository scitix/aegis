package analysis

import (
	"testing"

	"github.com/scitix/aegis/pkg/prom"
)

func TestInitAndAnalysis(t *testing.T) {
	err := InitAnalysisConfig("./priority.conf")
	if err != nil {
		t.Fatalf("%s", err)
	}

	t.Logf("%v", nodeOperateConfig)

	status := []prom.AegisNodeStatus{
		{
			Condition: "NodeCordon",
		},
		{
			Condition: "XIDApplicationErr",
		},
		{
			Condition: "GpfsMountLost",
		},

		{
			Condition: "GpuRegisterFailed",
		},
	}

	result := AnalysisNodeStatus(status)
	t.Logf("%+v", result)
	t.Logf("%+v", result.Cordon)
}
