package market

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// stubDailySource 只實作 BackfillHistory 會走到的 FetchDailyCandles；
// errBySymbol 有值時該檔回錯，用來製造「部分失敗」。
type stubDailySource struct {
	errBySymbol map[string]error
}

func (s *stubDailySource) FetchDailyCandles(_ context.Context, symbol string, _, _ time.Time) ([]Candle, error) {
	if err, ok := s.errBySymbol[symbol]; ok {
		return nil, err
	}
	// 四個價格都要給：toStoreCandles 會擋掉價格非正的 K 棒，
	// 只設 Close 的 stub 會被整根丟掉，讓測試因為錯誤的理由而通過。
	return []Candle{{
		Symbol: symbol, Timeframe: "1d",
		Open: 99, High: 101, Low: 98, Close: 100,
		Timestamp: time.Now(),
	}}, nil
}

func (s *stubDailySource) FetchMinuteCandles(context.Context, string, time.Time) ([]Candle, error) {
	return nil, errors.New("not used")
}

// stubCandleRepo 只需要 BulkInsert；insertErr 有值時模擬寫入失敗。
type stubCandleRepo struct {
	store.CandleRepo
	insertErr error
	inserted  int
}

func (r *stubCandleRepo) BulkInsert(_ context.Context, cs []store.Candle) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	r.inserted += len(cs)
	return nil
}

type callbackRecord struct {
	symbol string
	err    error
}

func TestBackfillHistoryReportsEverySymbol(t *testing.T) {
	fetchErr := errors.New("finmind 429")
	src := &stubDailySource{errBySymbol: map[string]error{"2454": fetchErr}}
	repo := &stubCandleRepo{}
	f := NewFetcher(src, repo, zap.NewNop())

	var got []callbackRecord
	failed := f.BackfillHistory(context.Background(), []string{"2330", "2454", "2317"}, 5,
		func(symbol string, err error) {
			got = append(got, callbackRecord{symbol, err})
		})

	if failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
	// 回呼要**每一檔都觸發一次**（含成功），否則前端進度條會卡在失敗檔之前不動。
	if len(got) != 3 {
		t.Fatalf("回呼次數 = %d, want 3（每檔一次）: %+v", len(got), got)
	}
	want := []string{"2330", "2454", "2317"}
	for i, w := range want {
		if got[i].symbol != w {
			t.Fatalf("第 %d 次回呼 symbol = %q, want %q", i, got[i].symbol, w)
		}
	}
	if got[0].err != nil || got[2].err != nil {
		t.Fatalf("成功的檔 err 應為 nil: %+v", got)
	}
	if !errors.Is(got[1].err, fetchErr) {
		t.Fatalf("失敗的檔應帶出原始錯誤，got %v", got[1].err)
	}
}

func TestBackfillHistoryReportsInsertFailure(t *testing.T) {
	insertErr := errors.New("db is down")
	f := NewFetcher(&stubDailySource{}, &stubCandleRepo{insertErr: insertErr}, zap.NewNop())

	var got []callbackRecord
	failed := f.BackfillHistory(context.Background(), []string{"2330"}, 5,
		func(symbol string, err error) { got = append(got, callbackRecord{symbol, err}) })

	// 抓得到但寫不進去也算失敗，回呼要帶出寫入錯誤而不是 nil。
	if failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
	if len(got) != 1 || !errors.Is(got[0].err, insertErr) {
		t.Fatalf("回呼未帶出寫入錯誤: %+v", got)
	}
}

func TestBackfillHistoryNilCallback(t *testing.T) {
	// scheduler 的每日回補傳 nil，不能因此 panic。
	f := NewFetcher(&stubDailySource{errBySymbol: map[string]error{"2454": errors.New("boom")}},
		&stubCandleRepo{}, zap.NewNop())

	if failed := f.BackfillHistory(context.Background(), []string{"2330", "2454"}, 5, nil); failed != 1 {
		t.Fatalf("failed = %d, want 1", failed)
	}
}


func TestToStoreCandlesDropsNonPositivePrices(t *testing.T) {
	// live DB 曾出現 4 根 OHLCV 全為 0 的日 K。無成交的日子應該是
	// 「沒有那筆資料」，不是一根價格為 0 的 K 棒——留著會污染 MA / ATR / zone 建構，
	// 而且不會有任何東西報錯。
	f := NewFetcher(&stubDailySource{}, &stubCandleRepo{}, zap.NewNop())
	now := time.Now()

	cases := []struct {
		name   string
		candle Candle
		want   bool
	}{
		{"全零", Candle{Symbol: "3630", Timeframe: "1d", Timestamp: now}, false},
		{"只有 open 為 0", Candle{Symbol: "a", Open: 0, High: 10, Low: 9, Close: 10, Timestamp: now}, false},
		{"只有 high 為 0", Candle{Symbol: "a", Open: 10, High: 0, Low: 9, Close: 10, Timestamp: now}, false},
		{"只有 low 為 0", Candle{Symbol: "a", Open: 10, High: 11, Low: 0, Close: 10, Timestamp: now}, false},
		{"只有 close 為 0", Candle{Symbol: "a", Open: 10, High: 11, Low: 9, Close: 0, Timestamp: now}, false},
		{"負價", Candle{Symbol: "a", Open: -1, High: 11, Low: 9, Close: 10, Timestamp: now}, false},
		{"正常", Candle{Symbol: "a", Open: 10, High: 11, Low: 9, Close: 10, Timestamp: now}, true},
		// volume 為 0 是正常的（該分鐘沒成交），不該被擋。
		{"零成交量但價格正常", Candle{Symbol: "a", Open: 10, High: 11, Low: 9, Close: 10, Volume: 0, Timestamp: now}, true},
	}

	for _, tc := range cases {
		got := f.toStoreCandles([]Candle{tc.candle})
		if kept := len(got) == 1; kept != tc.want {
			t.Errorf("%s: 保留 = %v, want %v", tc.name, kept, tc.want)
		}
	}
}

func TestToStoreCandlesKeepsGoodCandlesInBatch(t *testing.T) {
	// 一根壞的不該讓整批消失——只丟那一根，其餘照常寫入。
	f := NewFetcher(&stubDailySource{}, &stubCandleRepo{}, zap.NewNop())
	now := time.Now()

	got := f.toStoreCandles([]Candle{
		{Symbol: "2330", Open: 10, High: 11, Low: 9, Close: 10, Timestamp: now},
		{Symbol: "2330", Timestamp: now.Add(time.Hour)},
		{Symbol: "2330", Open: 12, High: 13, Low: 11, Close: 12, Timestamp: now.Add(2 * time.Hour)},
	})

	if len(got) != 2 {
		t.Fatalf("保留 %d 根, want 2", len(got))
	}
	if got[0].Close != 10 || got[1].Close != 12 {
		t.Fatalf("保留的順序或內容不對: %+v", got)
	}
}
