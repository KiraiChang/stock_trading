package signal

import (
	"sort"

	"github.com/trading/backend/internal/store"
)

// Stage 是降級的分類。scheduler 靠它產出 job_runs.error 的分類摘要——
// **只有一個布林時分不出是哪一種降級**。
type Stage string

const (
	// StageSignalPersistFailed：signals.Insert 失敗，訊號送出去了但歷史缺一列。
	StageSignalPersistFailed Stage = "signal_persist_failed"
	// StageQueueFailed：Redis enqueue 非預期失敗（READONLY／error）。
	//
	// ⛔ **Redis 設定停用不算**——那是設定狀態不是故障。
	StageQueueFailed Stage = "queue_failed"
	// StageDedupDegraded：判重降級成單層或更弱。
	//
	// 兩種來源：DB 判重查詢失敗；Redis 非預期故障而只剩 local reservation
	// （判重從跨 instance 退成 per-instance，保證確實變弱了）。
	StageDedupDegraded Stage = "dedup_degraded"
)

// EvaluateResult 是 Evaluate 的結果。
//
// ⚠️ **HTTP 200 / SignalGenerated 都不代表已寫入 signal history**——要看 DBPersisted。
type EvaluateResult struct {
	Signal *store.Signal `json:"signal"`

	// SignalGenerated 本次是否產生訊號（沒觸發條件、或被判重抑制時為 false）。
	SignalGenerated bool `json:"signal_generated"`
	// DBPersisted signal history 是否成功落 DB。
	DBPersisted bool `json:"db_persisted"`
	// QueueEnqueued 是否**真的**寫進 Redis queue。
	//
	// ⚠️ Redis 停用或退避時為 false——不能用 err == nil 推導。
	QueueEnqueued bool `json:"queue_enqueued"`
	// BroadcastAttempted 是否呼叫過 broadcast。
	//
	// ⛔ 語意是 *delivery attempted*，**不宣稱 delivered**：BroadcastFn 沒有回傳值
	// （func(string, *store.Signal)），系統無從得知客戶端有沒有收到。
	// 本套件因此**沒有「broadcast 失敗」這個狀態**。
	BroadcastAttempted bool `json:"broadcast_attempted"`

	// Degraded 等同 len(DegradedStages) > 0。
	Degraded bool `json:"degraded"`
	// DegradedStages 降級的分類清單。
	DegradedStages []Stage `json:"degraded_stages,omitempty"`

	// StageErrors 是各 stage 的實際錯誤，**內部用**。
	//
	// ⛔ json:"-" 只擋住這個結構的序列化，**不代表可以往外送**：
	// 寫進 job_runs.error 或 API 回應前一律要過錯誤分類器
	// （scheduler 的 safeJobErrorReason）。原始 cause 只進 log。
	StageErrors map[Stage]error `json:"-"`
}

// markDegraded 記一個降級 stage 與它的 cause。
//
// cause 可以是 nil（例如 Redis 停用那種沒有底層錯誤的情形），
// 但 backoff 一定帶 store.ErrRedisWriteBackoff sentinel。
func (r *EvaluateResult) markDegraded(stage Stage, cause error) {
	if r == nil {
		return
	}
	r.Degraded = true
	if r.StageErrors == nil {
		r.StageErrors = map[Stage]error{}
	}
	if _, seen := r.StageErrors[stage]; !seen {
		r.StageErrors[stage] = cause
		r.DegradedStages = append(r.DegradedStages, stage)
		sort.Slice(r.DegradedStages, func(i, j int) bool { return r.DegradedStages[i] < r.DegradedStages[j] })
	}
}

// FirstStageError 回傳一個代表 cause，供呼叫端做錯誤分類。
// 依 stage 名稱排序取第一個，讓同一組降級每次得到相同結果。
func (r *EvaluateResult) FirstStageError() (Stage, error) {
	if r == nil || len(r.DegradedStages) == 0 {
		return "", nil
	}
	stage := r.DegradedStages[0]
	return stage, r.StageErrors[stage]
}
