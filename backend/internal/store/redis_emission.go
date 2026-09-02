package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// 這個檔案是 docs/issue.md I-102 第 2 段用的三個 Redis 操作。
//
// **為什麼不沿用既有方法**：既有的 HSet / SAdd / Set / LPush 把三種完全不同的狀態
// 折成同一個 nil——
//
//   - rdb == nil（設定停用）           → return nil
//   - skipWrite()（READONLY 退避期間）  → return nil
//   - 實際收到 READONLY                → handleWriteErr 設完退避也 return nil
//
// 於是 `err == nil` **既不代表寫成功，也分不出是哪一種跳過**。而 I-102 的裁決要求
// 「設定停用不算 degraded、READONLY 算 degraded」——不分開就落實不了。
//
// ⛔ **這三個操作只回封閉的 status，不要改回用 err == nil 推導。**
// ⚠️ 範圍**僅限這三個**：既有方法維持原契約，不在本筆範圍內。

// ErrRedisWriteBackoff 是 backoff 狀態的 sentinel。
//
// ⚠️ **backoff 沒有底層 error 可帶**（skipWrite 只回布林），沒有 sentinel 的話
// 錯誤分類器產不出 readonly，stage_errors 也沒有東西可存。
var ErrRedisWriteBackoff = errors.New("redis write backoff (readonly)")

// ReserveStatus / EnqueueStatus / DeleteStatus 三個值域**刻意不同**，不共用型別——
// 「已存在」與「不是擁有者」是不同的事實，混在一起呼叫端就分不出該做什麼。
type ReserveStatus int

const (
	ReserveReserved ReserveStatus = iota
	ReserveAlreadyExists
	ReserveDisabled
	ReserveBackoff
	ReserveError
)

type EnqueueStatus int

const (
	EnqueueEnqueued EnqueueStatus = iota
	EnqueueDisabled
	EnqueueBackoff
	EnqueueError
)

type DeleteStatus int

const (
	DeleteDeleted DeleteStatus = iota
	DeleteNotOwner
	DeleteDisabled
	DeleteBackoff
	DeleteError
)

// Outcome 的不變量（三個操作一體適用，測試會逐條斷言）：
//
//	backoff                                  → Err 必須是 ErrRedisWriteBackoff
//	error                                    → Err 必須非 nil（原始 cause）
//	reserved/enqueued/already_exists/disabled/deleted/not_owner → Err == nil
//
// Err **只供內部觀測**（log 與錯誤分類），⛔ 不得寫進 job_runs.error 或 API 回應。
type ReserveOutcome struct {
	Status ReserveStatus
	Err    error
}

type EnqueueOutcome struct {
	Status EnqueueStatus
	Err    error
}

type DeleteOutcome struct {
	Status DeleteStatus
	Err    error
}

// ReserveEmission 以 SET NX PX 取得一個有期限的保留。
//
// **用 NX 而不是先 GET 再 SET**：多 instance 才不會同時放行。
// value 是 UUID token，釋放時要靠它做 compare-and-delete。
func (r *RedisClient) ReserveEmission(ctx context.Context, key, token string, ttl time.Duration) ReserveOutcome {
	if r.rdb == nil {
		return ReserveOutcome{Status: ReserveDisabled}
	}
	if r.skipWrite() {
		return ReserveOutcome{Status: ReserveBackoff, Err: ErrRedisWriteBackoff}
	}
	ok, err := r.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		// **第一次收到 READONLY 也要回 backoff**，不能像 handleWriteErr 那樣吃成成功。
		if r.classifyWriteErr(err) {
			return ReserveOutcome{Status: ReserveBackoff, Err: ErrRedisWriteBackoff}
		}
		return ReserveOutcome{Status: ReserveError, Err: err}
	}
	if !ok {
		return ReserveOutcome{Status: ReserveAlreadyExists}
	}
	return ReserveOutcome{Status: ReserveReserved}
}

// releaseScript 是原子的 compare-and-delete。
//
// ⛔ **不能用「GET 比對後再 DEL」**：兩個指令之間 reservation 可能已經過期並被別人
// 重新取得，於是照樣刪掉別人的。Lua 在 Redis 端是單一原子執行單位。
var releaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

// ReleaseEmission 只在值仍等於 token 時才刪。
//
// 回 DeleteNotOwner 代表那筆已經是別人的 reservation——**那是正常情形**，
// 本來就不該刪，呼叫端不必視為失敗。
func (r *RedisClient) ReleaseEmission(ctx context.Context, key, token string) DeleteOutcome {
	if r.rdb == nil {
		return DeleteOutcome{Status: DeleteDisabled}
	}
	if r.skipWrite() {
		return DeleteOutcome{Status: DeleteBackoff, Err: ErrRedisWriteBackoff}
	}
	n, err := releaseScript.Run(ctx, r.rdb, []string{key}, token).Int64()
	if err != nil {
		if r.classifyWriteErr(err) {
			return DeleteOutcome{Status: DeleteBackoff, Err: ErrRedisWriteBackoff}
		}
		return DeleteOutcome{Status: DeleteError, Err: err}
	}
	if n == 0 {
		return DeleteOutcome{Status: DeleteNotOwner}
	}
	return DeleteOutcome{Status: DeleteDeleted}
}

// EnqueueSignal 是 LPush 的明確語意版本。
//
// 既有的 LPush 保留不動（其他呼叫端仍在用），本筆的 signal queue 改走這一支：
// queue_enqueued 必須只在**真的寫進去**時為 true。
func (r *RedisClient) EnqueueSignal(ctx context.Context, key string, value interface{}) EnqueueOutcome {
	if r.rdb == nil {
		return EnqueueOutcome{Status: EnqueueDisabled}
	}
	b, err := json.Marshal(value)
	if err != nil {
		return EnqueueOutcome{Status: EnqueueError, Err: err}
	}
	if r.skipWrite() {
		return EnqueueOutcome{Status: EnqueueBackoff, Err: ErrRedisWriteBackoff}
	}
	if err := r.rdb.LPush(ctx, key, b).Err(); err != nil {
		if r.classifyWriteErr(err) {
			return EnqueueOutcome{Status: EnqueueBackoff, Err: ErrRedisWriteBackoff}
		}
		return EnqueueOutcome{Status: EnqueueError, Err: err}
	}
	return EnqueueOutcome{Status: EnqueueEnqueued}
}

// noteReadOnly 開始寫入退避。與 handleWriteErr 的差別是**它不吞掉錯誤**——
// 呼叫端會收到 backoff 狀態。
func (r *RedisClient) noteReadOnly() {
	r.writeBackoffUntil.Store(time.Now().Add(redisReadOnlyBackoff).UnixNano())
}

// classifyWriteErr 把一次寫入失敗分成「退避」與「一般錯誤」，並在前者開始退避。
//
// **抽成函式是為了讓「第一次收到 READONLY 就回 backoff」測得到**——三個操作
// 都走這裡，不必真的架一個 read-only replica 才驗得了那條分支。
//
// ⚠️ 與既有 handleWriteErr 的關鍵差別：**它不吞掉錯誤**。handleWriteErr 在
// 偵測到 READONLY 時設完退避就 return nil，等於把那一次的失敗當成成功。
func (r *RedisClient) classifyWriteErr(err error) (isBackoff bool) {
	if isRedisReadOnlyErr(err) {
		r.noteReadOnly()
		return true
	}
	return false
}
