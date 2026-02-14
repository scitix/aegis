package nodepoller

import (
	"testing"
)

// reload is the internal hot-reload path; drive it directly in tests so we
// don't need a live Kubernetes API server.
func watcherFromContent(t *testing.T, content string) *PriorityWatcher {
	t.Helper()
	w := NewPriorityWatcher()
	w.reload(map[string]string{"priority": content})
	return w
}

const testConfig = `
# format: Condition:Priority:AffectsLoad:DeviceIDMode
NodeNotReady:0:true:-
NodeCordon:1:true:-
GpuHung:3:true:all
GpuCheckFailed:3:true:mask
GpuNvlinkError:9:true:index
GpfsMountLost:10:true:id
HighGpuTemp:100:false:-
GpuMetricsHang:99:false:-
IBLost:3:true:all
IBPortSpeedAbnormal:10:false:id
`

func TestPriorityWatcher_IsCritical(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	cases := []struct {
		condition string
		want      bool
	}{
		{"NodeNotReady", true},   // priority 0
		{"NodeCordon", true},     // priority 1
		{"GpuHung", true},        // priority 3 ≤ Emergency(99)
		{"GpuNvlinkError", true},    // priority 9
		{"GpuMetricsHang", true},    // priority 99 == Emergency boundary → critical
		{"HighGpuTemp", false},      // priority 100 > Emergency
		{"UnknownCondition", false}, // not in config: conservative false
	}
	for _, c := range cases {
		if got := w.IsCritical(c.condition); got != c.want {
			t.Errorf("IsCritical(%q) = %v, want %v", c.condition, got, c.want)
		}
	}
}

func TestPriorityWatcher_IsCordon(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	cases := []struct {
		condition string
		want      bool
	}{
		{"NodeCordon", true},
		{"NodeNotReady", false},  // priority 0, not exactly NodeCordon(1)
		{"GpuHung", false},
		{"UnknownCondition", false},
	}
	for _, c := range cases {
		if got := w.IsCordon(c.condition); got != c.want {
			t.Errorf("IsCordon(%q) = %v, want %v", c.condition, got, c.want)
		}
	}
}

func TestPriorityWatcher_IsLoadAffecting(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	cases := []struct {
		condition string
		want      bool
	}{
		{"GpuHung", true},
		{"GpuNvlinkError", true},
		{"IBLost", true},
		{"GpfsMountLost", true},
		{"HighGpuTemp", false},
		{"GpuMetricsHang", false},
		{"IBPortSpeedAbnormal", false},
		{"UnknownCondition", false},
	}
	for _, c := range cases {
		if got := w.IsLoadAffecting(c.condition); got != c.want {
			t.Errorf("IsLoadAffecting(%q) = %v, want %v", c.condition, got, c.want)
		}
	}
}

func TestPriorityWatcher_GetIDMode(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	cases := []struct {
		condition string
		want      string
	}{
		{"GpuHung", "all"},
		{"GpuCheckFailed", "mask"},
		{"GpuNvlinkError", "index"},
		{"GpfsMountLost", "id"},
		{"HighGpuTemp", "-"},
		{"NodeNotReady", "-"},
		{"UnknownCondition", "-"}, // unknown: conservative "-"
	}
	for _, c := range cases {
		if got := w.GetIDMode(c.condition); got != c.want {
			t.Errorf("GetIDMode(%q) = %q, want %q", c.condition, got, c.want)
		}
	}
}

func TestPriorityWatcher_HotReload(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	// Before reload: GpuHung is critical and load-affecting
	if !w.IsCritical("GpuHung") {
		t.Fatal("before reload: IsCritical(GpuHung) should be true")
	}
	if !w.IsLoadAffecting("GpuHung") {
		t.Fatal("before reload: IsLoadAffecting(GpuHung) should be true")
	}

	// Reload with updated config: GpuHung priority raised to 200, AffectsLoad=false
	w.reload(map[string]string{"priority": "GpuHung:200:false:-"})

	if w.IsCritical("GpuHung") {
		t.Error("after reload: IsCritical(GpuHung) should be false (priority 200)")
	}
	if w.IsLoadAffecting("GpuHung") {
		t.Error("after reload: IsLoadAffecting(GpuHung) should be false")
	}
	// Previous entries not in new config should be gone
	if w.IsCritical("IBLost") {
		t.Error("after reload: IBLost should no longer be in config")
	}
}

func TestPriorityWatcher_ReloadMissingKey(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	// reload with a ConfigMap that has no "priority" key: config must be unchanged
	before := w.IsCritical("GpuHung")
	w.reload(map[string]string{"other-key": "GpuHung:200:false:-"})
	if w.IsCritical("GpuHung") != before {
		t.Error("reload with missing key should leave config unchanged")
	}
}

func TestPriorityWatcher_ReloadInvalidContent(t *testing.T) {
	w := watcherFromContent(t, testConfig)

	// reload with unparseable content: config must be unchanged
	w.reload(map[string]string{"priority": "BAD:::CONTENT:::TOO:::MANY"})
	// GpuHung should still be resolvable from the original load
	if !w.IsCritical("GpuHung") {
		t.Error("reload with invalid content should leave config unchanged")
	}
}

func TestPriorityWatcher_BackwardCompatTwoColumns(t *testing.T) {
	// PriorityWatcher must still work when the ConfigMap contains the old
	// two-column format (no AffectsLoad / DeviceIDMode columns).
	w := watcherFromContent(t, `
NodeNotReady:0
GpuHung:3
IBPortDown:999
`)
	if !w.IsCritical("GpuHung") {
		t.Error("2-col format: GpuHung should be critical")
	}
	// defaults: AffectsLoad=false, DeviceIDMode="-"
	if w.IsLoadAffecting("GpuHung") {
		t.Error("2-col format: AffectsLoad should default to false")
	}
	if got := w.GetIDMode("GpuHung"); got != "-" {
		t.Errorf("2-col format: DeviceIDMode should default to \"-\", got %q", got)
	}
}
