package signal

import (
	"context"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// T-088：Withhold（資料延遲）照常算到判重為止，觸發時⛔ 不 reserve、⛔ 不寫 signals、⛔ 不推播；
// 之後正常評估仍送得出去（⛔ 不被自己的 reservation 擋下）。
func TestEngineWithholdDoesNotDeliver(t *testing.T) {
	eng, candleRepo, signalRepo, _ := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK", base))
	var broadcast int
	eng.BroadcastFn = func(string, *store.Signal) { broadcast++ }

	res, err := eng.EvaluateWithOptions(context.Background(), "BRK", "1d", EvaluateOptions{Withhold: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Withheld || res.WithheldSignal == nil || res.Signal != nil || res.SignalGenerated {
		t.Fatalf("得到 %+v", res)
	}
	if rows, _ := signalRepo.GetBySymbol(context.Background(), "BRK", 10); len(rows) != 0 || broadcast != 0 {
		t.Fatalf("⛔ 不該送出：signals %d、推播 %d", len(rows), broadcast)
	}

	res, err = eng.EvaluateWithResult(context.Background(), "BRK", "1d")
	if err != nil || !res.SignalGenerated || res.Suppressed {
		t.Fatalf("正常評估應送出，得到 %+v（err=%v）", res, err)
	}
}

// 觸發但被判重擋下 → Suppressed（⛔ 不跟「沒有觸發」混在一起）。
func TestEngineReportsSuppressedDuplicate(t *testing.T) {
	eng, candleRepo, _, _ := newTestEngine(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedCandles(t, candleRepo, breakoutCandles("BRK", base))

	if res, err := eng.EvaluateWithResult(context.Background(), "BRK", "1d"); err != nil || !res.SignalGenerated {
		t.Fatalf("第一次應送出，得到 %+v（err=%v）", res, err)
	}
	res, err := eng.EvaluateWithResult(context.Background(), "BRK", "1d")
	if err != nil || !res.Suppressed || res.SignalGenerated {
		t.Fatalf("第二次應被判重擋下，得到 %+v（err=%v）", res, err)
	}

	flat, _, _, _ := newTestEngine(t)
	if res, err := flat.EvaluateWithResult(context.Background(), "NONE", "1d"); err == nil && res.Suppressed {
		t.Fatal("沒有資料⛔ 不算判重擋下")
	}
}
