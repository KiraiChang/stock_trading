package analysis

import (
	"strings"
	"testing"
)

func TestReportValueExtractors(t *testing.T) {
	// Python 回傳的 report 走 encoding/json，數值一律是 float64；int 分支是給
	// 直接以 Go map 組出來的來源用的。
	report := map[string]any{
		"run_id":       "sr_replay_001",
		"rows":         float64(12),
		"sources":      3,
		"bad_string":   float64(1),
		"bad_int":      "not-a-number",
		"null_payload": nil,
	}

	if got := StringFromReport(report, "run_id"); got != "sr_replay_001" {
		t.Fatalf("StringFromReport = %q, want sr_replay_001", got)
	}
	if got := StringFromReport(report, "missing"); got != "" {
		t.Fatalf("StringFromReport missing key = %q, want empty", got)
	}
	if got := StringFromReport(report, "bad_string"); got != "" {
		t.Fatalf("StringFromReport wrong type = %q, want empty", got)
	}
	if got := StringFromReport(report, "null_payload"); got != "" {
		t.Fatalf("StringFromReport null = %q, want empty", got)
	}

	if got := IntFromReport(report, "rows"); got != 12 {
		t.Fatalf("IntFromReport float64 = %d, want 12", got)
	}
	if got := IntFromReport(report, "sources"); got != 3 {
		t.Fatalf("IntFromReport int = %d, want 3", got)
	}
	if got := IntFromReport(report, "missing"); got != 0 {
		t.Fatalf("IntFromReport missing key = %d, want 0", got)
	}
	if got := IntFromReport(report, "bad_int"); got != 0 {
		t.Fatalf("IntFromReport wrong type = %d, want 0", got)
	}
}

// job_id 只有毫秒解析度，手動 API 與 cron 同一毫秒觸發過就會撞 UNIQUE 約束，
// 所以後綴必須帶隨機值。
func TestNewEvaluationJobIDIsUniqueWithinSameMillisecond(t *testing.T) {
	const n = 200
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := NewEvaluationJobID()
		if !strings.HasPrefix(id, "sr_eval_job_") {
			t.Fatalf("unexpected job id format: %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate job id generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
