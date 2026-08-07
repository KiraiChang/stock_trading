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
	return []Candle{{Symbol: symbol, Timeframe: "1d", Close: 100, Timestamp: time.Now()}}, nil
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
