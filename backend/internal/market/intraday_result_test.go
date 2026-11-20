package market

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/store"
)

type resultCandleRepo struct {
	store.CandleRepo
	failFor string
	written []string
}

func (r *resultCandleRepo) BulkInsert(_ context.Context, cs []store.Candle) error {
	if len(cs) > 0 && cs[0].Symbol == r.failFor {
		return errors.New("disk full")
	}
	if len(cs) > 0 {
		r.written = append(r.written, cs[0].Symbol)
	}
	return nil
}

type resultBatchSource struct {
	resp map[string][]Candle
	err  error
}

func (s resultBatchSource) FetchIntradayCandlesBatch(context.Context, []string) (map[string][]Candle, error) {
	return s.resp, s.err
}
func (resultBatchSource) RateLimit() int { return 600 }
func (resultBatchSource) BatchSize() int { return 10 }

func bar(sym string, h, m int) Candle {
	return Candle{Symbol: sym, Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
		Timestamp: time.Date(2026, 11, 2, h, m, 0, 0, time.UTC)}
}

// I-103：逐檔分得出 written（帶最後一根）、no_data（回應裡沒有）、store_failed（寫入失敗），順序跟請求相同；
// 整批請求失敗才回 error。
func TestFetchAndStoreIntradayBatchPerSymbolResults(t *testing.T) {
	repo := &resultCandleRepo{failFor: "B"}
	f := NewFetcher(nil, repo, zap.NewNop())
	f.SetIntradaySource(resultBatchSource{resp: map[string][]Candle{
		"A": {bar("A", 1, 2), bar("A", 1, 1)},
		"B": {bar("B", 1, 1)},
		"D": {},
	}})

	got, err := f.FetchAndStoreIntradayBatch(context.Background(), 7, []string{"C", "A", "B", "D"})
	if err != nil {
		t.Fatal(err)
	}
	want := []IntradayDataStatus{IntradayNoData, IntradayWritten, IntradayStoreFailed, IntradayNoData}
	if len(got) != len(want) {
		t.Fatalf("得到 %d 筆，期望 %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Status != w {
			t.Fatalf("第 %d 筆（%s）= %s，期望 %s", i, got[i].Symbol, got[i].Status, w)
		}
	}
	if !got[1].LastBarTS.Equal(bar("A", 1, 2).Timestamp) || got[1].Bars != 2 {
		t.Fatalf("A 的最後一根與根數 = %v／%d", got[1].LastBarTS, got[1].Bars)
	}
	if got[2].Err == nil {
		t.Fatal("store_failed 要帶錯誤給呼叫端分類")
	}

	f.SetIntradaySource(resultBatchSource{err: &HTTPStatusError{Code: 500}})
	if res, err := f.FetchAndStoreIntradayBatch(context.Background(), 8, []string{"A"}); err == nil || res != nil {
		t.Fatalf("整批失敗應回 error、⛔ 不逐檔結果，得到 %v／%v", res, err)
	}
}

// 寫入階段 ctx 到期：還沒寫的檔記 store_failed（回應已拿到、停在寫入）。
func TestStoreIntradayBatchContextExpiredIsStoreFailed(t *testing.T) {
	repo := &resultCandleRepo{}
	f := NewFetcher(nil, repo, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := f.StoreIntradayBatch(ctx, 1, []string{"A", "B"}, map[string][]Candle{"A": {bar("A", 1, 1)}})
	if got[0].Status != IntradayStoreFailed || !errors.Is(got[0].Err, context.Canceled) || got[1].Status != IntradayNoData {
		t.Fatalf("得到 %+v", got)
	}
}

type minuteClient struct {
	MarketDataSource
	cs  []Candle
	err error
}

func (c minuteClient) FetchMinuteCandles(context.Context, string, time.Time) ([]Candle, error) {
	return c.cs, c.err
}

// D4：FinMind 逐檔路徑的空回應是 no_data（以前回成功、接著用舊 K 棒評估）。
func TestFetchAndStoreMinuteResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client minuteClient
		repo   *resultCandleRepo
		want   IntradayDataStatus
	}{
		{"空回應", minuteClient{}, &resultCandleRepo{}, IntradayNoData},
		{"抓取失敗", minuteClient{err: errors.New("boom")}, &resultCandleRepo{}, IntradayFetchFailed},
		{"寫入失敗", minuteClient{cs: []Candle{bar("A", 1, 1)}}, &resultCandleRepo{failFor: "A"}, IntradayStoreFailed},
		{"成功", minuteClient{cs: []Candle{bar("A", 1, 1)}}, &resultCandleRepo{}, IntradayWritten},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFetcher(tc.client, tc.repo, zap.NewNop())
			if got := f.FetchAndStoreMinute(context.Background(), "A", time.Now()); got.Status != tc.want {
				t.Fatalf("得到 %s，期望 %s", got.Status, tc.want)
			}
		})
	}
}

// I-123 的觀察工具依訊息名稱與欄位過濾：`fetched intraday batch candles` 的 requested／stored ⛔ 不改，只新增欄位。
func TestFetchedIntradayBatchLogKeepsFields(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	f := NewFetcher(nil, &resultCandleRepo{failFor: "B"}, zap.New(core))
	f.SetIntradaySource(resultBatchSource{resp: map[string][]Candle{"A": {bar("A", 1, 1)}, "B": {bar("B", 1, 1)}}})
	if _, err := f.FetchAndStoreIntradayBatch(context.Background(), 42, []string{"A", "B", "C"}); err != nil {
		t.Fatal(err)
	}
	entries := logs.FilterMessage("fetched intraday batch candles").All()
	if len(entries) != 1 {
		t.Fatalf("應有 1 筆，得到 %d", len(entries))
	}
	got := entries[0].ContextMap()
	for k, v := range map[string]any{"requested": int64(3), "stored": int64(1), "no_data": int64(1), "store_failed": int64(1), "run_id": uint64(42)} {
		if got[k] != v {
			t.Fatalf("%s = %#v，期望 %#v（%v）", k, got[k], v, got)
		}
	}
	if n := logs.FilterMessage("intraday batch store failed").Len(); n != 1 {
		t.Fatalf("寫入失敗的 log 應有 1 筆，得到 %d", n)
	}
}
