package analyzer

import (
	"fmt"
	"testing"

	"github.com/scitix/aegis/pkg/analyzer/common"
)

func makeCfg(keywords []string, maxOut int) *common.PodLogConfig {
	return &common.PodLogConfig{
		FetchLines:     1000,
		Keywords:       keywords,
		MaxOutputLines: maxOut,
	}
}

func TestFilterLogs_NoKeywords_TailOnly(t *testing.T) {
	raw := make([]string, 100)
	for i := range raw {
		raw[i] = fmt.Sprintf("line %d", i)
	}
	cfg := makeCfg([]string{}, 60)
	got := filterLogs(raw, cfg)
	if len(got) != 60 {
		t.Fatalf("expected 60 lines, got %d", len(got))
	}
	if got[0] != "line 40" || got[59] != "line 99" {
		t.Fatalf("unexpected tail: first=%q last=%q", got[0], got[59])
	}
}

func TestFilterLogs_KeywordsSufficient(t *testing.T) {
	// 80 ERROR lines out of 100 — keyword hits exceed maxOut(60)
	raw := make([]string, 100)
	for i := range raw {
		if i%5 != 0 { // 80 error lines
			raw[i] = fmt.Sprintf("ERROR line %d", i)
		} else {
			raw[i] = fmt.Sprintf("INFO line %d", i)
		}
	}
	cfg := makeCfg([]string{"error"}, 60)
	got := filterLogs(raw, cfg)
	if len(got) != 60 {
		t.Fatalf("expected 60 lines, got %d", len(got))
	}
	// All output lines should contain "ERROR"
	for _, line := range got {
		if line[:5] != "ERROR" {
			t.Errorf("non-keyword line in output: %q", line)
		}
	}
}

func TestFilterLogs_KeywordsInsufficient_Supplement(t *testing.T) {
	// 3 ERROR lines in 10-line log; maxOut=5 → expect 3 keyword + 2 recent
	raw := []string{
		"INFO  boot",           // 0
		"INFO  ready",          // 1
		"ERROR oom",            // 2  keyword
		"INFO  serving",        // 3
		"INFO  request ok",     // 4
		"WARN  slow response",  // 5
		"ERROR timeout",        // 6  keyword
		"INFO  retry",          // 7
		"INFO  ok",             // 8
		"PANIC nil deref",      // 9  keyword (panic)
	}
	cfg := makeCfg([]string{"error", "panic"}, 5)
	got := filterLogs(raw, cfg)

	if len(got) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(got), got)
	}
	// Expected: indices [2, 6, 7, 8, 9] sorted
	expected := []string{raw[2], raw[6], raw[7], raw[8], raw[9]}
	for i, line := range got {
		if line != expected[i] {
			t.Errorf("line[%d]: got %q, want %q", i, line, expected[i])
		}
	}
}

func TestFilterLogs_NoKeywordMatches_FallbackToTail(t *testing.T) {
	raw := []string{"INFO a", "INFO b", "INFO c", "INFO d", "INFO e"}
	cfg := makeCfg([]string{"error"}, 3)
	got := filterLogs(raw, cfg)
	// No matches → supplement fills all 3 slots with tail
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	expected := []string{"INFO c", "INFO d", "INFO e"}
	for i, line := range got {
		if line != expected[i] {
			t.Errorf("line[%d]: got %q, want %q", i, line, expected[i])
		}
	}
}

func TestFilterLogs_EmptyInput(t *testing.T) {
	got := filterLogs([]string{}, makeCfg([]string{"error"}, 60))
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d lines", len(got))
	}
}

func TestFilterLogs_LessThanMaxOut_NoSupplementNeeded(t *testing.T) {
	raw := []string{"ERROR a", "ERROR b"}
	cfg := makeCfg([]string{"error"}, 60)
	got := filterLogs(raw, cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestFilterLogs_CaseInsensitive(t *testing.T) {
	raw := []string{"FATAL crash", "Warning low", "error OOM", "info ok"}
	cfg := makeCfg([]string{"FATAL", "ERROR"}, 10)
	got := filterLogs(raw, cfg)
	found := map[string]bool{}
	for _, line := range got {
		found[line] = true
	}
	if !found["FATAL crash"] {
		t.Error("expected FATAL crash in output")
	}
	if !found["error OOM"] {
		t.Error("expected error OOM in output")
	}
}
