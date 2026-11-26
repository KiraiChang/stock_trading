package portfolio

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// T-089 D2：新計畫的 evidence.decision_context 多記來源 timeframe 與 market_data_at（SR 快照的 analyzed_at），
// 其他鍵⛔ 沒有變；計畫的 analyzed_at 照舊複製 SR 快照的 analyzed_at。
func TestBuildSnapshotRecordsSourceTimeframeAndMarketDataAt(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	day := time.Date(2026, 11, 24, 16, 0, 0, 0, time.UTC) // 台北 11-25 00:00
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: day, CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"action":"BuySmall"}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}
	snap, err := a.buildSnapshot(&store.Position{PortfolioID: 1, Symbol: "2330", Version: 4}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.AnalyzedAt.Equal(day) || snap.PositionVersion != 4 || snap.RuleVersion != "position_sr_zone_v1" {
		t.Fatalf("analyzed_at、position_version、rule_version 照舊；得到 %v %d %s", snap.AnalyzedAt, snap.PositionVersion, snap.RuleVersion)
	}
	var ev map[string]any
	if err := json.Unmarshal([]byte(snap.Evidence), &ev); err != nil {
		t.Fatal(err)
	}
	dc := ev["decision_context"].(map[string]any)
	if dc["timeframe"] != "1d" {
		t.Fatalf("decision_context.timeframe 要是 1d，得到 %v", dc["timeframe"])
	}
	at, err := time.Parse(time.RFC3339Nano, dc["market_data_at"].(string))
	if err != nil || !at.Equal(day) {
		t.Fatalf("decision_context.market_data_at 要等於 SR 快照的 analyzed_at，得到 %v（%v）", dc["market_data_at"], err)
	}
	keys := make([]string, 0, len(dc))
	for k := range dc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "avg_cost,current_price,has_position,market_data_at,mode,position_state,shares,sr_zone_analysis_id,timeframe"
	if got := strings.Join(keys, ","); got != want {
		t.Fatalf("decision_context 只多兩個鍵、既有的都在；得到 %s", got)
	}
}
