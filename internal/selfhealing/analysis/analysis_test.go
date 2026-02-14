package analysis

import (
	"os"
	"strings"
	"testing"

	"github.com/scitix/aegis/pkg/prom"
)

func TestParseConditionConfig(t *testing.T) {
	t.Run("four columns full", func(t *testing.T) {
		input := `
# comment line
GpuHung:3:true:all
GpuCheckFailed:3:true:mask
GpuNvlinkError:9:true:index
GpfsMountLost:10:true:id
HighGpuTemp:100:false:-
`
		got, err := ParseConditionConfig(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		cases := []struct {
			condition    string
			priority     Priority
			affectsLoad  bool
			deviceIDMode string
		}{
			{"GpuHung", 3, true, "all"},
			{"GpuCheckFailed", 3, true, "mask"},
			{"GpuNvlinkError", 9, true, "index"},
			{"GpfsMountLost", 10, true, "id"},
			{"HighGpuTemp", 100, false, "-"},
		}
		for _, c := range cases {
			cfg, ok := got[c.condition]
			if !ok {
				t.Errorf("condition %q not found", c.condition)
				continue
			}
			if cfg.Priority != c.priority {
				t.Errorf("%s priority: want %d got %d", c.condition, c.priority, cfg.Priority)
			}
			if cfg.AffectsLoad != c.affectsLoad {
				t.Errorf("%s AffectsLoad: want %v got %v", c.condition, c.affectsLoad, cfg.AffectsLoad)
			}
			if cfg.DeviceIDMode != c.deviceIDMode {
				t.Errorf("%s DeviceIDMode: want %q got %q", c.condition, c.deviceIDMode, cfg.DeviceIDMode)
			}
		}
		if len(got) != len(cases) {
			t.Errorf("entry count: want %d got %d", len(cases), len(got))
		}
	})

	t.Run("two columns backward compat", func(t *testing.T) {
		input := `
NodeNotReady:0
GpuHung:3
IBLost:9
`
		got, err := ParseConditionConfig(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for cond, wantPri := range map[string]Priority{"NodeNotReady": 0, "GpuHung": 3, "IBLost": 9} {
			cfg, ok := got[cond]
			if !ok {
				t.Errorf("condition %q not found", cond)
				continue
			}
			if cfg.Priority != wantPri {
				t.Errorf("%s priority: want %d got %d", cond, wantPri, cfg.Priority)
			}
			if cfg.AffectsLoad != false {
				t.Errorf("%s AffectsLoad: want false got %v", cond, cfg.AffectsLoad)
			}
			if cfg.DeviceIDMode != "-" {
				t.Errorf("%s DeviceIDMode: want \"-\" got %q", cond, cfg.DeviceIDMode)
			}
		}
	})

	t.Run("three columns partial", func(t *testing.T) {
		input := `IBLost:3:true`
		got, err := ParseConditionConfig(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg := got["IBLost"]
		if cfg.Priority != 3 || !cfg.AffectsLoad || cfg.DeviceIDMode != "-" {
			t.Errorf("unexpected cfg: %+v", cfg)
		}
	})

	t.Run("blank lines and comments ignored", func(t *testing.T) {
		input := `
# full comment

  # indented comment
GpuHung:3:true:all
`
		got, err := ParseConditionConfig(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("want 1 entry, got %d", len(got))
		}
	})

	t.Run("whitespace around fields", func(t *testing.T) {
		input := `  GpuHung : 3 : true : all `
		got, err := ParseConditionConfig(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg, ok := got["GpuHung"]
		if !ok {
			t.Fatal("GpuHung not found")
		}
		if cfg.Priority != 3 || !cfg.AffectsLoad || cfg.DeviceIDMode != "all" {
			t.Errorf("unexpected cfg: %+v", cfg)
		}
	})

	t.Run("invalid priority returns error", func(t *testing.T) {
		_, err := ParseConditionConfig("GpuHung:notanumber:true:all")
		if err == nil {
			t.Fatal("expected error for invalid priority")
		}
	})

	t.Run("invalid AffectsLoad returns error", func(t *testing.T) {
		_, err := ParseConditionConfig("GpuHung:3:yes:all")
		if err == nil {
			t.Fatal("expected error for invalid AffectsLoad")
		}
	})

	t.Run("too many columns returns error", func(t *testing.T) {
		_, err := ParseConditionConfig("GpuHung:3:true:all:extra")
		if err == nil {
			t.Fatal("expected error for 5-column entry")
		}
	})

	t.Run("too few columns returns error", func(t *testing.T) {
		_, err := ParseConditionConfig("GpuHung")
		if err == nil {
			t.Fatal("expected error for 1-column entry")
		}
	})
}

func TestParsePriorityConfigBackwardCompat(t *testing.T) {
	// ParsePriorityConfig must accept both old 2-column and new 4-column
	// content and return only the Priority map.
	input := strings.Join([]string{
		"NodeNotReady:0",
		"GpuHung:3:true:all",
		"GpfsMountLost:10:true:id",
	}, "\n")

	got, err := ParsePriorityConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]Priority{
		"NodeNotReady":  0,
		"GpuHung":       3,
		"GpfsMountLost": 10,
	}
	for cond, wantPri := range want {
		if got[cond] != wantPri {
			t.Errorf("%s: want priority %d got %d", cond, wantPri, got[cond])
		}
	}
	if len(got) != len(want) {
		t.Errorf("entry count: want %d got %d", len(want), len(got))
	}
}

func TestParseConditionConfigRealFile(t *testing.T) {
	// Smoke-test: the real priority.conf must parse without error and contain
	// a representative set of known conditions with correct attributes.
	data, err := os.ReadFile("./priority.conf")
	if err != nil {
		t.Fatalf("cannot read priority.conf: %v", err)
	}

	got, err := ParseConditionConfig(string(data))
	if err != nil {
		t.Fatalf("failed to parse priority.conf: %v", err)
	}

	checks := []struct {
		cond         string
		affectsLoad  bool
		deviceIDMode string
	}{
		{"GpuHung", true, "all"},
		{"GpuCheckFailed", true, "mask"},
		{"GpuNvlinkError", true, "index"},
		{"HighGpuTemp", false, "-"},
		{"GpfsMountLost", true, "id"},
		{"IBLost", true, "all"},
		{"IBNetDriverFailedLoad", true, "id"},
		{"NodeNotReady", true, "-"},
		{"NodeHasRestarted", false, "-"},
	}
	for _, c := range checks {
		cfg, ok := got[c.cond]
		if !ok {
			t.Errorf("condition %q missing from priority.conf", c.cond)
			continue
		}
		if cfg.AffectsLoad != c.affectsLoad {
			t.Errorf("%s AffectsLoad: want %v got %v", c.cond, c.affectsLoad, cfg.AffectsLoad)
		}
		if cfg.DeviceIDMode != c.deviceIDMode {
			t.Errorf("%s DeviceIDMode: want %q got %q", c.cond, c.deviceIDMode, cfg.DeviceIDMode)
		}
	}
}

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
