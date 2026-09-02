package signal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// 這一組守的是 signal emission 的並行契約（見 docs/architecture.md「寫入失敗的一致性契約」）。

// ── 可控 Redis stub ───────────────────────────────────────────────
//
// 共用同一個 keys map 就等於共用同一個 Redis——兩個獨立的 Engine 指向同一份，
// 可以模擬多 instance。

type fakeRedis struct {
	mu       sync.Mutex
	keys     map[string]string
	reserveS *store.ReserveOutcome // 非 nil 時強制回這個結果
	deleteS  *store.DeleteOutcome
	enqueueS *store.EnqueueOutcome
	calls    int
}

func newFakeRedis() *fakeRedis { return &fakeRedis{keys: map[string]string{}} }

func (f *fakeRedis) ReserveEmission(_ context.Context, key, token string, _ time.Duration) store.ReserveOutcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.reserveS != nil {
		return *f.reserveS
	}
	if _, ok := f.keys[key]; ok {
		return store.ReserveOutcome{Status: store.ReserveAlreadyExists}
	}
	f.keys[key] = token
	return store.ReserveOutcome{Status: store.ReserveReserved}
}

func (f *fakeRedis) ReleaseEmission(_ context.Context, key, token string) store.DeleteOutcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteS != nil {
		return *f.deleteS
	}
	if v, ok := f.keys[key]; ok && v == token {
		delete(f.keys, key)
		return store.DeleteOutcome{Status: store.DeleteDeleted}
	}
	return store.DeleteOutcome{Status: store.DeleteNotOwner}
}

func (f *fakeRedis) EnqueueSignal(context.Context, string, interface{}) store.EnqueueOutcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.enqueueS != nil {
		return *f.enqueueS
	}
	return store.EnqueueOutcome{Status: store.EnqueueEnqueued}
}

func (f *fakeRedis) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newGuardEngine(redis emissionStore, now func() time.Time) *Engine {
	return &Engine{emission: redis, now: now, localReservations: newLocalReservations()}
}

// ── 5-1 canonical identity ───────────────────────────────────────

func TestSignalIdentityKeyQuantisesPriceAndPicksFieldByType(t *testing.T) {
	t.Run("量化到 1e-6，邊界內視為同一個", func(t *testing.T) {
		a := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.0000001}
		b := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.0000002}
		if signalIdentityKey(a) != signalIdentityKey(b) {
			t.Errorf("1e-6 內應視為同一個：%q vs %q", signalIdentityKey(a), signalIdentityKey(b))
		}
	})
	t.Run("差異大於量化尺度就是不同訊號", func(t *testing.T) {
		a := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.00}
		b := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.01}
		if signalIdentityKey(a) == signalIdentityKey(b) {
			t.Error("不同價位不該產生相同 identity")
		}
	})
	t.Run("依型別選欄", func(t *testing.T) {
		breakout := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 105, Support: 90}
		if got := signalIdentityKey(breakout); got != "2330|BREAKOUT|BUY|105.000000" {
			t.Errorf("BREAKOUT 應取 Resistance，得到 %q", got)
		}
		bounce := &store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Direction: "BUY", Resistance: 105, Support: 90}
		if got := signalIdentityKey(bounce); got != "2330|SUPPORT_BOUNCE|BUY|90.000000" {
			t.Errorf("SUPPORT_BOUNCE 應取 Support，得到 %q", got)
		}
		other := &store.Signal{Symbol: "2330", SignalType: "VOLUME_SPIKE", Direction: "BUY", Resistance: 105, Support: 90}
		if got := signalIdentityKey(other); got != "2330|VOLUME_SPIKE|BUY" {
			t.Errorf("其餘型別不帶價位，得到 %q", got)
		}
	})
	t.Run("DB 判重與 Redis key 用同一個定義", func(t *testing.T) {
		prev := store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.0000001}
		next := &store.Signal{Symbol: "2330", SignalType: "BREAKOUT", Direction: "BUY", Resistance: 100.0000002}
		if !sameSignalIdentity(prev, next) {
			t.Error("sameSignalIdentity 必須與 signalIdentityKey 同調")
		}
	})
}

// ── 5-2 Redis 不可用時的 local 競爭 ───────────────────────────────

func TestLocalReserveIsAtomicUnderConcurrency(t *testing.T) {
	now := time.Now()
	redis := newFakeRedis()
	redis.reserveS = &store.ReserveOutcome{Status: store.ReserveDisabled} // Redis 停用
	e := newGuardEngine(redis, func() time.Time { return now })

	const n = 32
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, ok := e.tryReserveEmission(context.Background(), "2330|BREAKOUT|BUY|100.000000", &EvaluateResult{}); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 1 {
		t.Errorf("Redis 不可用時只能有一個 goroutine 取得保留，實際 %d", allowed)
	}
}

// ── 5-3 Redis 恢復後仍被未到期的 local reservation 抑制 ───────────

func TestRedisRecoveryDoesNotBypassLocalReservation(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	redis := newFakeRedis()
	redis.reserveS = &store.ReserveOutcome{Status: store.ReserveDisabled}
	e := newGuardEngine(redis, clock)
	id := "2330|BREAKOUT|BUY|100.000000"

	if _, ok := e.tryReserveEmission(context.Background(), id, &EvaluateResult{}); !ok {
		t.Fatal("第一次應該放行")
	}
	// Redis 恢復——但它那邊沒有這筆 reservation（故障期間沒寫進去）。
	redis.reserveS = nil
	now = now.Add(signalCooldown / 2) // 仍在 cooldown 內

	if _, ok := e.tryReserveEmission(context.Background(), id, &EvaluateResult{}); ok {
		t.Error("cooldown 內不得因 Redis 恢復就繞過 local reservation")
	}
}

// ── 5-3b Redis 已有 reservation 時要回滾自己的 local token ────────

func TestRedisAlreadyExistsRollsBackLocalToken(t *testing.T) {
	now := time.Now()
	redis := newFakeRedis()
	redis.reserveS = &store.ReserveOutcome{Status: store.ReserveAlreadyExists}
	e := newGuardEngine(redis, func() time.Time { return now })

	if _, ok := e.tryReserveEmission(context.Background(), "id", &EvaluateResult{}); ok {
		t.Fatal("Redis 說已存在時應抑制")
	}
	if got := e.localReservations.size(); got != 0 {
		t.Errorf("必須回滾自己的 local token，否則會留下孤兒 reservation；剩 %d 筆", got)
	}
}

// ── 5-3c 兩個獨立 local store 共用 Redis：只有一個放行 ────────────

func TestTwoInstancesSharingRedisAllowOnlyOne(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	shared := newFakeRedis() // 同一個 Redis
	a := newGuardEngine(shared, clock)
	b := newGuardEngine(shared, clock) // 各自獨立的 local store

	_, okA := a.tryReserveEmission(context.Background(), "id", &EvaluateResult{})
	_, okB := b.tryReserveEmission(context.Background(), "id", &EvaluateResult{})

	if okA == okB {
		t.Errorf("跨 instance 仲裁由 Redis SET NX 負責，只能有一個放行（A=%v B=%v）", okA, okB)
	}
}

// ── 5-3d 同一 Engine 的第二次在第 2 步就短路，不呼叫 Redis ────────

func TestSecondGoroutineShortCircuitsBeforeRedis(t *testing.T) {
	now := time.Now()
	redis := newFakeRedis()
	e := newGuardEngine(redis, func() time.Time { return now })

	e.tryReserveEmission(context.Background(), "id", &EvaluateResult{})
	before := redis.callCount()
	e.tryReserveEmission(context.Background(), "id", &EvaluateResult{})

	if redis.callCount() != before {
		t.Error("local 已擋下時不該再呼叫 Redis（七步流程的第 2 步）")
	}
}

// ── 5-3e / 5-3f local map 的有界清理 ─────────────────────────────

func TestLocalSweepRemovesExpiredButKeepsLive(t *testing.T) {
	now := time.Now()
	l := newLocalReservations()

	l.reserve("old", "t1", time.Minute, now)
	// 操作**其他** identity——舊 key 之後再也不會被讀到，惰性過期清不掉它。
	later := now.Add(2 * time.Minute)
	l.reserve("new", "t2", time.Hour, later)

	if got := l.size(); got != 1 {
		t.Errorf("過期項目應被整掃清除（即使只操作其他 identity），剩 %d 筆", got)
	}

	t.Run("未過期的 reservation 不得被清掉", func(t *testing.T) {
		l2 := newLocalReservations()
		base := time.Now()
		// TTL 給得比整個測試的時間跨度長，這樣「還在不在」只取決於清理策略本身。
		const liveTTL = 24 * time.Hour
		l2.reserve("live", "tok", liveTTL, base)

		// 大量新 identity 湧入，時間持續前進以反覆觸發整掃。
		for i := 0; i < 200; i++ {
			at := base.Add(time.Duration(i) * time.Minute)
			l2.reserve(fmt.Sprintf("flood-%d", i), "t", liveTTL, at)
		}

		// 仍在 TTL 內，所以必須被擋下——被放行就代表清理誤刪了未過期的項目。
		if l2.reserve("live", "other", liveTTL, base.Add(200*time.Minute)) {
			t.Error("未過期的 reservation 被清掉了——那會讓同一 identity 在 cooldown 內再次放行")
		}
	})
}

// ── 5-4 compare-and-delete 不得刪掉別人的 token ──────────────────

func TestLocalReleaseIsCompareAndDelete(t *testing.T) {
	now := time.Now()
	l := newLocalReservations()
	l.reserve("k", "mine", time.Hour, now)

	l.release("k", "someone-else") // 舊請求想釋放
	if l.size() != 1 {
		t.Error("不得刪掉不同 token 的 reservation")
	}
	l.release("k", "mine")
	if l.size() != 0 {
		t.Error("自己的 token 應該刪得掉")
	}
}

// TestReleaseDoesNotDeleteReplacementToken 是 5-4 的完整版：
// **要斷言「替代 token 仍在」**，而不只是「回了 not_owner」。
//
// 情境：舊請求 A 取得保留 → 過期 → 新請求 B 取得同一個 key 的保留 →
// A 這時才來釋放。若用裸刪除，A 會刪掉 B 的保留，等於自己打開重複推送的破口。
func TestReleaseDoesNotDeleteReplacementToken(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	redis := newFakeRedis()
	e := newGuardEngine(redis, func() time.Time { return now })
	const id = "2330|BREAKOUT|BUY|100.000000"
	key := emissionKey(id)

	oldHeld, ok := e.tryReserveEmission(ctx, id, &EvaluateResult{})
	if !ok {
		t.Fatal("A 應取得保留")
	}

	// A 過期，B 取得同一個 key（local 與 Redis 都換成 B 的 token）
	now = now.Add(signalCooldown + time.Second)
	e.localReservations.release(key, oldHeld.token)
	redis.mu.Lock()
	delete(redis.keys, key)
	redis.mu.Unlock()

	newHeld, ok := e.tryReserveEmission(ctx, id, &EvaluateResult{})
	if !ok {
		t.Fatal("B 應取得保留")
	}
	if newHeld.token == oldHeld.token {
		t.Fatal("兩次 token 必須不同（UUID）")
	}

	// A 這時才釋放——不得動到 B 的保留。
	e.releaseEmission(ctx, oldHeld, &EvaluateResult{})

	redis.mu.Lock()
	got, present := redis.keys[key]
	redis.mu.Unlock()
	if !present || got != newHeld.token {
		t.Errorf("Redis 端的替代 token 被刪掉了（present=%v value=%q want %q）", present, got, newHeld.token)
	}
	if e.localReservations.size() != 1 {
		t.Error("local 端的替代 token 也被刪掉了")
	}
	// 而且 B 的保留仍然有效：同一個 identity 這時應被抑制。
	if _, allowed := e.tryReserveEmission(ctx, id, &EvaluateResult{}); allowed {
		t.Error("B 的保留應仍然擋得住後續請求")
	}
}

// ── 5-11 / 5-12 / 5-13 reservation 與 release 的降級分類 ─────────

func TestReservationDegradedClassification(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name         string
		outcome      store.ReserveOutcome
		wantAllowed  bool
		wantDegraded bool
	}{
		{"停用 → 放行且不算降級", store.ReserveOutcome{Status: store.ReserveDisabled}, true, false},
		{"backoff → 放行且算降級", store.ReserveOutcome{Status: store.ReserveBackoff, Err: store.ErrRedisWriteBackoff}, true, true},
		{"error → 放行且算降級", store.ReserveOutcome{Status: store.ReserveError, Err: errors.New("boom")}, true, true},
		{"已存在 → 抑制", store.ReserveOutcome{Status: store.ReserveAlreadyExists}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redis := newFakeRedis()
			redis.reserveS = &tt.outcome
			e := newGuardEngine(redis, func() time.Time { return now })
			res := &EvaluateResult{}

			_, allowed := e.tryReserveEmission(context.Background(), "id", res)
			if allowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v", allowed, tt.wantAllowed)
			}
			if res.Degraded != tt.wantDegraded {
				t.Errorf("degraded = %v, want %v（stages=%v）", res.Degraded, tt.wantDegraded, res.DegradedStages)
			}
			if tt.wantDegraded && len(res.DegradedStages) > 0 && res.DegradedStages[0] != StageDedupDegraded {
				t.Errorf("降級 stage 應為 dedup_degraded，得到 %v", res.DegradedStages)
			}
		})
	}
}

func TestReleaseBackoffMarksDegradedAndStillFreesLocal(t *testing.T) {
	now := time.Now()
	redis := newFakeRedis()
	e := newGuardEngine(redis, func() time.Time { return now })
	res := &EvaluateResult{}

	held, ok := e.tryReserveEmission(context.Background(), "id", res)
	if !ok {
		t.Fatal("應先取得保留")
	}
	redis.deleteS = &store.DeleteOutcome{Status: store.DeleteBackoff, Err: store.ErrRedisWriteBackoff}
	e.releaseEmission(context.Background(), held, res)

	if !res.Degraded {
		t.Error("release 遇到 backoff 要標 dedup_degraded")
	}
	// local 仍要釋放：Redis 那筆反正會過期，local 不放會把自己多擋一個 cooldown。
	if got := e.localReservations.size(); got != 0 {
		t.Errorf("local token 仍要釋放，剩 %d 筆", got)
	}
}

// ── 5-5：DB 已落盤但兩個通道都沒送出 ─────────────────────────────
//
// **明示接受的取捨**：reservation 會被釋放，但下一輪仍會被 DB 判重擋下
// （讀得到剛寫進去那列、又在 cooldown 內），所以**不會自動重送**。
// 要保證投遞得另立 outbox／retry，那會把本筆擴張成「投遞保證」。
func TestReservationReleasedButDBDedupStillSuppressesNextRound(t *testing.T) {
	now := time.Now()
	redis := newFakeRedis()
	e := newGuardEngine(redis, func() time.Time { return now })
	res := &EvaluateResult{}

	held, ok := e.tryReserveEmission(context.Background(), "id", res)
	if !ok {
		t.Fatal("第一輪應放行")
	}
	// queue 非預期失敗、也沒有 BroadcastFn → 兩個通道都沒送出 → 釋放
	res.markDegraded(StageQueueFailed, store.ErrRedisWriteBackoff)
	e.releaseEmission(context.Background(), held, res)

	if e.localReservations.size() != 0 {
		t.Error("兩個通道都沒送出時應釋放 reservation")
	}
	// 釋放後 reservation 這一層確實會放行——真正擋住重送的是 DB 判重，
	// 它在 Evaluate 裡於 reservation 之前執行。下面那支測的就是那一半。
	if _, ok := e.tryReserveEmission(context.Background(), "id", &EvaluateResult{}); !ok {
		t.Error("釋放後 reservation 層應該放行（重送與否由 DB 判重決定）")
	}
}

// TestDBDedupSuppressesEvenAfterReservationReleased 走**真正的 EvaluateWithResult**，
// 證明「DB 已落盤 → 下一輪不會重送」——上一支只驗到 reservation 層會放行。
func TestDBDedupSuppressesEvenAfterReservationReleased(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	sigRepo := &dedupSignalRepo{}
	e := newDedupEngine(t, sigRepo, newFakeRedis(), now)

	first, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatalf("第一輪不該失敗：%v", err)
	}
	if !first.SignalGenerated {
		t.Fatalf("第一輪應產生訊號（fixture 設計如此）")
	}
	if !first.DBPersisted || len(sigRepo.inserted) != 1 {
		t.Fatalf("第一輪應落盤，DBPersisted=%v inserted=%d", first.DBPersisted, len(sigRepo.inserted))
	}

	// 把 reservation 清掉，模擬「兩個通道都沒送出而釋放」之後的狀態。
	e.localReservations = newLocalReservations()
	e.emission = newFakeRedis()

	second, err := e.EvaluateWithResult(ctx, dedupSymbol, "1d")
	if err != nil {
		t.Fatalf("第二輪不該失敗：%v", err)
	}
	if second.SignalGenerated {
		t.Error("DB 已落盤且在 cooldown 內，第二輪必須被 DB 判重抑制——不會自動重送")
	}
	if len(sigRepo.inserted) != 1 {
		t.Errorf("不得重複寫入，實際 %d 筆", len(sigRepo.inserted))
	}
}

// ── EvaluateResult 的降級聚合 ────────────────────────────────────

func TestEvaluateResultDegradedStagesAreStableAndDeduplicated(t *testing.T) {
	res := &EvaluateResult{}
	res.markDegraded(StageQueueFailed, errors.New("a"))
	res.markDegraded(StageDedupDegraded, errors.New("b"))
	res.markDegraded(StageQueueFailed, errors.New("c")) // 重複

	if len(res.DegradedStages) != 2 {
		t.Fatalf("同一 stage 只記一次，得到 %v", res.DegradedStages)
	}
	if res.DegradedStages[0] != StageDedupDegraded || res.DegradedStages[1] != StageQueueFailed {
		t.Errorf("stage 應排序以求穩定，得到 %v", res.DegradedStages)
	}
	if got := res.StageErrors[StageQueueFailed]; got == nil || got.Error() != "a" {
		t.Errorf("同一 stage 應保留第一個 cause，得到 %v", got)
	}
	stage, cause := res.FirstStageError()
	if stage != StageDedupDegraded || cause == nil || cause.Error() != "b" {
		t.Errorf("FirstStageError = %v/%v", stage, cause)
	}
}
