package market

import (
	"context"
	"testing"

	"github.com/trading/backend/internal/store"
)

// realCapitalReductionPayload 取自 2026-08-11 的實際回應（2478 有兩筆，另兩檔各一筆）。
//
// 這三個案例的 `ClosingPriceonTheLastTradingDay` 與我們 candles 裡的前一根收盤價
// **完全相同**（22.75 / 80.80 / 35.80），是資料源正確性的獨立佐證。
const realCapitalReductionPayload = `{"msg":"success","status":200,"data":[
 {"date":"2019-10-07","stock_id":"2478","ClosingPriceonTheLastTradingDay":35.8,"PostReductionReferencePrice":44.4,"LimitUp":48.8,"LimitDown":40.0,"OpeningReferencePrice":44.4,"ExrightReferencePrice":-1.0,"ReasonforCapitalReduction":"彌補虧損"},
 {"date":"2016-10-21","stock_id":"2478","ClosingPriceonTheLastTradingDay":17.65,"PostReductionReferencePrice":20.99,"LimitUp":23.05,"LimitDown":18.9,"OpeningReferencePrice":21.0,"ExrightReferencePrice":-1.0,"ReasonforCapitalReduction":"彌補虧損"},
 {"date":"2020-01-01","stock_id":"BAD","ClosingPriceonTheLastTradingDay":0,"PostReductionReferencePrice":0,"LimitUp":0,"LimitDown":0,"OpeningReferencePrice":0,"ExrightReferencePrice":-1.0,"ReasonforCapitalReduction":""}
]}`

func TestFetchCapitalReductionsParsesRealPayload(t *testing.T) {
	c := newSplitTestClient(t, realCapitalReductionPayload)

	got, err := c.FetchCapitalReductions(context.Background(), "2478")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	// 價格為 0 的那筆要被丟掉——一筆壞事件會污染該檔的整段歷史。
	if len(got) != 2 {
		t.Fatalf("解析出 %d 筆, want 2（價格為 0 的應被丟棄）: %+v", len(got), got)
	}

	byDate := map[string]store.CorporateAction{}
	for _, a := range got {
		byDate[a.EventDate.Format("2006-01-02")] = a
	}

	a := byDate["2019-10-07"]
	if a.Symbol != "2478" {
		t.Errorf("Symbol = %q, want 2478", a.Symbol)
	}
	if a.ActionType != store.CorporateActionCapitalReduction {
		t.Errorf("ActionType = %q, want %q", a.ActionType, store.CorporateActionCapitalReduction)
	}
	if want := 44.4 / 35.8; !nearly(a.Factor, want) {
		t.Errorf("Factor = %v, want %v", a.Factor, want)
	}
	// **減資的係數必定 > 1**：股數變少、價格變高，與反分割同向。
	if a.Factor <= 1 {
		t.Errorf("減資的係數 = %v, 應該 > 1（股數變少、價格變高）", a.Factor)
	}
	// 減資改變股數，所以成交量係數等於價格係數——不可以是 1。
	if a.VolumeFactor != a.Factor {
		t.Errorf("VolumeFactor = %v, want %v（等於價格係數，因為股數確實改變）",
			a.VolumeFactor, a.Factor)
	}
	if a.BeforePrice != 35.8 || a.AfterPrice != 44.4 {
		t.Errorf("前後價 = %v / %v, want 35.8 / 44.4", a.BeforePrice, a.AfterPrice)
	}
	if _, ok := byDate["2016-10-21"]; !ok {
		t.Error("同一檔的第二筆事件被漏掉了")
	}
}

// TestCapitalReductionAdjustsVolumeDownward：減資的係數 > 1，成交量調整方向是**縮小**。
// 這與分割相反，容易寫錯——股數變少，歷史的成交量張數要跟著縮小才可比。
func TestCapitalReductionAdjustsVolumeDownward(t *testing.T) {
	// 2603 的實際數字：80.80 → 187.00（係數 2.3144）
	c := store.Candle{Close: 80.80, Volume: 23144, AdjFactor: 187.0 / 80.8, VolFactor: 187.0 / 80.8}

	if got := c.AdjustedClose(); got < 186.9 || got > 187.1 {
		t.Errorf("還原價 = %v, want ≈187", got)
	}
	if got := c.AdjustedVolume(); got < 9999 || got > 10001 {
		t.Errorf("還原量 = %v, want ≈10000（股數變少，歷史量要縮小）", got)
	}
	if raw, adj := c.Close*float64(c.Volume), c.AdjustedClose()*c.AdjustedVolume(); raw-adj > 1 || adj-raw > 1 {
		t.Errorf("還原前後乘積不同: %v vs %v", raw, adj)
	}
}
