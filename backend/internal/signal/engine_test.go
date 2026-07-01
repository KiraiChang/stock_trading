package signal

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

// newTestEngine 建立接在暫存 sqlite 檔案上的完整 Engine（真正的 migration、
// CandleRepo/IndicatorRepo/SignalRepo），驗證 Evaluate 整條路徑真的能跑通，
// 不只是各自獨立的純函式。測試結束會自動清理暫存檔。
func newTestEngine(t *testing.T) (*Engine, store.CandleRepo, store.SignalRepo) {
	t.Helper()

	tmp, err := os.CreateTemp("", "signal-engine-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	candleRepo := store.NewCandleRepo(db)
	indicatorRepo := store.NewIndicatorRepo(db)
	signalRepo := store.NewSignalRepo(db)
	rdb := store.NewRedis(store.DisabledRedisConfig())
	indEngine := indicator.NewEngine(candleRepo, indicatorRepo, rdb, zap.NewNop())
	sigEngine := NewEngine(candleRepo, signalRepo, rdb, indEngine, zap.NewNop())

	return sigEngine, candleRepo, signalRepo
}

func seedCandles(t *testing.T, repo store.CandleRepo, candles []store.Candle) {
	t.Helper()
	if err := repo.BulkInsert(context.Background(), candles); err != nil {
		t.Fatalf("seed candles failed: %v", err)
	}
}

func TestEngine_Evaluate_InsufficientCandlesReturnsError(t *testing.T) {
	eng, _, _ := newTestEngine(t)

	sig, err := eng.Evaluate(context.Background(), "NODATA", "1d")
	if err == nil {
		t.Fatal("expected an error for a symbol with no candles")
	}
	if sig != nil {
		t.Errorf("expected nil signal on error, got %+v", sig)
	}
}

func TestEngine_Evaluate_FlatDataProducesNoSignal(t *testing.T) {
	eng, candleRepo, signalRepo := newTestEngine(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]store.Candle, 0, 40)
	for i := 0; i < 40; i++ {
		candles = append(candles, store.Candle{
			Symbol: "FLAT", Timeframe: "1d",
			Open: 100, High: 100.5, Low: 99.5, Close: 100,
			Volume: 1000, Timestamp: base.AddDate(0, 0, i),
		})
	}
	seedCandles(t, candleRepo, candles)

	sig, err := eng.Evaluate(context.Background(), "FLAT", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != nil {
		t.Errorf("expected no signal for flat data, got %+v", sig)
	}

	rows, err := signalRepo.GetBySymbol(context.Background(), "FLAT", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no signal rows inserted for flat data, got %d", len(rows))
	}
}

func TestEngine_Evaluate_BreakoutGeneratesAndPersistsSignal(t *testing.T) {
	eng, candleRepo, signalRepo := newTestEngine(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const n = 34 // 多頭 zigzag 走勢，足夠 DetectTrend 判斷出 BULLISH（HH+HL）
	candles := make([]store.Candle, 0, n+1)
	var lastClose float64
	for i := 0; i < n; i++ {
		c := 100 + 0.6*float64(i) + 1.0*math.Sin(float64(i))
		candles = append(candles, store.Candle{
			Symbol: "BRK", Timeframe: "1d",
			Open: c - 0.1, High: c + 0.8, Low: c - 0.8, Close: c,
			Volume: 1000, Timestamp: base.AddDate(0, 0, i),
		})
		lastClose = c
	}
	// 突破K棒：大漲 + 帶量，收盤遠超過前面所有壓力位（35 根，剛好滿足
	// indicator.Compute 需要的 >=35 根門檻）
	breakoutClose := lastClose + 20
	candles = append(candles, store.Candle{
		Symbol: "BRK", Timeframe: "1d",
		Open: lastClose + 0.2, High: breakoutClose + 0.5, Low: lastClose, Close: breakoutClose,
		Volume: 5000, Timestamp: base.AddDate(0, 0, n),
	})
	seedCandles(t, candleRepo, candles)

	var broadcastSymbol string
	var broadcastSignal *store.Signal
	eng.BroadcastFn = func(symbol string, sig *store.Signal) {
		broadcastSymbol = symbol
		broadcastSignal = sig
	}

	sig, err := eng.Evaluate(context.Background(), "BRK", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil {
		t.Fatal("expected a BREAKOUT signal, got nil")
	}
	if sig.SignalType != "BREAKOUT" || sig.Direction != "BUY" {
		t.Errorf("got SignalType=%s Direction=%s, want BREAKOUT/BUY", sig.SignalType, sig.Direction)
	}
	if sig.Trend != string(Bullish) {
		t.Errorf("Trend = %s, want BULLISH", sig.Trend)
	}

	// 確認真的寫進 DB，不只是回傳值
	rows, err := signalRepo.GetBySymbol(context.Background(), "BRK", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 persisted signal, got %d", len(rows))
	}

	// 確認 BroadcastFn 真的被呼叫（WebSocket 推播的掛勾點）
	if broadcastSymbol != "BRK" {
		t.Errorf("BroadcastFn symbol = %q, want BRK", broadcastSymbol)
	}
	if broadcastSignal == nil || broadcastSignal.SignalType != "BREAKOUT" {
		t.Errorf("BroadcastFn signal = %+v, want a BREAKOUT signal", broadcastSignal)
	}
}

func TestEngine_EvaluateAll_ContinuesAfterPerSymbolFailure(t *testing.T) {
	eng, candleRepo, signalRepo := newTestEngine(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]store.Candle, 0, 40)
	for i := 0; i < 40; i++ {
		candles = append(candles, store.Candle{
			Symbol: "OK", Timeframe: "1d",
			Open: 100, High: 100.5, Low: 99.5, Close: 100,
			Volume: 1000, Timestamp: base.AddDate(0, 0, i),
		})
	}
	seedCandles(t, candleRepo, candles)

	// "MISSING" 完全沒有 candles，"OK" 有足夠但平盤資料；EvaluateAll 不應該
	// 因為其中一檔失敗就中斷，兩者都應該被嘗試過
	eng.EvaluateAll(context.Background(), []string{"MISSING", "OK"}, "1d")

	rows, err := signalRepo.GetBySymbol(context.Background(), "OK", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("flat data shouldn't produce a signal, got %d rows", len(rows))
	}
}
