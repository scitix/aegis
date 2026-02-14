package deviceaware

import (
	"testing"

	"github.com/scitix/aegis/pkg/prom"
)

// staticLookup is a test stub for ConditionLookup that returns fixed values.
type staticLookup map[string]struct {
	affectsLoad bool
	idMode      string
}

func (s staticLookup) IsLoadAffecting(condition string) bool {
	return s[condition].affectsLoad
}

func (s staticLookup) GetIDMode(condition string) string {
	e, ok := s[condition]
	if !ok {
		return "-"
	}
	return e.idMode
}

func newLookup(pairs ...string) staticLookup {
	// pairs: condition, idMode, condition, idMode, ...
	// AffectsLoad is not used by the parse functions so always false here.
	m := make(staticLookup)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = struct {
			affectsLoad bool
			idMode      string
		}{false, pairs[i+1]}
	}
	return m
}

func statuses(pairs ...string) []prom.AegisNodeStatus {
	// pairs: condition, id, condition, id, ...
	out := make([]prom.AegisNodeStatus, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, prom.AegisNodeStatus{Condition: pairs[i], ID: pairs[i+1]})
	}
	return out
}

// ── parseGPUStatus ────────────────────────────────────────────────────────────

func TestParseGPUStatus_AllMode(t *testing.T) {
	lookup := newLookup("GpuHung", "all")
	got := parseGPUStatus(statuses("GpuHung", ""), lookup)
	if got != "0,1,2,3,4,5,6,7" {
		t.Errorf("all mode: want \"0,1,2,3,4,5,6,7\" got %q", got)
	}
}

func TestParseGPUStatus_IndexMode(t *testing.T) {
	lookup := newLookup("GpuNvlinkError", "index")

	t.Run("single gpu", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuNvlinkError", "3"), lookup)
		if got != "3" {
			t.Errorf("want \"3\" got %q", got)
		}
	})

	t.Run("multiple gpus", func(t *testing.T) {
		ss := []prom.AegisNodeStatus{
			{Condition: "GpuNvlinkError", ID: "1"},
			{Condition: "GpuNvlinkError", ID: "5"},
			{Condition: "GpuNvlinkError", ID: "7"},
		}
		got := parseGPUStatus(ss, lookup)
		if got != "1,5,7" {
			t.Errorf("want \"1,5,7\" got %q", got)
		}
	})

	t.Run("index out of range is ignored", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuNvlinkError", "8"), lookup)
		if got != "" {
			t.Errorf("out-of-range index: want \"\" got %q", got)
		}
	})

	t.Run("non-numeric id is ignored", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuNvlinkError", "abc"), lookup)
		if got != "" {
			t.Errorf("non-numeric id: want \"\" got %q", got)
		}
	})

	t.Run("empty id is ignored", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuNvlinkError", ""), lookup)
		if got != "" {
			t.Errorf("empty id: want \"\" got %q", got)
		}
	})
}

func TestParseGPUStatus_MaskMode(t *testing.T) {
	lookup := newLookup("GpuCheckFailed", "mask")

	t.Run("alternating bits", func(t *testing.T) {
		// "10100101" → GPUs 0,2,5,7 disabled
		got := parseGPUStatus(statuses("GpuCheckFailed", "10100101"), lookup)
		if got != "0,2,5,7" {
			t.Errorf("want \"0,2,5,7\" got %q", got)
		}
	})

	t.Run("all ones", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuCheckFailed", "11111111"), lookup)
		if got != "0,1,2,3,4,5,6,7" {
			t.Errorf("want all gpus got %q", got)
		}
	})

	t.Run("all zeros", func(t *testing.T) {
		got := parseGPUStatus(statuses("GpuCheckFailed", "00000000"), lookup)
		if got != "" {
			t.Errorf("all-zero mask: want \"\" got %q", got)
		}
	})
}

func TestParseGPUStatus_IgnoreMode(t *testing.T) {
	lookup := newLookup("GpuMetricsHang", "-", "GpuRegisterFailed", "-")
	ss := []prom.AegisNodeStatus{
		{Condition: "GpuMetricsHang", ID: "2"},
		{Condition: "GpuRegisterFailed", ID: "4"},
	}
	got := parseGPUStatus(ss, lookup)
	if got != "" {
		t.Errorf("ignore mode: want \"\" got %q", got)
	}
}

func TestParseGPUStatus_UnknownConditionIgnored(t *testing.T) {
	lookup := newLookup() // empty: all conditions unknown → mode "-"
	got := parseGPUStatus(statuses("SomeFutureCondition", "3"), lookup)
	if got != "" {
		t.Errorf("unknown condition: want \"\" got %q", got)
	}
}

func TestParseGPUStatus_MixedModes(t *testing.T) {
	// all-mode fault alongside per-index faults: result must be all GPUs
	lookup := newLookup(
		"GpuHung", "all",
		"GpuNvlinkError", "index",
		"GpuMetricsHang", "-",
	)
	ss := []prom.AegisNodeStatus{
		{Condition: "GpuHung", ID: ""},
		{Condition: "GpuNvlinkError", ID: "2"},
		{Condition: "GpuMetricsHang", ID: "5"},
	}
	got := parseGPUStatus(ss, lookup)
	if got != "0,1,2,3,4,5,6,7" {
		t.Errorf("mixed all+index: want all gpus got %q", got)
	}
}

func TestParseGPUStatus_NoFaults(t *testing.T) {
	got := parseGPUStatus(nil, newLookup())
	if got != "" {
		t.Errorf("no faults: want \"\" got %q", got)
	}
}

// ── parseByIDMode ─────────────────────────────────────────────────────────────

func TestParseByIDMode_IDMode(t *testing.T) {
	lookup := newLookup(
		"IBLinkAbnormal", "id",
		"IBNetDriverFailedLoad", "id",
	)

	t.Run("single device", func(t *testing.T) {
		got := parseByIDMode(statuses("IBLinkAbnormal", "mlx5_0"), lookup)
		if got != "mlx5_0" {
			t.Errorf("want \"mlx5_0\" got %q", got)
		}
	})

	t.Run("multiple devices deduplicated and sorted", func(t *testing.T) {
		ss := []prom.AegisNodeStatus{
			{Condition: "IBLinkAbnormal", ID: "mlx5_2"},
			{Condition: "IBNetDriverFailedLoad", ID: "mlx5_0"},
			{Condition: "IBLinkAbnormal", ID: "mlx5_2"}, // duplicate
		}
		got := parseByIDMode(ss, lookup)
		if got != "mlx5_0,mlx5_2" {
			t.Errorf("want \"mlx5_0,mlx5_2\" got %q", got)
		}
	})

	t.Run("empty id falls back to defaultDeviceId", func(t *testing.T) {
		got := parseByIDMode(statuses("IBLinkAbnormal", ""), lookup)
		if got != defaultDeviceId {
			t.Errorf("empty id: want %q got %q", defaultDeviceId, got)
		}
	})
}

func TestParseByIDMode_AllMode(t *testing.T) {
	lookup := newLookup("IBLost", "all", "IBModuleLost", "all")

	t.Run("single all-mode condition", func(t *testing.T) {
		got := parseByIDMode(statuses("IBLost", ""), lookup)
		if got != defaultDeviceId {
			t.Errorf("want %q got %q", defaultDeviceId, got)
		}
	})

	t.Run("multiple all-mode conditions collapse to one entry", func(t *testing.T) {
		ss := []prom.AegisNodeStatus{
			{Condition: "IBLost", ID: ""},
			{Condition: "IBModuleLost", ID: ""},
		}
		got := parseByIDMode(ss, lookup)
		if got != defaultDeviceId {
			t.Errorf("want single %q got %q", defaultDeviceId, got)
		}
	})
}

func TestParseByIDMode_IgnoreMode(t *testing.T) {
	lookup := newLookup("IBRegisterFailed", "-")
	got := parseByIDMode(statuses("IBRegisterFailed", "mlx5_0"), lookup)
	if got != "" {
		t.Errorf("ignore mode: want \"\" got %q", got)
	}
}

func TestParseByIDMode_UnknownConditionIgnored(t *testing.T) {
	lookup := newLookup() // all unknown → "-"
	got := parseByIDMode(statuses("SomeFutureIBFault", "mlx5_0"), lookup)
	if got != "" {
		t.Errorf("unknown condition: want \"\" got %q", got)
	}
}

func TestParseByIDMode_MixedAllAndID(t *testing.T) {
	// If any condition triggers "all", the result must contain defaultDeviceId
	// alongside any per-device entries.
	lookup := newLookup(
		"IBLost", "all",
		"IBLinkAbnormal", "id",
	)
	ss := []prom.AegisNodeStatus{
		{Condition: "IBLost", ID: ""},
		{Condition: "IBLinkAbnormal", ID: "mlx5_0"},
	}
	got := parseByIDMode(ss, lookup)
	// output is sorted; both "all" and "mlx5_0" should be present
	if got != "all,mlx5_0" {
		t.Errorf("want \"all,mlx5_0\" got %q", got)
	}
}

func TestParseByIDMode_NoFaults(t *testing.T) {
	got := parseByIDMode(nil, newLookup())
	if got != "" {
		t.Errorf("no faults: want \"\" got %q", got)
	}
}
