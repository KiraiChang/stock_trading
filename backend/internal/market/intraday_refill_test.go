package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/pkg/timeutil"
)

// 盤中的補回（回答 I-122「缺的 K 棒會不會補回」；I-123～I-126 計畫書「八」）：
// 走正式的抓取流程——真的 Yahoo client（httptest）→ FetchAndStoreIntradayBatch → SQLite 的 candles。
//
// Yahoo 的批次端點一次回傳**當日全部**的 1 分 K，寫入是 `ON CONFLICT … DO UPDATE`，
// 所以某一輪失敗時缺的棒，下一輪成功時就補回來。⚠️ 13:30 是最後一輪，那一輪失敗就沒有下一輪可以補。

// refillServer 依呼叫次序回應：第一輪 503，第二輪是當日 09:00～09:02（09:00 為盤前 null），
// 第三輪 09:02 那根的收盤價改了（盤中那一根還沒走完時的值會被下一輪覆蓋）。
func refillServer(t *testing.T) string {
	t.Helper()
	bodies := []string{"", yahooFixture, refillFixtureUpdated}
	var mu sync.Mutex
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		n := call
		call++
		mu.Unlock()
		if n == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("maintenance"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bodies[n]))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/FinanceChartService.ApacLibraCharts"
}

// refillFixtureUpdated 與 yahooFixture 相同，只有 09:02 的收盤價由 2432 改成 2436。
const refillFixtureUpdated = `[
  {
    "symbol": "2330.TW",
    "chart": {
      "meta": { "name": "台積電", "quoteType": "EQUITY", "gmtoffset": 28800 },
      "timestamp": [1784077200, 1784077260, 1784077320],
      "indicators": { "quote": [ {
        "open":   [null, 2425, 2430],
        "high":   [null, 2430, 2437],
        "low":    [null, 2420, 2425],
        "close":  [null, 2428, 2436],
        "volume": [null, 2262, 640]
      } ] },
      "quote": { "price": "2440", "marketStatus": "open" }
    }
  }
]`

func TestIntradayBatchNextRoundRefillsMissedBars(t *testing.T) {
	_, candles, _ := newAdjusterTestDB(t)
	client := NewYahooQuoteClient(config.YahooConfig{BaseURL: refillServer(t), RateLimit: 10000, BatchSize: 40})
	f := NewFetcher(nil, candles, zap.NewNop())
	f.SetIntradaySource(client)
	ctx := context.Background()
	from := time.Date(2026, 7, 15, 9, 0, 0, 0, timeutil.TaipeiTZ)
	to := from.Add(10 * time.Minute)

	// 09:01 那一輪：Yahoo 503，什麼都沒寫進去。
	if _, err := f.FetchAndStoreIntradayBatch(ctx, []string{"2330"}); err == nil || !IsTransient(err) {
		t.Fatalf("第一輪應是暫時性失敗，得到 %v", err)
	}
	if got, _ := candles.GetRange(ctx, "2330", "1m", from, to); len(got) != 0 {
		t.Fatalf("第一輪失敗，⛔ 不該有棒，得到 %d 根", len(got))
	}

	// 09:02 那一輪：回當日全部 → 第一輪缺的 09:01 補回來。
	if _, err := f.FetchAndStoreIntradayBatch(ctx, []string{"2330"}); err != nil {
		t.Fatalf("第二輪：%v", err)
	}
	got, err := candles.GetRange(ctx, "2330", "1m", from, to)
	if err != nil || len(got) != 2 {
		t.Fatalf("第二輪之後應有 09:01、09:02 兩根，得到 %d 根（err=%v）", len(got), err)
	}
	if ts := got[0].Timestamp.In(timeutil.TaipeiTZ); ts.Hour() != 9 || ts.Minute() != 1 {
		t.Fatalf("第一根應是第一輪缺的 09:01，得到 %s", ts.Format("15:04"))
	}

	// 下一輪覆蓋還在走的那一根（ON CONFLICT DO UPDATE），⛔ 不會重複。
	if _, err := f.FetchAndStoreIntradayBatch(ctx, []string{"2330"}); err != nil {
		t.Fatalf("第三輪：%v", err)
	}
	got, _ = candles.GetRange(ctx, "2330", "1m", from, to)
	if len(got) != 2 || got[1].Close != 2436 || got[1].Volume != 640 {
		t.Fatalf("第三輪之後應仍是兩根、09:02 被覆蓋成 2436／640，得到 %+v", got)
	}
}
