package indicator

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// 這一組測試守的是 docs/architecture.md「寫入失敗的一致性契約」 的 indicator 那半：
// Compute 的三種失敗必須分得開，且 Upsert 失敗要 fail-fast。

// 嵌入介面：只實作這組測試會用到的方法，其餘被呼叫時 nil panic——
// 那正是想要的訊號（測試不該碰到它們）。
type stubCandleRepo struct {
	store.CandleRepo
	candles []store.Candle
	err     error
}

func (s *stubCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	return s.candles, s.err
}

type stubIndicatorRepo struct {
	store.IndicatorRepo
	upsertErr error
	upserted  int
}

func (s *stubIndicatorRepo) Upsert(context.Context, *store.IndicatorSnapshot) error {
	s.upserted++
	return s.upsertErr
}

func enoughCandles(n int) []store.Candle {
	out := make([]store.Candle, 0, n)
	base := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		p := 100.0 + float64(i%7)
		out = append(out, store.Candle{
			Symbol: "2454", Timeframe: "1m",
			Open: p, High: p + 1, Low: p - 1, Close: p,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return out
}

func newTestEngine(candles store.CandleRepo, ind store.IndicatorRepo) *Engine {
	// redis 傳 nil client：RedisClient 的方法都會在 rdb == nil 時早退，
	// 這組測試不碰快取行為。
	return NewEngine(candles, ind, &store.RedisClient{}, zap.NewNop())
}

// TestComputeDistinguishesReadFailureFromInsufficientData 是本段的核心分流測試。
//
// **原本兩者共用一個分支**（`if err != nil || len(candles) < 35`），handler 因此
// 無法把 DB 讀取失敗回 5xx、把資料不足回 422。
func TestComputeDistinguishesReadFailureFromInsufficientData(t *testing.T) {
	t.Run("讀取失敗不得被當成資料不足", func(t *testing.T) {
		cause := errors.New("dial tcp 10.0.0.5:5432: connection refused")
		e := newTestEngine(&stubCandleRepo{err: cause}, &stubIndicatorRepo{})

		_, err := e.Compute(context.Background(), "2454", "1m")
		if err == nil {
			t.Fatal("讀取失敗必須回 error")
		}
		if errors.Is(err, ErrInsufficientCandles) {
			t.Error("讀取失敗被誤判成資料不足——handler 會把它回成 422")
		}
		if !errors.Is(err, cause) {
			t.Error("讀取失敗必須包住原 cause，供內部 log 與錯誤分類使用")
		}
	})

	t.Run("資料不足是 sentinel，且不得包 nil cause", func(t *testing.T) {
		e := newTestEngine(&stubCandleRepo{candles: enoughCandles(minCandles - 1)}, &stubIndicatorRepo{})

		_, err := e.Compute(context.Background(), "2454", "1m")
		if !errors.Is(err, ErrInsufficientCandles) {
			t.Fatalf("資料不足要能用 errors.Is 判斷，得到 %v", err)
		}
		// 原本寫成 `%w, err` 而 err == nil，訊息會出現 %!w(<nil>)、Unwrap 也拿不到東西。
		if got := err.Error(); contains(got, "%!w") || contains(got, "<nil>") {
			t.Errorf("資料不足的 error 不得包 nil cause，得到 %q", got)
		}
	})
}

// TestComputeFailsFastWhenUpsertFails 守住本筆最核心的行為改變。
func TestComputeFailsFastWhenUpsertFails(t *testing.T) {
	cause := errors.New("pq: numeric field overflow (dsn=postgres://u:p@db:5432)")
	ind := &stubIndicatorRepo{upsertErr: cause}
	e := newTestEngine(&stubCandleRepo{candles: enoughCandles(minCandles + 5)}, ind)

	snap, err := e.Compute(context.Background(), "2454", "1m")

	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Upsert 失敗要回 ErrPersistence，得到 %v", err)
	}
	if snap != nil {
		// 回傳非 nil 等於讓呼叫端拿一份沒落盤的 snapshot 繼續用——那正是原本的成因。
		t.Error("落盤失敗時不得回傳 snapshot")
	}
	if !errors.Is(err, cause) {
		t.Error("persistence error 要包住原 cause（只供 log 與分類，不回給 API）")
	}
	if ind.upserted != 1 {
		t.Errorf("Upsert 應該被呼叫一次，實際 %d", ind.upserted)
	}
}

func TestComputeSucceedsWhenUpsertSucceeds(t *testing.T) {
	ind := &stubIndicatorRepo{}
	e := newTestEngine(&stubCandleRepo{candles: enoughCandles(minCandles + 5)}, ind)

	snap, err := e.Compute(context.Background(), "2454", "1m")
	if err != nil {
		t.Fatalf("正常路徑不該失敗：%v", err)
	}
	if snap == nil {
		t.Fatal("正常路徑要回傳 snapshot")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
