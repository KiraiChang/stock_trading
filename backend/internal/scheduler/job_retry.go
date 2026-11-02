package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/pkg/timeutil"
)

// 暫時性失敗的自動重跑（I-123～I-126 計畫書 docs/plans/i123-i126-job-retry.md）。
//
//   - 層一（本檔）：三個每日工作整輪失敗、原因是暫時性時，隔 interval 重跑；
//     主檔成功之後連帶觸發被它擋下的對帳。
//   - 層二：`intraday` 的批次在同一輪內重試（runIntradayBatch）。
//   - 層三：`corporate_action_sync` 暫時性失敗的標的跑第二輪（retryTransientSymbols）。
//
// 三層都由 `job_retry.enabled` 控制；結束寫入的重試（層四，finishRunStatus）⛔ 不受它控制。
// ⚠️ 重跑鏈只在記憶體裡：backend 重啟後⛔ 不保留。

const (
	stockSymbolSyncJob     = "stock_symbol_sync"
	corporateActionSyncJob = "corporate_action_sync"
)

const (
	defaultJobRetryInterval  = 10 * time.Minute
	defaultJobRetryMaxReruns = 3
	defaultJobRetryCutoff    = "08:45"
	// jobRetryRecordMargin 是最晚起跑時刻額外扣掉的紀錄寫入時間：
	// startRun 上限 10 秒 ＋ 結束寫入重試最多 133 秒 ＋ 餘裕（計畫書「3.2」）。
	// 最晚起跑 ＝ 截止 − 業務預算 − 這 3 分鐘，整輪（含紀錄）就會在截止前結束。
	jobRetryRecordMargin = 3 * time.Minute
)

// runTrigger 是一輪的發起者。只有 cron 與自動重跑（含連帶觸發）的失敗會排重跑；
// 手動觸發的失敗⛔ 不排——要不要再按由人決定。
type runTrigger int

const (
	triggerCron runTrigger = iota
	triggerManual
	triggerRerun
	// triggerLinked 是主檔成功之後連帶觸發的對帳，計入對帳自己的每日上限。
	triggerLinked
)

func (t runTrigger) String() string {
	switch t {
	case triggerCron:
		return "cron"
	case triggerManual:
		return "manual"
	case triggerRerun:
		return "rerun"
	case triggerLinked:
		return "linked"
	}
	return "unknown"
}

func (t runTrigger) automatic() bool { return t != triggerManual }

// retryClock 讓測試注入時鐘與計時器（⛔ 不真的等十分鐘）。
// 層二、層三的等待與層二的重試預算也走它，所以測試控制得了「到期」的時刻。
type retryClock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) retryTimer
}

type retryTimer interface{ Stop() bool }

type realRetryClock struct{}

func (realRetryClock) Now() time.Time { return time.Now() }

func (realRetryClock) AfterFunc(d time.Duration, f func()) retryTimer { return time.AfterFunc(d, f) }

// jobRetrySettings 是正規化之後的 job_retry 設定。
type jobRetrySettings struct {
	enabled      bool
	interval     time.Duration
	maxReruns    int
	cutoffHour   int
	cutoffMinute int
}

func (s jobRetrySettings) cutoffString() string {
	return fmt.Sprintf("%02d:%02d", s.cutoffHour, s.cutoffMinute)
}

// normalizeJobRetry：間隔、上限 ≤ 0 時退回預設；截止時刻空白時用預設、格式錯誤時用預設並記 Error
// （比照 corporateActionCron：設定打錯⛔ 不讓重跑靜默失效，也⛔ 不讓它跑到盤中）。
func normalizeJobRetry(cfg config.JobRetryConfig, log *zap.Logger) jobRetrySettings {
	set := jobRetrySettings{
		enabled:   cfg.Enabled,
		interval:  time.Duration(cfg.IntervalMin) * time.Minute,
		maxReruns: cfg.MaxReruns,
	}
	if cfg.IntervalMin <= 0 {
		set.interval = defaultJobRetryInterval
	}
	if cfg.MaxReruns <= 0 {
		set.maxReruns = defaultJobRetryMaxReruns
	}
	raw := strings.TrimSpace(cfg.Cutoff)
	if raw == "" {
		raw = defaultJobRetryCutoff
	}
	cutoff, err := time.Parse("15:04", raw)
	if err != nil {
		log.Error("job_retry.cutoff 格式錯誤（要 HH:MM），改用預設值",
			zap.String("cutoff", cfg.Cutoff), zap.String("fallback", defaultJobRetryCutoff), zap.Error(err))
		cutoff, _ = time.Parse("15:04", defaultJobRetryCutoff)
	}
	set.cutoffHour, set.cutoffMinute = cutoff.Hour(), cutoff.Minute()
	return set
}

// retryJobSpec 是層一對一個工作需要知道的事。
type retryJobSpec struct {
	// budget 是業務預算（⛔ 不含紀錄寫入），用來算最晚起跑時刻。
	budget func() time.Duration
	// acquire 取得執行所有權（四個入口共用，D5）。⛔ 不能碰 jobRetry.mu——它在鎖內被呼叫。
	acquire func() bool
	// run 必須在已取得所有權時呼叫；它跑完之後自己釋放。
	run func(trigger runTrigger)
}

// retryChain 是一個工作**當天**的重跑鏈（計畫書「3.2」規則 1）：計數與至多一個待跑的重跑。
type retryChain struct {
	day string
	// count 是今天已排的重跑次數（對帳的連帶觸發也算）。排了就算，之後被取消也⛔ 不退回。
	count int
	// succeeded：今天任一入口成功過一次，舊鏈就失效、當天⛔ 不再重跑（規則 2）。
	succeeded bool
	// pendingAt 是待跑的重跑時刻，零值＝沒有。失敗的那一輪寫結束紀錄之前就決定好
	// （error 要寫得出幾點重跑），計時器則等它釋放所有權之後才排（afterRelease）。
	pendingAt time.Time
	timer     retryTimer
	// seq 識別計時器：取消、成功或換日之後，舊的計時器到期會被忽略。
	seq uint64
}

// delistingLink 是連帶重跑的狀態（計畫書「3.2」規則 4），全部在 jobRetry.mu 底下判斷。
type delistingLink struct {
	day string
	// stockWritten：主檔**最新一輪**已成功落盤——⛔ 不是「主檔今天曾成功」。
	// 任何一輪主檔開始時設為假，success 寫入 job_runs 成功之後才設為真；
	// 它對應對帳的前置條件讀到的「當日最新一筆是 success」。
	stockWritten bool
	// stockGen 是主檔成功的代號，每次 stockWritten 設為真時加 1。
	stockGen uint64
	// usedGen 是對帳已經用掉的代號：同一個代號最多觸發一次對帳。
	usedGen uint64
	waiting bool // 對帳今天在等主檔
	pending bool // 對帳待觸發
}

type jobRetry struct {
	set   jobRetrySettings
	clock retryClock
	// spawn 啟動連帶觸發的那一輪（它在別的工作的 goroutine 裡被決定，要另起一個）。
	spawn func(func())
	log   *zap.Logger
	jobs  map[string]retryJobSpec

	mu      sync.Mutex
	stopped bool
	chains  map[string]*retryChain
	link    delistingLink
}

// SetJobRetry 注入 job_retry 設定。未呼叫時三層都不動作（比照其他選填排程）。
// **必須在 Start() 之前**：Start() 會記一行設定與最晚起跑時刻，供部署後確認。
func (s *Scheduler) SetJobRetry(cfg config.JobRetryConfig) {
	r := &jobRetry{
		set:    normalizeJobRetry(cfg, s.log),
		clock:  realRetryClock{},
		spawn:  func(f func()) { go f() },
		log:    s.log,
		chains: map[string]*retryChain{},
	}
	r.jobs = map[string]retryJobSpec{
		stockSymbolSyncJob: {
			budget:  func() time.Duration { return stockSymbolSyncTimeout },
			acquire: s.acquireStockSymbolSync,
			run:     s.runStockSymbolSyncAndRelease,
		},
		corporateActionSyncJob: {
			budget:  s.corporateActionTimeout,
			acquire: s.acquireCorporateActionSync,
			run:     s.runCorporateActionSyncAndRelease,
		},
		delistingReconcileJobName: {
			budget:  func() time.Duration { return delistingReconcileTimeout },
			acquire: s.acquireDelistingReconcile,
			run:     s.runDelistingReconcileAndRelease,
		},
	}
	s.retry = r
}

// enabled 對 nil 安全：沒呼叫 SetJobRetry 的 Scheduler（多數測試）就是關閉。
func (r *jobRetry) enabled() bool { return r != nil && r.set.enabled }

func retryDayKey(t time.Time) string { return t.In(timeutil.TaipeiTZ).Format("2006-01-02") }

// now 是重跑用的時鐘（對 nil 安全：沒呼叫 SetJobRetry 時用系統時鐘）。三個工作在取得所有權之後、
// startRun 之前用它記下這一輪的起跑時刻，結束時傳給 planRerun／onSuccess／onStockWritten／onDelistingNotReady。
func (r *jobRetry) now() time.Time {
	if r == nil {
		return time.Now()
	}
	return r.clock.Now()
}

// startedBeforeToday：這一輪是不是在前一天（台北）起跑、今天才結束。
// **一輪算哪一天看它的起跑時刻**（review R1）：前一天 23:55 開始、今天 00:05 成功的一輪，
// ⛔ 不能把今天標成已成功——否則今天 06:30 的暫時性失敗就不再排重跑。對帳的前置
// （StockSymbolSyncSucceededToday）也是看起跑日，兩邊才一致。
func startedBeforeToday(started, now time.Time) bool { return retryDayKey(started) != retryDayKey(now) }

func retryClockText(t time.Time) string { return t.In(timeutil.TaipeiTZ).Format("15:04") }

// latestStart ＝ now 那天的截止時刻 − 業務預算 − jobRetryRecordMargin。
func (r *jobRetry) latestStart(job string, now time.Time) time.Time {
	local := now.In(timeutil.TaipeiTZ)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(),
		r.set.cutoffHour, r.set.cutoffMinute, 0, 0, timeutil.TaipeiTZ)
	return cutoff.Add(-r.jobs[job].budget() - jobRetryRecordMargin)
}

// chainLocked 取今天的鏈；換日就換一條新的，舊的計時器停掉（⛔ 不讓重跑跨日）。呼叫端持有 r.mu。
func (r *jobRetry) chainLocked(job string, now time.Time) *retryChain {
	day := retryDayKey(now)
	c := r.chains[job]
	if c == nil || c.day != day {
		if c != nil {
			r.cancelPendingLocked(c)
		}
		c = &retryChain{day: day}
		r.chains[job] = c
	}
	return c
}

// linkLocked 取今天的連帶狀態；換日就重置（代號照舊遞增）。呼叫端持有 r.mu。
func (r *jobRetry) linkLocked(now time.Time) *delistingLink {
	if day := retryDayKey(now); r.link.day != day {
		r.link = delistingLink{day: day, stockGen: r.link.stockGen, usedGen: r.link.usedGen}
	}
	return &r.link
}

func (r *jobRetry) cancelPendingLocked(c *retryChain) {
	if c.timer != nil {
		c.timer.Stop()
	}
	c.pendingAt, c.timer = time.Time{}, nil
	c.seq++
}

// planRerun 在一輪以 failed 結束、**寫結束紀錄之前**呼叫，回傳要附在 job_runs.error 後面的固定文字
// （計畫書「3.2」的「記錄」）。eligible 由呼叫端判斷：原因是暫時性，而且⛔ 不是這一輪自己的預算用完。
//
// 只決定重跑時刻、⛔ 不排計時器：計時器在這一輪釋放所有權之後才排（afterRelease），
// 否則結束寫入重試得久一點，重跑就會撞上自己還沒釋放的那一輪。
//
// started 是這一輪的起跑時刻：前一天起跑、今天才失敗的一輪，重跑時刻必然跨到起跑的隔天，
// 照「3.2」⛔ 不排，也⛔ 不動今天的鏈（⛔ 不佔今天的額度）；文字沿用既有的「已停止（最晚起跑 HH:MM）」，
// 時刻是起跑那天的最晚起跑。
func (r *jobRetry) planRerun(job string, trigger runTrigger, eligible bool, started time.Time) string {
	if !r.enabled() || !eligible || !trigger.automatic() {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return ""
	}
	now := r.clock.Now()
	if startedBeforeToday(started, now) {
		latest := r.latestStart(job, started)
		r.log.Warn("自動重跑已停止：這一輪在前一天起跑",
			zap.String("job", job), zap.String("latest_start", retryClockText(latest)))
		return fmt.Sprintf("; 自動重跑已停止（最晚起跑 %s）", retryClockText(latest))
	}
	c := r.chainLocked(job, now)
	switch {
	case c.succeeded:
		return ""
	case !c.pendingAt.IsZero():
		// 同一天只有一條鏈、至多一個待跑的重跑：沿用已排的那一個。
		return fmt.Sprintf("; 自動重跑 %d/%d（%s）", c.count, r.set.maxReruns, retryClockText(c.pendingAt))
	case c.count >= r.set.maxReruns:
		r.log.Warn("自動重跑已用完", zap.String("job", job), zap.Int("max", r.set.maxReruns))
		return fmt.Sprintf("; 自動重跑已用完（%d/%d）", r.set.maxReruns, r.set.maxReruns)
	}
	at := now.Add(r.set.interval)
	latest := r.latestStart(job, now)
	if retryDayKey(at) != c.day || at.After(latest) {
		r.log.Warn("自動重跑已停止：重跑時刻晚於最晚起跑時刻",
			zap.String("job", job), zap.String("at", retryClockText(at)), zap.String("latest_start", retryClockText(latest)))
		return fmt.Sprintf("; 自動重跑已停止（最晚起跑 %s）", retryClockText(latest))
	}
	c.count++
	c.pendingAt = at
	c.seq++
	r.log.Info("自動重跑已排定", zap.String("job", job), zap.String("trigger", trigger.String()),
		zap.Int("n", c.count), zap.Int("max", r.set.maxReruns), zap.String("at", retryClockText(at)))
	return fmt.Sprintf("; 自動重跑 %d/%d（%s）", c.count, r.set.maxReruns, retryClockText(at))
}

// onSuccess：這個工作今天成功了一次（任一入口；這一輪⛔ 不是 failed 就算——partial 代表整輪有跑完，
// 整輪重跑救不了個別的失敗）。舊鏈失效：取消待跑的重跑，當天⛔ 不再排（計畫書「3.2」規則 2）。
// 前一天起跑的一輪⛔ 不動今天的狀態（startedBeforeToday）。
func (r *jobRetry) onSuccess(job string, started time.Time) {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	if startedBeforeToday(started, now) {
		return
	}
	c := r.chainLocked(job, now)
	c.succeeded = true
	r.cancelPendingLocked(c)
	if job == delistingReconcileJobName {
		l := r.linkLocked(now)
		l.waiting, l.pending = false, false
	}
}

// afterRelease 在一輪釋放執行所有權之後呼叫：把 planRerun 決定的重跑掛上計時器；
// 對帳另外檢查「待觸發」（所有權釋放後補觸發，計畫書「3.2」規則 4）。
func (r *jobRetry) afterRelease(job string) {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	if c := r.chains[job]; c != nil && !r.stopped && !c.pendingAt.IsZero() && c.timer == nil {
		d := c.pendingAt.Sub(r.clock.Now())
		if d < 0 {
			d = 0
		}
		seq := c.seq
		c.timer = r.clock.AfterFunc(d, func() { r.fire(job, c, seq) })
	}
	r.mu.Unlock()
	if job == delistingReconcileJobName {
		r.tryTriggerDelisting()
	}
}

// fire 是重跑的計時器到期（計畫書「3.2」規則 3）：先確認還是同一天、還沒成功、
// 還沒超過最晚起跑時刻，再取得執行所有權。取不到 → 這次重跑取消、這條鏈結束
// （正在跑的那一輪由它自己的結果決定要不要再排）。
func (r *jobRetry) fire(job string, c *retryChain, seq uint64) {
	r.mu.Lock()
	if r.stopped || r.chains[job] != c || c.seq != seq || c.pendingAt.IsZero() {
		r.mu.Unlock()
		return
	}
	n := c.count
	c.pendingAt, c.timer = time.Time{}, nil
	now := r.clock.Now()
	latest := r.latestStart(job, now)
	var skip string
	switch {
	case c.succeeded:
		skip = "今天已經成功"
	case retryDayKey(now) != c.day:
		skip = "已換日"
	case now.After(latest):
		skip = "已超過最晚起跑時刻 " + retryClockText(latest)
	}
	if skip != "" {
		r.mu.Unlock()
		r.log.Warn("自動重跑取消："+skip, zap.String("job", job), zap.Int("n", n))
		return
	}
	spec := r.jobs[job]
	if !spec.acquire() {
		r.mu.Unlock()
		r.log.Warn("自動重跑取消：已有一輪在執行中，這條重跑鏈結束", zap.String("job", job), zap.Int("n", n))
		return
	}
	r.mu.Unlock()
	r.log.Info("自動重跑開始", zap.String("job", job), zap.Int("n", n), zap.Int("max", r.set.maxReruns))
	spec.run(triggerRerun)
}

// onStockStart：任何一輪 stock_symbol_sync 開始時（已取得所有權、寫開始紀錄之前），
// 「主檔最新一輪已成功落盤」設為假——這一輪開始之後，job_runs 的最新一筆就不是 success 了。
func (r *jobRetry) onStockStart() {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	r.linkLocked(r.clock.Now()).stockWritten = false
	r.mu.Unlock()
}

// onStockWritten：主檔這一輪的 success **已寫入 job_runs** 之後呼叫（失敗或寫入失敗都⛔ 不呼叫）。
// 設為真、代號加 1；對帳在等 → 改成待觸發，接著嘗試觸發。
// 前一天起跑的一輪⛔ 不算：對帳的前置要求最新一筆起跑於今天，這一輪過不了它。
func (r *jobRetry) onStockWritten(started time.Time) {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	now := r.clock.Now()
	if startedBeforeToday(started, now) {
		r.mu.Unlock()
		return
	}
	l := r.linkLocked(now)
	l.stockWritten = true
	l.stockGen++
	if l.waiting {
		l.pending = true
	}
	r.mu.Unlock()
	r.tryTriggerDelisting()
}

// onDelistingNotReady：對帳因前置條件不成立而失敗（cron 或自動重跑發起的；⛔ 不排自己的計時器，
// 前置不成立⛔ 不是暫時性）。**在鎖內重新確認**：主檔最新一輪已成功落盤、而且代號比對帳用掉的新
// → 直接待觸發（對帳釋放所有權時觸發）；否則登記等待。這擋住「對帳讀到主檔還在 running →
// 主檔成功並通知 → 對帳才登記等待」的交錯。前一天起跑的一輪⛔ 不在今天登記。
func (r *jobRetry) onDelistingNotReady(trigger runTrigger, started time.Time) {
	if !r.enabled() || !trigger.automatic() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	if startedBeforeToday(started, now) {
		return
	}
	l := r.linkLocked(now)
	if l.stockWritten && l.stockGen > l.usedGen {
		l.pending = true
		return
	}
	l.waiting = true
}

// tryTriggerDelisting 是連帶觸發：在鎖內再確認一次旗標與代號、當天對帳還沒成功、還有額度、
// 還沒超過對帳的最晚起跑時刻，再取得對帳的執行所有權。
// **取不到（對帳正在跑或還沒釋放）→ 待觸發留著**，它釋放所有權時（afterRelease）會再試一次。
func (r *jobRetry) tryTriggerDelisting() {
	if !r.enabled() {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	now := r.clock.Now()
	l := r.linkLocked(now)
	if !l.pending {
		r.mu.Unlock()
		return
	}
	c := r.chainLocked(delistingReconcileJobName, now)
	var drop string
	switch {
	case c.succeeded:
		l.waiting = false
		drop = "對帳今天已經成功"
	case !l.stockWritten || l.stockGen <= l.usedGen:
		// 旗標已經不成立（中間又有一輪主檔開始），或這個代號已經用過：回到等待，等下一次主檔成功。
		l.waiting = true
		drop = "主檔最新一輪尚未成功落盤，或這次成功已經觸發過"
	case c.count >= r.set.maxReruns:
		drop = "對帳的自動重跑已用完"
	case now.After(r.latestStart(delistingReconcileJobName, now)):
		drop = "已超過對帳的最晚起跑時刻"
	}
	if drop != "" {
		l.pending = false
		gen := l.stockGen
		r.mu.Unlock()
		r.log.Info("連帶觸發對帳取消："+drop, zap.Uint64("stock_gen", gen))
		return
	}
	spec := r.jobs[delistingReconcileJobName]
	if !spec.acquire() {
		r.mu.Unlock()
		return
	}
	l.pending, l.waiting = false, false
	l.usedGen = l.stockGen
	c.count++
	r.cancelPendingLocked(c)
	n, gen := c.count, l.stockGen
	r.mu.Unlock()
	r.log.Info("主檔已成功落盤，連帶觸發對帳",
		zap.Int("n", n), zap.Int("max", r.set.maxReruns), zap.Uint64("stock_gen", gen))
	r.spawn(func() { spec.run(triggerLinked) })
}

// stop 取消全部待跑的重跑（Stop() 時）。
func (r *jobRetry) stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	for _, c := range r.chains {
		r.cancelPendingLocked(c)
	}
}

// logStartup 記一行設定與三個工作的最晚起跑時刻，供部署後確認開關真的進了容器（計畫書「3.7」）。
func (r *jobRetry) logStartup(log *zap.Logger) {
	if r == nil {
		log.Info("job retry: not configured", zap.Bool("enabled", false))
		return
	}
	now := r.clock.Now()
	log.Info("job retry config",
		zap.Bool("enabled", r.set.enabled),
		zap.Duration("interval", r.set.interval),
		zap.Int("max_reruns", r.set.maxReruns),
		zap.String("cutoff", r.set.cutoffString()),
		zap.String("latest_start_corporate_action_sync", retryClockText(r.latestStart(corporateActionSyncJob, now))),
		zap.String("latest_start_stock_symbol_sync", retryClockText(r.latestStart(stockSymbolSyncJob, now))),
		zap.String("latest_start_delisting_reconcile", retryClockText(r.latestStart(delistingReconcileJobName, now))))
}

// ── 層二、層三共用的等待 ─────────────────────────────────────────────

// errIntradayRetryBudget 是層二的重試預算用完時，重試 context 的取消原因。
var errIntradayRetryBudget = errors.New("intraday retry budget exhausted")

// budgetContext 回傳一個 d 之後被取消的 context。**用注入的計時器而⛔ 不是 context.WithTimeout**：
// 測試才控制得了「預算在寫 K 棒途中到期」這種時刻。到期時 Err() 是 Canceled、Cause 是 cause。
func (r *jobRetry) budgetContext(parent context.Context, d time.Duration, cause error) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	t := r.clock.AfterFunc(d, func() { cancel(cause) })
	return ctx, func() {
		t.Stop()
		cancel(context.Canceled)
	}
}

// wait 等 d；ctx 先結束就提早返回 ctx 的錯誤。
func (r *jobRetry) wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan struct{})
	t := r.clock.AfterFunc(d, func() { close(done) })
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	}
}
