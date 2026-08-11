package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// realYahooPayload 取自 2026-08-11 的實際回應，**四種記錄形狀都放進來**，
// 因為每一種都曾經讓我寫錯：
//
//  1. 多次配息標的的「年度彙總」列：recordType=YEAR 但**沒有 exDate**，不是事件。
//  2. 多次配息標的的事件列：recordType=SUB，有 exDate。
//  3. **一年只配一次的標的：事件列的 recordType 也是 YEAR**，而且自己帶 exDate。
//     用 recordType 過濾會讓這類標的（市場上大多數）變成「沒有除權息」——不會報錯，就是空的。
//  4. exDatePreviousClose.raw 為字串 "-"（無資料），直接轉型會炸。
const realYahooPayload = `{"data":{"dividendByYear":[
 {"yearBySort":"2026","recordType":"YEAR","period":"","exDividend":{"cash":"1.60"},"exRight":{"stock":"-"}},
 {"exDate":"2026-07-21T00:00:00+08:00","year":"2026","period":"H1","symbol":"0050","recordType":"SUB",
  "exDatePreviousClose":{"fmt":"99.20","raw":"99.2"},"exDividend":{"cash":"0.60"},"exRight":null},
 {"exDate":"2026-01-22T00:00:00+08:00","year":"2025","period":"H2","symbol":"0050","recordType":"SUB",
  "exDatePreviousClose":{"fmt":"71.85","raw":71.85},"exDividend":{"cash":"1.00"},"exRight":null},
 {"exDate":"2026-07-02T00:00:00+08:00","year":"2025","period":"FY","symbol":"2317","recordType":"YEAR",
  "exDatePreviousClose":{"fmt":"248.0","raw":"248"},"exDividend":{"cash":"5.80"},"exRight":{"stock":"-"}},
 {"exDate":"2016-09-02T00:00:00+08:00","year":"2016","period":"FY","symbol":"2317","recordType":"YEAR",
  "exDatePreviousClose":{"fmt":"87.40","raw":"87.4"},"exDividend":{"cash":"4.00"},"exRight":{"stock":"1.00"}},
 {"exDate":"2010-01-01T00:00:00+08:00","year":"2010","period":"FY","symbol":"XXXX","recordType":"YEAR",
  "exDatePreviousClose":{"fmt":"-","raw":"-"},"exDividend":{"cash":"1.00"},"exRight":{"stock":"-"}}
]}}`

func newYahooDividendTestClient(t *testing.T, payload string) *YahooDividendClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	c := NewYahooDividendClient(600, zap.NewNop())
	c.baseURLForTest = srv.URL
	return c
}

func TestYahooDividendParsesAllRecordShapes(t *testing.T) {
	c := newYahooDividendTestClient(t, realYahooPayload)

	got, err := c.FetchDividends(context.Background(), "0050")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// 年度彙總（沒有 exDate）與 prevClose 為 "-" 的都要被排除，剩 4 筆。
	if len(got) != 4 {
		t.Fatalf("解析出 %d 筆, want 4: %+v", len(got), got)
	}

	byDate := map[string]store.CorporateAction{}
	for _, a := range got {
		byDate[a.EventDate.Format("2006-01-02")] = a
	}

	// 一年只配一次的標的，事件列的 recordType 是 YEAR——必須被收進來。
	if _, ok := byDate["2026-07-02"]; !ok {
		t.Error("recordType=YEAR 但有 exDate 的事件被漏掉了——不可用 recordType 過濾")
	}

	// 純現金：價格下修，但**股數沒變，成交量係數必須是 1**。
	cash := byDate["2026-07-21"]
	if want := (99.2 - 0.6) / 99.2; !nearly(cash.Factor, want) {
		t.Errorf("純現金的價格係數 = %v, want %v", cash.Factor, want)
	}
	if cash.VolumeFactor != 1 {
		t.Errorf("純現金的成交量係數 = %v, want 1——現金股利不改變股數", cash.VolumeFactor)
	}
	if cash.ActionType != store.CorporateActionDividendCash {
		t.Errorf("ActionType = %q, want %q", cash.ActionType, store.CorporateActionDividendCash)
	}

	// 配股＋配息：兩個係數都要含 (1+stock/10) 的分母。
	both := byDate["2016-09-02"]
	if want := (87.4 - 4.0) / 1.1 / 87.4; !nearly(both.Factor, want) {
		t.Errorf("權息的價格係數 = %v, want %v", both.Factor, want)
	}
	if want := 1 / 1.1; !nearly(both.VolumeFactor, want) {
		t.Errorf("權息的成交量係數 = %v, want %v——配股會改變股數", both.VolumeFactor, want)
	}
	if both.ActionType != store.CorporateActionDividendBoth {
		t.Errorf("ActionType = %q, want %q", both.ActionType, store.CorporateActionDividendBoth)
	}

	// raw 可能是 JSON 數字而不是字串，兩種都要吃。
	if _, ok := byDate["2026-01-22"]; !ok {
		t.Error("raw 為 JSON 數字的那筆被漏掉了")
	}
}

func nearly(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// TestYahooDividendCashOnlyKeepsVolumeUnchanged 是本階段最容易寫錯的一條，
// 獨立成一支測試：現金股利若被算進成交量，長期回測的量能指標會系統性偏移，
// 而且不會有任何東西報錯。
func TestYahooDividendCashOnlyKeepsVolumeUnchanged(t *testing.T) {
	c := store.Candle{Close: 100, Volume: 1000, AdjFactor: 0.99, VolFactor: 1}

	if got := c.AdjustedClose(); !nearly(got, 99) {
		t.Errorf("還原價 = %v, want 99", got)
	}
	if got := c.AdjustedVolume(); got != 1000 {
		t.Errorf("還原量 = %v, want 1000——現金股利不該調整成交量", got)
	}
}

// TestAdjustedVolumeFallsBackToAdjFactor：Phase 1 寫入的資料沒有 vol_factor，
// 那時只有分割、價量共用一個係數，所以退回 AdjFactor 才是對的。
func TestAdjustedVolumeFallsBackToAdjFactor(t *testing.T) {
	c := store.Candle{Close: 188.65, Volume: 1000, AdjFactor: 0.25} // VolFactor 為 0

	if got := c.AdjustedVolume(); got != 4000 {
		t.Errorf("VolFactor 缺值時還原量 = %v, want 4000（退回 AdjFactor）", got)
	}
}
