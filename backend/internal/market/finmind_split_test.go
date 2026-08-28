package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/store"
)

// realSplitPayload 是 2026-08-11 從 FinMind 實際取回的 TaiwanStockSplitPrice 回應片段。
//
// **為什麼要用真實 payload 當 fixture**：adjuster 的測試用的是 stub source，所以
// 「FinMind 的 JSON 欄位名對不對」這件事在此之前完全沒有被驗過——欄位改名或拼錯只會
// 得到一堆零值，而零值會被 before/after <= 0 的檢查靜靜丟掉，看起來像「這期間沒有事件」。
//
// 四種 type 都放進來，因為它們是實際會出現的（實測分佈：面額變更 22、反分割 6、
// 分割 4、空字串 1）。
const realSplitPayload = `{"msg":"success","status":200,"data":[
 {"date":"2025-06-18","stock_id":"0050","type":"分割","before_price":188.65,"after_price":47.16,"max_price":51.85,"min_price":42.44,"open_price":47.16},
 {"date":"2024-12-11","stock_id":"00632R","type":"反分割","before_price":3.28,"after_price":22.96,"max_price":25.25,"min_price":20.67,"open_price":22.96},
 {"date":"2019-09-09","stock_id":"6548","type":"面額變更","before_price":312.0,"after_price":31.2,"max_price":34.3,"min_price":28.1,"open_price":31.2},
 {"date":"2026-03-31","stock_id":"00631L","type":"","before_price":443.15,"after_price":20.14,"max_price":24.16,"min_price":16.12,"open_price":20.14},
 {"date":"2020-01-01","stock_id":"BAD","type":"分割","before_price":0,"after_price":0,"max_price":0,"min_price":0,"open_price":0}
]}`

func newSplitTestClient(t *testing.T, payload string) *FinMindClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	return NewFinMindClient(config.FinMindConfig{
		APIKey: "test", BaseURL: srv.URL, RateLimit: 600,
	})
}

func TestFetchSplitPricesParsesRealPayload(t *testing.T) {
	c := newSplitTestClient(t, realSplitPayload)

	got, err := c.FetchSplitPrices(context.Background(),
		time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// 前後價為 0 的那筆要被丟掉——一筆壞事件會污染該檔的整段歷史。
	if len(got) != 4 {
		t.Fatalf("解析出 %d 筆, want 4（價格為 0 的那筆應被丟棄）: %+v", len(got), got)
	}

	byID := map[string]store.CorporateAction{}
	for _, a := range got {
		byID[a.Symbol] = a
	}

	cases := []struct {
		symbol     string
		wantType   string
		wantFactor float64
		wantDate   string
	}{
		{"0050", "分割", 47.16 / 188.65, "2025-06-18"},
		{"00632R", "反分割", 22.96 / 3.28, "2024-12-11"},                          // 係數 > 1
		{"6548", "面額變更", 31.2 / 312.0, "2019-09-09"},                           // 面額變更也要收
		{"00631L", store.CorporateActionUnknown, 20.14 / 443.15, "2026-03-31"}, // type 為空
	}
	for _, tc := range cases {
		a, ok := byID[tc.symbol]
		if !ok {
			t.Errorf("%s 沒有被解析出來", tc.symbol)
			continue
		}
		if a.ActionType != tc.wantType {
			t.Errorf("%s 的 ActionType = %q, want %q——來源的 type 應原樣保留，不可一律記成分割",
				tc.symbol, a.ActionType, tc.wantType)
		}
		if diff := a.Factor - tc.wantFactor; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s 的係數 = %v, want %v", tc.symbol, a.Factor, tc.wantFactor)
		}
		if got := a.EventDate.Format("2006-01-02"); got != tc.wantDate {
			t.Errorf("%s 的事件日 = %s, want %s", tc.symbol, got, tc.wantDate)
		}
		if a.BeforePrice <= 0 || a.AfterPrice <= 0 {
			t.Errorf("%s 的前後價不該是非正數: %+v", tc.symbol, a)
		}
		// 分割會改變股數，成交量係數必須等於價格係數。
		// **漏設會寫入 0**，被 DB 的 ck_corporate_actions_volume_factor 擋下——
		// 2026-08-11 正式環境就是這樣失敗的，而當時這支測試只驗了 Factor。
		if a.VolumeFactor != a.Factor {
			t.Errorf("%s 的 VolumeFactor = %v, want %v（等於價格係數）",
				tc.symbol, a.VolumeFactor, a.Factor)
		}
		if a.VolumeFactor <= 0 {
			t.Errorf("%s 的 VolumeFactor 是非正數，會違反 DB 的 CHECK 約束", tc.symbol)
		}
		if a.Source == "" {
			t.Errorf("%s 沒有記來源", tc.symbol)
		}
	}
}

// TestFetchSplitPricesReverseSplitVolumeDirection：反分割的係數大於 1，
// 對應的成交量調整必須是**縮小**（股數變少）。這是價乘量除在係數 > 1 時的行為，
// 容易被誤以為只有分割（係數 < 1）才需要處理。
func TestFetchSplitPricesReverseSplitVolumeDirection(t *testing.T) {
	c := store.Candle{Close: 3.28, Volume: 7000, AdjFactor: 22.96 / 3.28}

	if got := c.AdjustedClose(); got < 22.95 || got > 22.97 {
		t.Errorf("反分割還原價 = %v, want ≈22.96", got)
	}
	if got := c.AdjustedVolume(); got < 999 || got > 1001 {
		t.Errorf("反分割還原量 = %v, want ≈1000（7 股併 1 股，歷史量要縮小）", got)
	}
}
