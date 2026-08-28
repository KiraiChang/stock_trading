package market

import (
	"sync"
	"time"
)

// 缺漏偵測的對照來源代號（`issue.md` I-091）。breaker 以此為鍵。
const (
	SourceTWSECalendar = "twse_calendar"
	SourceTWSEMarket   = "twse_market"
	SourceTWSEStockDay = "twse_stock_day"
	SourceTPExStockDay = "tpex_stock_day"
)

// SourceBreaker 是**來源層級**的斷路器。
//
// ⚠️ **與逐 symbol 的 consecutive_failures 是兩件事，不要混為一談。**
// 同一來源對五個不同 symbol 各失敗一次，逐 symbol 的計數都是 1，
// **永遠推導不出「這個來源已連敗五次」**——那正是需要這一層的原因。
//
// **只存在行程內記憶體**：這是 runtime 安全閥，不需要跨重啟保存，
// 重啟後重新探測是可接受的（而且比帶著一個過期的開啟狀態啟動更安全）。
//
// ⚠️ **只有「真的送出請求且失敗」才呼叫 Fail**。能力限制（該來源根本不提供這種查詢）
// 與 deferred（對照源尚未涵蓋該日期）**沒有對來源發出任何失敗請求**，
// 把它們算進來會讓「幾個查不了的候選」直接打開 breaker，
// **連原本驗得到的資料也一起被跳過**——用一個已知限制去癱瘓一個健康的來源。
type SourceBreaker struct {
	mu       sync.Mutex
	failures map[string]int
	openedAt map[string]time.Time

	threshold int
	cooldown  time.Duration
	// now 可注入，讓冷卻與恢復測得到，不必真的等 60 分鐘。
	now func() time.Time
}

// NewSourceBreaker 的 threshold 與 cooldown 由呼叫端提供**已正規化的值**
// （見 scheduler.normalizeCandleGapDetectionConfig）。這裡仍對非正值兜底，
// 因為 threshold <= 0 會讓 breaker 從第一次失敗就永遠開著，偵測整條變成不可達。
func NewSourceBreaker(threshold int, cooldown time.Duration) *SourceBreaker {
	if threshold < 1 {
		threshold = 1
	}
	if cooldown <= 0 {
		cooldown = time.Minute
	}
	return &SourceBreaker{
		failures:  make(map[string]int, 4),
		openedAt:  make(map[string]time.Time, 4),
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
	}
}

// IsOpen 回報該來源目前是否斷路中。
//
// **冷卻結束時自動恢復**——恢復條件是時間到，不是人工介入。恢復時連同失敗計數一起歸零，
// 否則下一次失敗會立刻再度開啟，等於冷卻沒有意義。
func (b *SourceBreaker) IsOpen(source string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isOpenLocked(source)
}

func (b *SourceBreaker) isOpenLocked(source string) bool {
	opened, ok := b.openedAt[source]
	if !ok {
		return false
	}
	if b.now().Sub(opened) >= b.cooldown {
		delete(b.openedAt, source)
		delete(b.failures, source)
		return false
	}
	return true
}

// Fail 記一次**實際送出且失敗**的請求。達到門檻即開啟斷路。
func (b *SourceBreaker) Fail(source string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// 已經開著就不再累加：冷卻期間本來就不該再送請求，若有也不該讓冷卻時間被延後。
	if b.isOpenLocked(source) {
		return
	}
	b.failures[source]++
	if b.failures[source] >= b.threshold {
		b.openedAt[source] = b.now()
	}
}

// Succeed 把該來源的連續失敗計數歸零。
//
// **「連續」的語意靠這支維持**：中間成功過一次就不算連續，否則一整天零星的失敗
// 累加起來也會打開 breaker，那不是斷路器要偵測的狀況。
func (b *SourceBreaker) Succeed(source string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.failures, source)
	delete(b.openedAt, source)
}

// Failures 回傳目前的連續失敗數，供 log 與測試觀察。
func (b *SourceBreaker) Failures(source string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures[source]
}
