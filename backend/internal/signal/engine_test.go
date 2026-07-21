package signal

import (
	"context"
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
// CandleRepo/IndicatorRepo/SignalRepo/ChipScoreRepo），驗證 Evaluate 整條
// 路徑真的能跑通，不只是各自獨立的純函式。測試結束會自動清理暫存檔。
func newTestEngine(t *testing.T) (*Engine, store.CandleRepo, store.SignalRepo, store.ChipScoreRepo) {
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
	chipScoreRepo := store.NewChipScoreRepo(db)
	rdb := store.NewRedis(store.DisabledRedisConfig())
	indEngine := indicator.NewEngine(candleRepo, indicatorRepo, rdb, zap.NewNop())
	sigEngine := NewEngine(candleRepo, signalRepo, rdb, indEngine, chipScoreRepo, zap.NewNop())

	return sigEngine, candleRepo, signalRepo, chipScoreRepo
}

func seedCandles(t *testing.T, repo store.CandleRepo, candles []store.Candle) {
	t.Helper()
	if err := repo.BulkInsert(context.Background(), candles); err != nil {
		t.Fatalf("seed candles failed: %v", err)
	}
}

func TestEngine_Evaluate_InsufficientCandlesReturnsError(t *testing.T) {
	eng, _, _, _ := newTestEngine(t)

	sig, err := eng.Evaluate(context.Background(), "NODATA", "1d")
	if err == nil {
		t.Fatal("expected an error for a symbol with no candles")
	}
	if sig != nil {
		t.Errorf("expected nil signal on error, got %+v", sig)
	}
}

func TestEngine_Evaluate_FlatDataProducesNoSignal(t *testing.T) {
	eng, candleRepo, signalRepo, _ := newTestEngine(t)

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
	eng, candleRepo, signalRepo, _ := newTestEngine(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK", base))

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

func breakoutCandles(symbol string, base time.Time) []store.Candle {
	// 兩個 swing high 95 -> 100 與兩個 swing low 79 -> 84 形成 BULLISH。
	// 放量突破 K 收上 100，後續兩根也站穩 100，滿足 BREAKOUT 確認窗。
	highs := []float64{
		81, 83, 85, 87, 89, 91, 93, 95, 93, 92, 91, 90, 88, 86, 84, 83, 85,
		87, 89, 92, 100, 96, 94, 92, 90, 88, 90, 92, 94, 96, 98, 99, 99, 99,
	}
	candles := make([]store.Candle, 0, len(highs)+1)
	for i, high := range highs {
		low := high - 4
		close := high - 1
		candles = append(candles, store.Candle{
			Symbol: symbol, Timeframe: "1d",
			Open: close - 0.1, High: high, Low: low, Close: close,
			Volume: 1000, Timestamp: base.AddDate(0, 0, i),
		})
	}
	candles = append(candles, store.Candle{
		Symbol: symbol, Timeframe: "1d",
		Open: 99, High: 106, Low: 98, Close: 105,
		Volume: 5000, Timestamp: base.AddDate(0, 0, len(highs)),
	})
	candles = append(candles, store.Candle{
		Symbol: symbol, Timeframe: "1d",
		Open: 105, High: 108, Low: 103, Close: 106,
		Volume: 1200, Timestamp: base.AddDate(0, 0, len(highs)+1),
	})
	candles = append(candles, store.Candle{
		Symbol: symbol, Timeframe: "1d",
		Open: 106, High: 109, Low: 104, Close: 107,
		Volume: 1100, Timestamp: base.AddDate(0, 0, len(highs)+2),
	})
	return candles
}

func TestEngine_Evaluate_ChipBullishBoostsBreakoutStrength(t *testing.T) {
	eng, candleRepo, signalRepo, chipScoreRepo := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK_BULL", base))

	if err := chipScoreRepo.Upsert(context.Background(), &store.ChipScore{
		Symbol: "BRK_BULL", TradeDate: base, Signal: "BULLISH", TotalScore: 50,
	}); err != nil {
		t.Fatalf("seed chip score failed: %v", err)
	}

	sig, err := eng.Evaluate(context.Background(), "BRK_BULL", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil || sig.SignalType != "BREAKOUT" {
		t.Fatalf("expected a BREAKOUT signal, got %+v", sig)
	}
	if sig.Strength != baseStrength*chipStrengthBoost {
		t.Errorf("Strength = %v, want %v (1.0 * chipStrengthBoost)", sig.Strength, chipStrengthBoost)
	}
	if !sig.ChipSignal.Valid || sig.ChipSignal.String != "BULLISH" {
		t.Errorf("ChipSignal = %+v, want BULLISH", sig.ChipSignal)
	}

	rows, err := signalRepo.GetBySymbol(context.Background(), "BRK_BULL", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Strength != chipStrengthBoost {
		t.Errorf("expected persisted signal with Strength=%v, got %+v", chipStrengthBoost, rows[0])
	}
}

func TestEngine_Evaluate_ChipBearishReducesBreakoutStrength(t *testing.T) {
	eng, candleRepo, _, chipScoreRepo := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK_BEAR", base))

	if err := chipScoreRepo.Upsert(context.Background(), &store.ChipScore{
		Symbol: "BRK_BEAR", TradeDate: base, Signal: "BEARISH", TotalScore: -50,
	}); err != nil {
		t.Fatalf("seed chip score failed: %v", err)
	}

	sig, err := eng.Evaluate(context.Background(), "BRK_BEAR", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil || sig.SignalType != "BREAKOUT" {
		t.Fatalf("expected a BREAKOUT signal, got %+v", sig)
	}
	if sig.Strength != chipStrengthReduce {
		t.Errorf("Strength = %v, want %v (1.0 * chipStrengthReduce)", sig.Strength, chipStrengthReduce)
	}
	if !sig.ChipSignal.Valid || sig.ChipSignal.String != "BEARISH" {
		t.Errorf("ChipSignal = %+v, want BEARISH", sig.ChipSignal)
	}
}

func TestEngine_Evaluate_NoChipDataKeepsDefaultStrength(t *testing.T) {
	eng, candleRepo, _, _ := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK_NOCHIP", base))

	sig, err := eng.Evaluate(context.Background(), "BRK_NOCHIP", "1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.Strength != 1.0 {
		t.Errorf("Strength = %v, want 1.0 (no chip data available)", sig.Strength)
	}
	if sig.ChipSignal.Valid {
		t.Errorf("ChipSignal = %+v, want invalid/empty (no chip data)", sig.ChipSignal)
	}
}

func TestEngine_Evaluate_SuppressesDuplicateSignalWithinCooldown(t *testing.T) {
	eng, candleRepo, signalRepo, _ := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK_DEDUP", base))

	broadcasts := 0
	eng.BroadcastFn = func(symbol string, sig *store.Signal) {
		broadcasts++
	}

	first, err := eng.Evaluate(context.Background(), "BRK_DEDUP", "1d")
	if err != nil {
		t.Fatalf("first evaluate failed: %v", err)
	}
	if first == nil || first.SignalType != "BREAKOUT" {
		t.Fatalf("expected first BREAKOUT signal, got %+v", first)
	}

	second, err := eng.Evaluate(context.Background(), "BRK_DEDUP", "1d")
	if err != nil {
		t.Fatalf("second evaluate failed: %v", err)
	}
	if second != nil {
		t.Fatalf("expected duplicate signal to be suppressed, got %+v", second)
	}

	rows, err := signalRepo.GetBySymbol(context.Background(), "BRK_DEDUP", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected only 1 persisted signal after duplicate suppression, got %d", len(rows))
	}
	if broadcasts != 1 {
		t.Errorf("BroadcastFn calls = %d, want 1", broadcasts)
	}
}

func TestEngine_ShouldSuppressDuplicate_AllowsSignalOutsideCooldown(t *testing.T) {
	eng, _, signalRepo, _ := newTestEngine(t)
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	if err := signalRepo.Insert(context.Background(), &store.Signal{
		Symbol:     "BRK_COOLDOWN",
		SignalType: "BREAKOUT",
		Direction:  "BUY",
		Price:      105,
		Resistance: 100,
		Timestamp:  ts.Add(-signalCooldown),
	}); err != nil {
		t.Fatalf("seed previous signal failed: %v", err)
	}

	suppress, err := eng.shouldSuppressDuplicate(context.Background(), "BRK_COOLDOWN", &store.Signal{
		Symbol:     "BRK_COOLDOWN",
		SignalType: "BREAKOUT",
		Direction:  "BUY",
		Price:      106,
		Resistance: 100,
		Timestamp:  ts,
	})
	if err != nil {
		t.Fatalf("duplicate check failed: %v", err)
	}
	if suppress {
		t.Fatal("expected signal at cooldown boundary to be allowed")
	}
}

// baseStrength 代表加權前的基準強度 1.0（CheckBreakout/CheckSupportBounce
// 設定的預設值），只是讓斷言讀起來像「1.0 * chipStrengthBoost」而不是裸
// 數字，方便跟 applyChipWeighting 的規則說明對照。
const baseStrength = 1.0

func TestEngine_EvaluateAll_ContinuesAfterPerSymbolFailure(t *testing.T) {
	eng, candleRepo, signalRepo, _ := newTestEngine(t)

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
