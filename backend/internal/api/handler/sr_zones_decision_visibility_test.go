package handler

import (
	"testing"

	"github.com/trading/backend/internal/store"
)

// 階段 D 的隔離：decision_visible=false 的事件只進 states，不進任何決策桶。
//
// **這份 Go 實作不是備援而是必要**：event_state_summary 的桶有兩份實作，Python 在
// build_event_state_summary、Go 在 eventStateSummaryJSON，而 carry-forward 的回程
// 走的是 Go 這份。少了它，新事件會在第二次分析時經 active_bullish_events 進入決策。
func TestEventStateSummaryJSONExcludesDecisionInvisibleStates(t *testing.T) {
	states := []store.MarketEventState{
		{
			EventKey:        "ZONE:SUPPORT_BREAKDOWN:SUPPORT:98.0000:100.0000",
			EventType:       "HIGH_VOLUME_BREAKDOWN",
			EventFamily:     "SUPPORT_BREAKDOWN",
			EventScope:      "ZONE",
			ZoneKey:         "SUPPORT:98.0000:100.0000",
			RootEventType:   "HIGH_VOLUME_BREAKDOWN",
			LatestEventType: "HIGH_VOLUME_BREAKDOWN",
			Direction:       "BEARISH",
			State:           "CONFIRMED",
			Active:          true,
			StateJSON:       store.RawJSON(`{"decision_visible":true}`),
		},
		{
			EventKey:        "ZONE:SUPPORT_RETEST:SUPPORT:98.0000:100.0000",
			EventType:       "SUPPORT_RETEST_HELD",
			EventFamily:     "SUPPORT_RETEST",
			EventScope:      "ZONE",
			ZoneKey:         "SUPPORT:98.0000:100.0000",
			RootEventType:   "SUPPORT_RETEST_HELD",
			LatestEventType: "SUPPORT_RETEST_HELD",
			Direction:       "BULLISH",
			State:           "CONFIRMED",
			Active:          true,
			StateJSON:       store.RawJSON(`{"decision_visible":false}`),
		},
	}

	summary := eventStateSummaryJSON(nil, states)

	if got := len(summary["states"].([]any)); got != 2 {
		t.Fatalf("states 要收全部（持久化來源），got %d want 2", got)
	}
	for _, bucket := range []string{"active", "candidates", "confirmed", "resolved", "expired", "active_bearish_events", "active_bullish_events"} {
		for _, item := range summary[bucket].([]any) {
			if item.(map[string]any)["type"] == "SUPPORT_RETEST_HELD" {
				t.Fatalf("decision_visible=false 的事件不該出現在 %s", bucket)
			}
		}
	}
	if got := len(summary["active_bullish_events"].([]any)); got != 0 {
		t.Fatalf("active_bullish_events 只看 direction，必須被旗標擋掉，got %d", got)
	}
	if got := summary["latest_event_type"]; got != "HIGH_VOLUME_BREAKDOWN" {
		t.Fatalf("latest_event_type 不該被不可見事件頂掉，got %v", got)
	}
}

// 階段 D 之前寫進 market_event_states 的列沒有 decision_visible 這個鍵。
// 缺鍵當成 false 會讓既有事件整批從決策桶消失——那是最嚴重的行為改變，所以預設是 true。
func TestEventDecisionVisibleDefaultsToTrue(t *testing.T) {
	cases := []struct {
		name string
		raw  store.RawJSON
		want bool
	}{
		{"state_json 是空的", store.RawJSON(""), true},
		{"沒有這個鍵", store.RawJSON(`{"carried_from_previous":true}`), true},
		{"解析不了", store.RawJSON(`{`), true},
		{"明寫 true", store.RawJSON(`{"decision_visible":true}`), true},
		{"明寫 false", store.RawJSON(`{"decision_visible":false}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := eventDecisionVisible(tc.raw); got != tc.want {
				t.Fatalf("eventDecisionVisible = %v, want %v", got, tc.want)
			}
		})
	}
}
