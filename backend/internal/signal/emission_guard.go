package signal

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trading/backend/internal/store"
)

// emissionStore 是 signal 需要的**最小** Redis 介面，只含本筆用到的三個操作。
//
// ⛔ **不要把整個 RedisClient 抽成介面**——那會把無關方法一起拖進來。
// 介面定義在**消費端（signal）**而不是 store，依賴反轉，與專案既有的 repo 慣例一致。
type emissionStore interface {
	ReserveEmission(ctx context.Context, key, token string, ttl time.Duration) store.ReserveOutcome
	ReleaseEmission(ctx context.Context, key, token string) store.DeleteOutcome
	EnqueueSignal(ctx context.Context, key string, value interface{}) store.EnqueueOutcome
}

// localSweepInterval 是過期項目整掃的最小間隔。
//
// ⚠️ **只清除已過期項目，不做容量上限淘汰**：Redis 停用或退避時 local map 是**唯一**
// 判重層，淘汰一個尚未過期的 reservation 會讓同一 identity 在 cooldown 內再次放行,
// 直接違反 dedup 契約——而且正好發生在保證最弱的時候。
//
// 成本：整掃仍由**某一次** reserve 支付 O(n)，只是**頻率有上界**。不需要背景 goroutine。
const localSweepInterval = time.Minute

// localReservations 是 Redis 不可用時的後備判重層。
//
// ⛔ **per-instance 且重啟即失憶**，不能宣稱強一致性。這是刻意接受的降級。
type localReservations struct {
	mu        sync.Mutex
	entries   map[string]localEntry
	lastSweep time.Time
}

type localEntry struct {
	token     string
	expiresAt time.Time
}

func newLocalReservations() *localReservations {
	return &localReservations{entries: map[string]localEntry{}}
}

// reserve 是**單一原子操作**：查與寫在同一個 critical section 內完成。
//
// ⛔ 介面刻意不暴露分開的 get / set——寫成「先查、放開鎖、再寫」時，
// 兩個 goroutine 會同時查無資料而一起放行，那正是這層要擋的東西。
func (l *localReservations) reserve(key, token string, ttl time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweepLocked(now)

	if e, ok := l.entries[key]; ok && now.Before(e.expiresAt) {
		return false
	}
	l.entries[key] = localEntry{token: token, expiresAt: now.Add(ttl)}
	return true
}

// release 是 compare-and-delete：只在值仍等於自己的 token 時才刪。
//
// ⛔ 不能用裸刪除——舊請求的釋放會刪掉後來那次已經成立的新 reservation。
func (l *localReservations) release(key, token string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[key]; ok && e.token == token {
		delete(l.entries, key)
	}
}

// sweepLocked 清除已過期項目。呼叫端必須已持有 mu。
//
// **不能只做「讀到同一個 key 才刪」的惰性過期**：identity 帶價位，價位一變就是新 key，
// 舊 key 再也不會被讀到，於是永遠不會被清掉——長時間執行下 map 會持續成長。
func (l *localReservations) sweepLocked(now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < localSweepInterval {
		return
	}
	l.lastSweep = now
	for k, e := range l.entries {
		if !now.Before(e.expiresAt) {
			delete(l.entries, k)
		}
	}
}

func (l *localReservations) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// reservation 是一次成功的保留，供後續釋放使用。
type reservation struct {
	key   string
	token string
}

// tryReserveEmission 執行判重的七步流程（設計見 docs/architecture.md「寫入失敗的一致性契約」）。
//
//	1  以 UUID token 原子取得 local reservation
//	2  local 沒取得              → 抑制（不必問 Redis）
//	3  Redis reserved            → 兩邊都保留 → 放行
//	4  Redis already_exists      → compare-and-delete 回滾自己的 local token → 抑制
//	5  Redis disabled            → 保留 local → 放行（不算 degraded：設定狀態不是故障）
//	6  Redis backoff             → 保留 local → 放行 ＋ dedup_degraded
//	7  Redis error               → 保留 local → 放行 ＋ dedup_degraded
//
// **先 local 後 Redis** 讓 mutex 不必跨越 Redis 網路呼叫；Redis 抖動由第 4 步的回滾處理。
func (e *Engine) tryReserveEmission(ctx context.Context, identity string, res *EvaluateResult) (*reservation, bool) {
	key := emissionKey(identity)
	token := uuid.NewString()
	now := e.now()

	if !e.localReservations.reserve(key, token, signalCooldown, now) {
		return nil, false // 步驟 2
	}
	held := &reservation{key: key, token: token}

	outcome := e.emission.ReserveEmission(ctx, key, token, signalCooldown)
	switch outcome.Status {
	case store.ReserveReserved: // 步驟 3
		return held, true
	case store.ReserveAlreadyExists: // 步驟 4
		e.localReservations.release(key, token)
		return nil, false
	case store.ReserveDisabled: // 步驟 5
		return held, true
	default: // 步驟 6、7
		res.markDegraded(StageDedupDegraded, outcome.Err)
		return held, true
	}
}

// releaseEmission 釋放兩邊的保留。
//
// ⚠️ **backoff / error 時 local token 仍要釋放**：Redis 那筆反正會過期、此刻也刪不掉；
// local 若不釋放，本 instance 會被自己多擋一個 cooldown，而那個 reservation 對應的是
// 一次「連嘗試都沒送出去」的訊號。寧可讓本 instance 下一輪能重試。
func (e *Engine) releaseEmission(ctx context.Context, held *reservation, res *EvaluateResult) {
	if held == nil {
		return
	}
	outcome := e.emission.ReleaseEmission(ctx, held.key, held.token)
	switch outcome.Status {
	case store.DeleteBackoff, store.DeleteError:
		res.markDegraded(StageDedupDegraded, outcome.Err)
	}
	e.localReservations.release(held.key, held.token)
}
