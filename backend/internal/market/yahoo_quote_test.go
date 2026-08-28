package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/pkg/timeutil"
)

func newTestYahooClient(t *testing.T, body string) *YahooQuoteClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewYahooQuoteClient(config.YahooConfig{
		// 正式端點的 baseURL 尾端是路徑段（...ApacLibraCharts），matrix 參數（;autoRefresh=...）
		// 接在其後才能被 url.Parse 正確解析。httptest server.URL 只有 host:port、無路徑，
		// 直接接 ";" 會被當成非法 port，故補一段路徑還原真實端點形狀（handler 對所有路徑皆回應）。
		BaseURL:   server.URL + "/FinanceChartService.ApacLibraCharts",
		RateLimit: 10000,
		BatchSize: 40,
	})
}

// yahooFixture 對應實測回應：2330.TW 首筆為盤前 null、其後兩根有效；
// 0050.TW（ETF）整段陣列為 null（實測盤後常見），應被完全跳過。
// timestamp：1784077200=09:00、1784077260=09:01、1784077320=09:02（台北時區）。
const yahooFixture = `[
  {
    "symbol": "2330.TW",
    "chart": {
      "meta": { "name": "台積電", "quoteType": "EQUITY", "gmtoffset": 28800 },
      "timestamp": [1784077200, 1784077260, 1784077320],
      "indicators": { "quote": [ {
        "open":   [null, 2425, 2430],
        "high":   [null, 2430, 2435],
        "low":    [null, 2420, 2425],
        "close":  [null, 2428, 2432],
        "volume": [null, 2262, 502]
      } ] },
      "quote": { "price": "2440", "marketStatus": "close" }
    }
  },
  {
    "symbol": "0050.TW",
    "chart": {
      "meta": { "name": "元大台灣50", "quoteType": "ETF", "gmtoffset": 28800 },
      "timestamp": [1784077200, 1784077260, 1784077320],
      "indicators": { "quote": [ {
        "open":   [null, null, null],
        "high":   [null, null, null],
        "low":    [null, null, null],
        "close":  [null, null, null],
        "volume": [null, null, null]
      } ] },
      "quote": { "price": "106.30", "marketStatus": "close" }
    }
  }
]`

func TestFetchIntradayCandlesBatch_ParsesSkipsNullAndMapsSymbols(t *testing.T) {
	client := newTestYahooClient(t, yahooFixture)

	got, err := client.FetchIntradayCandlesBatch(context.Background(), []string{"2330", "0050"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 0050 全 null 應被跳過，不出現在 map
	if _, ok := got["0050"]; ok {
		t.Errorf("0050 全為 null 棒，不應出現在結果中")
	}

	// 2330 應以系統格式（去尾碼）為 key，首筆 null 被跳過，剩兩根
	candles := got["2330"]
	if len(candles) != 2 {
		t.Fatalf("2330 期望 2 根 K 棒（首筆 null 跳過），實得 %d", len(candles))
	}

	first := candles[0]
	if first.Symbol != "2330" {
		t.Errorf("symbol 應轉回系統格式 2330，實得 %s", first.Symbol)
	}
	if first.Timeframe != "1m" {
		t.Errorf("timeframe 應為 1m，實得 %s", first.Timeframe)
	}
	if first.Open != 2425 || first.High != 2430 || first.Low != 2420 || first.Close != 2428 || first.Volume != 2262 {
		t.Errorf("首根 OHLCV 對映錯誤：%+v", first)
	}
	// 1784077260 = 2026-07-15 09:01 台北時間
	ts := first.Timestamp.In(timeutil.TaipeiTZ)
	if ts.Hour() != 9 || ts.Minute() != 1 {
		t.Errorf("timestamp 應為台北 09:01，實得 %s", ts.Format("15:04"))
	}
}

func TestFetchIntradayCandlesBatch_EmptyInput(t *testing.T) {
	client := newTestYahooClient(t, yahooFixture)
	got, err := client.FetchIntradayCandlesBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("空輸入應回傳空 map，實得 %d 筆", len(got))
	}
}

func TestSymbolConversion(t *testing.T) {
	cases := []struct {
		sys, yahoo string
	}{
		{"2330", "2330.TW"},      // TWSE 補預設尾碼
		{"6488.TWO", "6488.TWO"}, // 已含尾碼原樣保留（上櫃）
	}
	for _, c := range cases {
		if got := toYahooSymbol(c.sys); got != c.yahoo {
			t.Errorf("toYahooSymbol(%q) = %q, 期望 %q", c.sys, got, c.yahoo)
		}
	}
	if got := fromYahooSymbol("2330.TW"); got != "2330" {
		t.Errorf("fromYahooSymbol(2330.TW) = %q, 期望 2330", got)
	}
	if got := fromYahooSymbol("6488.TWO"); got != "6488" {
		t.Errorf("fromYahooSymbol(6488.TWO) = %q, 期望 6488", got)
	}
}
