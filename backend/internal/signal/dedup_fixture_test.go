package signal

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

// 這個檔提供「能真的觸發一次訊號」的 fixture，讓 EvaluateWithResult 走完整條路，
// 而不是只測 tryReserveEmission。

const dedupSymbol = "2330"

type dedupCandleRepo struct {
	store.CandleRepo
	candles []store.Candle
}

func (d *dedupCandleRepo) GetLatestN(_ context.Context, _, _ string, n int) ([]store.Candle, error) {
	if n >= len(d.candles) {
		return d.candles, nil
	}
	return d.candles[len(d.candles)-n:], nil
}

type dedupIndicatorRepo struct{ store.IndicatorRepo }

func (dedupIndicatorRepo) Upsert(context.Context, *store.IndicatorSnapshot) error { return nil }

// dedupChipRepo 讓 applyChipWeighting 走「查無籌碼資料」那條——
// 它本來就設計成缺資料不阻塞訊號產生。
type dedupChipRepo struct{ store.ChipScoreRepo }

func (dedupChipRepo) GetLatest(context.Context, string) (*store.ChipScore, error) {
	return nil, sql.ErrNoRows
}

type dedupSignalRepo struct {
	store.SignalRepo
	inserted  []store.Signal
	insertErr error
	getErr    error
}

func (d *dedupSignalRepo) Insert(_ context.Context, s *store.Signal) error {
	if d.insertErr != nil {
		return d.insertErr
	}
	d.inserted = append(d.inserted, *s)
	return nil
}

func (d *dedupSignalRepo) GetBySymbol(_ context.Context, _ string, _ int) ([]store.Signal, error) {
	if d.getErr != nil {
		return nil, d.getErr
	}
	// 最新在前，比照真實 repo。
	out := make([]store.Signal, 0, len(d.inserted))
	for i := len(d.inserted) - 1; i >= 0; i-- {
		out = append(out, d.inserted[i])
	}
	return out, nil
}

// bounceCandles 造一段「支撐反彈」形狀：長期在 100 附近打底，最後一根探到支撐、
// 收在上半部。目的只是讓 Evaluate 真的產生一個訊號，數值本身不是斷言對象。
func bounceCandles() []store.Candle {
	base := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	out := make([]store.Candle, 0, 120)
	for i := 0; i < 119; i++ {
		// 反覆在 100 觸底，讓 CalcSupportResistance 認出 100 是支撐。
		low := 100.0
		if i%3 != 0 {
			low = 101.5
		}
		out = append(out, store.Candle{
			Symbol: dedupSymbol, Timeframe: "1d",
			Open: 103, High: 105, Low: low, Close: 104,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
		})
	}
	// 最後一根：探到 100、收 104.5（上半部）
	out = append(out, store.Candle{
		Symbol: dedupSymbol, Timeframe: "1d",
		Open: 101, High: 105, Low: 100, Close: 104.5,
		Volume: 3000, Timestamp: base.Add(119 * 24 * time.Hour),
	})
	return out
}

func newDedupEngine(t *testing.T, sigRepo store.SignalRepo, redis emissionStore, now time.Time) *Engine {
	t.Helper()
	candles := &dedupCandleRepo{candles: bounceCandles()}
	ind := indicator.NewEngine(candles, dedupIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	e := NewEngine(candles, sigRepo, &store.RedisClient{}, ind, dedupChipRepo{}, zap.NewNop())
	e.SetEmissionStoreForTest(redis)
	e.SetClockForTest(func() time.Time { return now })
	return e
}

// TestBounceFixtureActuallyFires 是上面那些測試的前提檢查。
//
// ⚠️ **fixture 失效時要立刻看得出來**——否則「第二輪沒有訊號」會變成永遠會過的
// 空測試（第一輪本來就沒訊號）。
func TestBounceFixtureActuallyFires(t *testing.T) {
	e := newDedupEngine(t, &dedupSignalRepo{}, newFakeRedis(), time.Now())
	res, err := e.EvaluateWithResult(context.Background(), dedupSymbol, "1d")
	if err != nil {
		t.Fatalf("fixture 不該回錯：%v", err)
	}
	if !res.SignalGenerated {
		t.Fatal("fixture 必須能觸發一次訊號，否則下游的判重測試都是空測試")
	}
}

// TestDegradedSuccessWhenInsertFails 走完整條路，驗 degraded-success 的核心行為：
// signals 寫不進去時**訊號照樣送出**，但結果要標得出來。
func TestDegradedSuccessWhenInsertFails(t *testing.T) {
	ctx := context.Background()
	sigRepo := &dedupSignalRepo{insertErr: errors.New("pq: numeric field overflow")}
	redis := newFakeRedis()
	e := newDedupEngine(t, sigRepo, redis, time.Now())

	broadcasts := 0
	e.BroadcastFn = func(string, *store.Signal) { broadcasts++ }

	res, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatalf("Insert 失敗不該讓 Evaluate 回錯（那是 fail-fast，不是 degraded-success）：%v", err)
	}
	if !res.SignalGenerated {
		t.Fatal("訊號仍應產生")
	}
	if res.DBPersisted {
		t.Error("Insert 失敗時 DBPersisted 必須是 false")
	}
	if !res.QueueEnqueued {
		t.Error("Insert 失敗不該阻止 enqueue")
	}
	if broadcasts != 1 || !res.BroadcastAttempted {
		t.Errorf("Insert 失敗不該阻止 broadcast（broadcasts=%d attempted=%v）", broadcasts, res.BroadcastAttempted)
	}
	if !res.Degraded || len(res.DegradedStages) != 1 || res.DegradedStages[0] != StageSignalPersistFailed {
		t.Errorf("要標成 signal_persist_failed，得到 %v", res.DegradedStages)
	}
}

// TestDBDedupFailureMarksDedupDegraded：判重查詢失敗會 fail-open，
// 那時只剩 reservation 一層——必須看得見。
func TestDBDedupFailureMarksDedupDegraded(t *testing.T) {
	ctx := context.Background()
	sigRepo := &dedupSignalRepo{getErr: errors.New("dial: connection refused")}
	e := newDedupEngine(t, sigRepo, newFakeRedis(), time.Now())

	res, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatalf("判重查詢失敗不該讓 Evaluate 回錯：%v", err)
	}
	if !res.SignalGenerated {
		t.Fatal("fail-open：判重查不到就照樣送出")
	}
	if !res.Degraded {
		t.Fatal("判重降級成單層必須標記")
	}
	found := false
	for _, st := range res.DegradedStages {
		if st == StageDedupDegraded {
			found = true
		}
	}
	if !found {
		t.Errorf("要標成 dedup_degraded，得到 %v", res.DegradedStages)
	}
}

// TestQueueDisabledIsNotDegraded：Redis 設定停用是設定狀態，不是故障。
func TestQueueDisabledIsNotDegraded(t *testing.T) {
	ctx := context.Background()
	redis := newFakeRedis()
	redis.enqueueS = &store.EnqueueOutcome{Status: store.EnqueueDisabled}
	e := newDedupEngine(t, &dedupSignalRepo{}, redis, time.Now())

	res, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatal(err)
	}
	if res.QueueEnqueued {
		t.Error("停用時 queue_enqueued 必須是 false")
	}
	if res.Degraded {
		t.Errorf("Redis 停用不算降級，得到 stages=%v", res.DegradedStages)
	}
}

// TestQueueBackoffIsDegraded：READONLY／退避是非預期失敗。
func TestQueueBackoffIsDegraded(t *testing.T) {
	ctx := context.Background()
	redis := newFakeRedis()
	redis.enqueueS = &store.EnqueueOutcome{Status: store.EnqueueBackoff, Err: store.ErrRedisWriteBackoff}
	e := newDedupEngine(t, &dedupSignalRepo{}, redis, time.Now())

	res, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatal(err)
	}
	if res.QueueEnqueued {
		t.Error("退避時 queue_enqueued 必須是 false")
	}
	if !res.Degraded || res.StageErrors[StageQueueFailed] == nil {
		t.Errorf("要標成 queue_failed 並帶 sentinel，得到 %v", res.DegradedStages)
	}
}
