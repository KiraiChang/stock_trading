package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// ── 測試替身 ────────────────────────────────────────────────────────────────

type gapVerificationStub struct {
	states     map[string]store.CandleVerificationState
	loadErr    error
	recorded   [][]store.VerificationAttempt
	recordErr  error
	loadCalled int
}

func (s *gapVerificationStub) LoadStates(
	_ context.Context, _ string, _ []string,
) (map[string]store.CandleVerificationState, error) {
	s.loadCalled++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	out := make(map[string]store.CandleVerificationState, len(s.states))
	for k, v := range s.states {
		out[k] = v
	}
	return out, nil
}

func (s *gapVerificationStub) RecordAttempts(_ context.Context, a []store.VerificationAttempt) error {
	s.recorded = append(s.recorded, append([]store.VerificationAttempt(nil), a...))
	return s.recordErr
}

func (s *gapVerificationStub) lastBatch() []store.VerificationAttempt {
	if len(s.recorded) == 0 {
		return nil
	}
	return s.recorded[len(s.recorded)-1]
}

type gapReferenceStub struct {
	nonTrading map[string]bool
	calErr     error
	traded     map[string]map[string]bool // symbol → date → 有成交
	tradedErr  map[string]error
	openSource map[string]bool
	queried    []string
	// sourceAsOf 是市場層級端點涵蓋到的最後一天。留空代表「跟上了」——
	// 由 newGapScheduler 填成最新的預期交易日，這樣預設情境不會落進 deferred。
	sourceAsOf    string
	sourceAsOfErr error
}

func (r *gapReferenceStub) TradingCalendar(_ context.Context, year int) (*market.TradingCalendar, error) {
	if r.calErr != nil {
		return nil, r.calErr
	}
	return market.NewTradingCalendarForTest(year, r.nonTrading), nil
}

func (r *gapReferenceStub) MarketLastTradingDate(context.Context) (string, error) {
	if r.sourceAsOfErr != nil {
		return "", r.sourceAsOfErr
	}
	return r.sourceAsOf, nil
}

func (r *gapReferenceStub) StockTradedDates(
	_ context.Context, symbol, _ string, _ int, _ time.Month,
) (map[string]bool, error) {
	r.queried = append(r.queried, symbol)
	if err, ok := r.tradedErr[symbol]; ok {
		return nil, err
	}
	return r.traded[symbol], nil
}

func (r *gapReferenceStub) IsSourceOpen(source string) bool { return r.openSource[source] }

// gapCandleStub 只回「這些標的在區間內有哪幾天」。
type gapCandleStub struct {
	store.CandleRepo
	dates map[string][]string
	err   error
}

func (s *gapCandleStub) CandleDatesInRange(
	_ context.Context, symbols []string, _ string, _, _ time.Time,
) (map[string][]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string][]string, len(symbols))
	for _, sym := range symbols {
		if d, ok := s.dates[sym]; ok {
			out[sym] = d
		}
	}
	return out, nil
}

func newGapScheduler(
	verification *gapVerificationStub, reference *gapReferenceStub,
	candles store.CandleRepo, symbols store.StockSymbolRepo,
	mutate func(*config.CandleGapDetectionConfig),
) (*Scheduler, *schedulerJobRunRepoStub) {
	jobRuns := &schedulerJobRunRepoStub{}
	if reference.sourceAsOf == "" && reference.sourceAsOfErr == nil {
		// 預設讓對照源「跟上了」：否則每支測試都會先落進 deferred 或陳舊升級，
		// 驗不到它們各自要驗的東西。deferred 與陳舊由專門的測試覆蓋。
		days := recentTradingDays(1)
		reference.sourceAsOf = days[len(days)-1]
	}
	if symbols == nil {
		// 四項必要依賴之一。多數測試不關心它的內容，但**不能是 nil**——
		// 缺任一項偵測就不註冊（那件事由 TestCandleGapDetectionRequiresAllFourDependencies 專門驗）。
		symbols = &universeSymbolStub{}
	}
	s := New(
		nil, nil, nil, jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
	s.SetEvaluationUniverse(&universeRepoStub{}, candles, symbols,
		config.EvaluationUniverseConfig{Days: 10})
	cfg := config.CandleGapDetectionConfig{
		Enabled: true, AggregateRatio: 0.5, AggregateMinSymbols: 5,
		CandidateCapPerRun: 20, TimeoutSec: 300, LookbackTradingDays: 3,
		RequestIntervalMs: 100, MarketStaleDays: 2, CalendarTTLHours: 24,
		BreakerFailures: 5, BreakerCooldownMin: 60,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	s.SetCandleGapDetection(verification, reference, cfg)
	return s, jobRuns
}

func listedState(mkt string) store.StockSymbolState {
	return store.StockSymbolState{IsListed: true, Market: mkt}
}

// recentTradingDays 回傳往回數的 n 個交易日（跳過週末），升冪，與實作的預期集合一致。
func recentTradingDays(n int) []string {
	today := timeutil.TodayTaipei()
	out := make([]string, 0, n)
	for d := today.AddDate(0, 0, -1); len(out) < n; d = d.AddDate(0, 0, -1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// gapJobRun 取出偵測寫的那筆 job_runs。
//
// 這些測試只呼叫 runCandleGapDetection，所以紀錄只會有一筆；同時斷言 job 名稱是
// **獨立的 candle_gap_detection**，不是共用 parent 的——兩者要分得開。
func gapJobRun(t *testing.T, jobRuns *schedulerJobRunRepoStub) schedulerJobRunFinish {
	t.Helper()
	if len(jobRuns.started) != 1 || jobRuns.started[0] != candleGapDetectionJob {
		t.Fatalf("應只寫一筆 %s 的紀錄，實際 started=%v", candleGapDetectionJob, jobRuns.started)
	}
	if len(jobRuns.finished) != 1 {
		t.Fatalf("沒有寫出結束狀態，finished=%v", jobRuns.finished)
	}
	return jobRuns.finished[0]
}

func nullTimeAt(t time.Time) store.NullTime {
	return store.NullTime{NullTime: sql.NullTime{Time: t, Valid: true}}
}

// ── 本體 ────────────────────────────────────────────────────────────────────

// 矩陣 #2：單檔中段缺漏要報。**這是跳過最佳化（T-062）之後的主要盲點**——
// 該檔今天有 K 棒就會被整檔跳過，五天前那個洞永遠不會被重新抓取。
func TestCandleGapDetectionReportsMiddleGapConfirmedByExchange(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		// 交易所那天**有成交** → 我們沒有 = 真的缺口。
		traded: map[string]map[string]bool{"2330": {days[1]: true}},
	}
	candles := &gapCandleStub{dates: map[string][]string{
		"2330": {days[0], days[2]}, // 中間那天缺
	}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("確認缺口必須 partial，得到 %q（error=%q）", run.status, run.errMsg)
	}
	if !strings.Contains(run.errMsg, "upstream_data_gap") {
		t.Errorf("error 欄要帶 upstream_data_gap，得到 %q", run.errMsg)
	}
	// **不得計入 symbols_failed**：那些標的的回補本身是成功的（上游回什麼就寫什麼）。
	if run.symbolsFailed != 0 {
		t.Errorf("缺口不得計入 symbols_failed，得到 %d", run.symbolsFailed)
	}
	// gap 是**成功的驗證**：last_verified_at 要更新、failures 要歸零。
	batch := verification.lastBatch()
	if len(batch) != 1 || batch[0].LastResult != store.VerificationGap {
		t.Fatalf("寫回的結論不對：%+v", batch)
	}
	if batch[0].LastVerifiedAt.IsZero() || batch[0].ConsecutiveFailures != 0 {
		t.Errorf("gap 仍是成功的驗證：%+v", batch[0])
	}
}

// 矩陣 #3：合法停止買賣（2867 形狀）**不得告警**——交易所那幾天也沒有成交。
//
// 這正是「用交易所核對」取代「用筆數猜測」的價值：不需要為它做例外。
func TestCandleGapDetectionDoesNotReportLegitimateHalt(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		traded: map[string]map[string]bool{"2867": {}}, // 交易所那幾天也沒成交
	}
	candles := &gapCandleStub{dates: map[string][]string{"2867": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2867"},
		map[string]store.StockSymbolState{"2867": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "success" {
		t.Errorf("交易所那幾天也沒成交＝正常，不得告警，得到 %q（%q）", run.status, run.errMsg)
	}
	batch := verification.lastBatch()
	if len(batch) != 1 || batch[0].LastResult != store.VerificationVerified {
		t.Errorf("應記成 verified：%+v", batch)
	}
}

// 矩陣 #13、#21：aggregate 只看**單一 (market, date) 分組**的比例。
//
// 全池同日缺漏 → 一次來源級告警，**不得展開逐檔請求**（那會在上游已經出問題時
// 把偵測變成對交易所的壓力測試）。
func TestCandleGapDetectionAggregatesWholeMarketOutage(t *testing.T) {
	days := recentTradingDays(3)
	symbols := []string{"1101", "1102", "1103", "1104", "1105", "1106"}
	states := make(map[string]store.StockSymbolState, len(symbols))
	dates := make(map[string][]string, len(symbols))
	for _, sym := range symbols {
		states[sym] = listedState("上市")
		dates[sym] = []string{days[0], days[2]} // 全部都缺 days[1]
	}
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{}
	s, jobRuns := newGapScheduler(verification, reference, &gapCandleStub{dates: dates}, nil, nil)

	s.runCandleGapDetection(context.Background(), symbols, states, nil)

	if len(reference.queried) != 0 {
		t.Errorf("aggregate 短路後不得逐檔請求，實際查了 %v", reference.queried)
	}
	run := gapJobRun(t, jobRuns)
	if !strings.Contains(run.errMsg, "市場層級缺漏") {
		t.Errorf("要發市場層級告警，得到 %q", run.errMsg)
	}
	// **短路的那批也要寫 state**——理由是公平排序的簿記：不更新 last_attempted_at
	// 的話它們會永遠排在最前面並佔滿 cap，把其他候選餓死。
	if len(verification.lastBatch()) != len(symbols) {
		t.Errorf("短路的那批也要寫回 state，得到 %d 筆", len(verification.lastBatch()))
	}
}

// 矩陣 #30：某市場有效池 < aggregate_min_symbols 時**強制逐檔**，不套用比例。
//
// 否則單一檔的市場裡那一檔合法停止買賣，比例就是 100%，
// 會被短路成來源級告警——直接違反「2867 這類不得告警」的要求。
func TestCandleGapDetectionForcesPerSymbolWhenPoolBelowMinimum(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{"6182": {}}}
	candles := &gapCandleStub{dates: map[string][]string{"6182": {days[0], days[2]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	// 上櫃只有 1 檔（< 5），比例會是 100%。
	s.runCandleGapDetection(context.Background(), []string{"6182"},
		map[string]store.StockSymbolState{"6182": listedState("上櫃")}, nil)

	if len(reference.queried) == 0 {
		t.Error("小母體必須強制逐檔核對，不得套用比例短路")
	}
	if run := gapJobRun(t, jobRuns); run.status != "success" {
		t.Errorf("逐檔驗出「交易所也沒成交」＝正常，得到 %q（%q）", run.status, run.errMsg)
	}
}

// 矩陣 #23、#26：breaker 開啟的來源要被跳過，**不計入 cap**，由其他來源補滿；
// 且**只要有任何候選被跳過，該輪就是 partial**。
func TestCandleGapDetectionSkipsBreakerOpenSourceAndStillPartial(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		openSource: map[string]bool{market.SourceTWSEStockDay: true},
		traded:     map[string]map[string]bool{"6182": {}},
	}
	candles := &gapCandleStub{dates: map[string][]string{
		"2330": {days[0]}, "6182": {days[0]},
	}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, func(c *config.CandleGapDetectionConfig) {
		c.CandidateCapPerRun = 1 // cap 只有 1：被跳過的不得佔用它
	})

	s.runCandleGapDetection(context.Background(), []string{"2330", "6182"},
		map[string]store.StockSymbolState{
			"2330": listedState("上市"), "6182": listedState("上櫃"),
		}, nil)

	// 上市那檔被跳過（不佔 cap），上櫃那檔仍要被驗到。
	if !reflect.DeepEqual(reference.queried, []string{"6182"}) {
		t.Errorf("被跳過的不該佔用 cap，其他來源要補上，實際查了 %v", reference.queried)
	}
	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("有候選因 breaker 被跳過就該 partial，得到 %q", run.status)
	}
	if !strings.Contains(run.errMsg, "breaker open") {
		t.Errorf("error 欄要說明是 breaker，得到 %q", run.errMsg)
	}
	// **被跳過的不得更新 last_attempted_at**：它是刻意不被嘗試，不是嘗試失敗。
	for _, a := range verification.lastBatch() {
		if a.Symbol == "2330" {
			t.Error("breaker 跳過的候選不得寫入 attempt")
		}
	}
}

// 矩陣 #7、#14：個股端點失敗 → verification_unavailable ＋ partial，
// **不得誤判成「無成交」而停在 success**。
func TestCandleGapDetectionMarksUnavailableWhenLookupFails(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		tradedErr: map[string]error{"2330": market.ErrVerificationUnavailable},
	}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("驗不了要 partial，得到 %q", run.status)
	}
	if !strings.Contains(run.errMsg, "verification_unavailable") {
		t.Errorf("error 欄要帶 verification_unavailable，得到 %q", run.errMsg)
	}
	batch := verification.lastBatch()
	if len(batch) != 1 || batch[0].LastResult != store.VerificationUnavailable {
		t.Fatalf("應記成 unavailable：%+v", batch)
	}
	// 沒有任何成功且至少一個 unavailable → failures +1。
	if batch[0].ConsecutiveFailures != 1 {
		t.Errorf("consecutive_failures 應 +1，得到 %d", batch[0].ConsecutiveFailures)
	}
	// **不得覆寫既有的 last_verified_at**——repo 用 COALESCE 保護，這裡要傳零值。
	if !batch[0].LastVerifiedAt.IsZero() {
		t.Error("沒驗成就不該帶 last_verified_at")
	}
}

// 矩陣 #24：StatesBySymbols 整體失敗時，偵測的收斂是**整批 unavailable ＋ partial**
// （與回補的「全量重抓」不同）。
//
// 驗不了卻記 success，正是本筆要消滅的誤導。
func TestCandleGapDetectionUnavailableWhenSymbolStatesFail(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{}
	s, jobRuns := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"}, nil, errors.New("boom"))

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("失去 market routing 要 partial，得到 %q", run.status)
	}
	if !strings.Contains(run.errMsg, "verification_unavailable") {
		t.Errorf("error 欄要帶 verification_unavailable，得到 %q", run.errMsg)
	}
	if len(reference.queried) != 0 {
		t.Error("拿不到 market 就不該亂猜端點去查")
	}
}

// 矩陣 #24 的端到端合成：**同一次** StatesBySymbols 失敗，回補與偵測要走出方向相反的收斂。
//
// 這件事原本分成兩支測（上面那支，與 scheduler_test.go 的
// TestEvaluationUniverseSyncFallsBackToFullPoolWhenMasterLookupFails），兩邊行為都有守到，
// 但**分開測會讓「其中一邊忘了處理」看不出來**——那正是矩陣原文要求合成一支的理由：
//
//   - 回補：多抓可接受，靜默少抓不可接受 → 退回**全量**。
//   - 偵測：失去 market routing 就決定不了核對端點，猜了會產生假結果 → **不猜**，
//     記 partial ＋ verification_unavailable。
//
// 只驗其中一邊的話，另一邊被改成「跟著一起降級」或「跟著一起硬幹」都不會有東西報錯。
//
// 順帶釘住**兩筆 job_runs 是分開的**：偵測判 partial 不得污染 evaluation_universe_sync
// 的狀態，而且偵測寫在 parent 自己的紀錄關閉之後（scheduler.go 的尾端呼叫）。
func TestEvaluationUniverseSyncAndGapDetectionDivergeWhenMasterLookupFails(t *testing.T) {
	pool := []string{"1101", "1102", "1103"}
	symbols := &universeSymbolStub{err: errors.New("boom")}
	// candles 不能是 nil：對 parent 它是合法的 nil（退回全量），但那樣偵測的四項依賴
	// 不齊就不會註冊，這支測試會變成只驗了回補那一半而毫無提示。
	s, source, jobRuns := newUniverseSyncSchedulerWithSymbols(pool, &universeCandleStub{}, symbols)
	s.SetCandleGapDetection(&gapVerificationStub{}, &gapReferenceStub{},
		config.CandleGapDetectionConfig{
			Enabled: true, AggregateRatio: 0.5, AggregateMinSymbols: 5,
			CandidateCapPerRun: 20, TimeoutSec: 300, LookbackTradingDays: 3,
			RequestIntervalMs: 100, MarketStaleDays: 2, CalendarTTLHours: 24,
			BreakerFailures: 5, BreakerCooldownMin: 60,
		})

	s.runEvaluationUniverseSync(context.Background())

	// ── 前提：兩邊看到的必須是**同一次**查詢的失敗 ──
	//
	// 沒有這條斷言的話，日後若改成回補與偵測各查一次主檔、兩次都失敗，這支測試
	// 照樣會過——但它宣稱守住的「同一個失敗走出兩個方向」就不再成立了，
	// 而且多一次查詢本身就是迴歸（整輪只讀一次是刻意的）。
	if symbols.calls != 1 {
		t.Errorf("StatesBySymbols 被呼叫 %d 次, 期望 1 次（兩邊共用同一次查詢的結果）", symbols.calls)
	}

	// ── 回補側：退回全量 ──
	if !reflect.DeepEqual(source.fetched, pool) {
		t.Errorf("回補實際抓取 %v, 期望整池 %v（主檔查詢失敗要退回全量）", source.fetched, pool)
	}

	// ── 偵測側：同一個失敗走相反方向 ──
	wantStarted := []string{"evaluation_universe_sync", candleGapDetectionJob}
	if !reflect.DeepEqual(jobRuns.started, wantStarted) {
		t.Fatalf("job_runs 應為兩筆且分開，實際 started=%v 期望 %v", jobRuns.started, wantStarted)
	}
	if len(jobRuns.finished) != 2 {
		t.Fatalf("兩筆紀錄都要寫出結束狀態，finished=%v", jobRuns.finished)
	}

	parent, detection := jobRuns.finished[0], jobRuns.finished[1]
	if parent.status != "success" {
		t.Errorf("回補退回全量後本身是成功的，status = %q（偵測的 partial 不得污染它）", parent.status)
	}
	if detection.status != "partial" {
		t.Errorf("偵測失去 market routing 要 partial，得到 %q", detection.status)
	}
	if !strings.Contains(detection.errMsg, "verification_unavailable") {
		t.Errorf("偵測的 error 欄要帶 verification_unavailable，得到 %q", detection.errMsg)
	}
	if detection.symbolsTotal != len(pool) {
		t.Errorf("偵測的 symbols_total = %d, 期望池大小 %d", detection.symbolsTotal, len(pool))
	}
}

// 矩陣 #11、#17 在偵測層的收斂：日曆不可用 → 整輪 partial，不得回報正常。
func TestCandleGapDetectionUnavailableWhenCalendarFails(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{calErr: market.ErrVerificationUnavailable}
	s, jobRuns := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" || !strings.Contains(run.errMsg, "verification_unavailable") {
		t.Errorf("日曆不可用要 partial ＋ unavailable，得到 %q / %q", run.status, run.errMsg)
	}
}

// 矩陣 #22：首次出現的候選（state 無列）必須排在最前面被選中，
// **不得因為 SQL 查不到而消失**。
func TestCandleGapDetectionPrioritisesNeverAttemptedCandidates(t *testing.T) {
	days := recentTradingDays(3)
	old := time.Now().Add(-72 * time.Hour)
	verification := &gapVerificationStub{states: map[string]store.CandleVerificationState{
		// AAA 有 state（很舊），ZZZ 完全沒有 state。
		"AAA": {Symbol: "AAA", Timeframe: "1d",
			LastAttemptedAt: nullTimeAt(old),
			LastResult:      store.VerificationVerified},
	}}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{}}
	candles := &gapCandleStub{dates: map[string][]string{
		"AAA": {days[0]}, "ZZZ": {days[0]},
	}}
	s, _ := newGapScheduler(verification, reference, candles, nil, func(c *config.CandleGapDetectionConfig) {
		c.CandidateCapPerRun = 1
	})

	s.runCandleGapDetection(context.Background(), []string{"AAA", "ZZZ"},
		map[string]store.StockSymbolState{
			"AAA": listedState("上市"), "ZZZ": listedState("上市"),
		}, nil)

	// **沒有 state 的優先**，即使 symbol 排序在後。
	if !reflect.DeepEqual(reference.queried, []string{"ZZZ"}) {
		t.Errorf("從未嘗試過的候選要排最前面，實際查了 %v", reference.queried)
	}
}

// state 寫入失敗不得中斷該輪，但**必須看得見**——排序鍵不前進會退回飢餓，
// 而 ceil(N/cap) 的上界正是以「寫入成功」為前提。
func TestCandleGapDetectionDegradesWhenStateWriteFails(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{recordErr: errors.New("db down")}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{"2330": {}}}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("state 寫入失敗要 degraded，得到 %q", run.status)
	}
	if !strings.Contains(run.errMsg, "verification_state_write_failed") {
		t.Errorf("error 欄要說明，得到 %q", run.errMsg)
	}
}

// 空池不是錯誤，與 parent 的處理一致。
func TestCandleGapDetectionEmptyPoolIsSuccess(t *testing.T) {
	verification := &gapVerificationStub{}
	s, jobRuns := newGapScheduler(verification, &gapReferenceStub{}, &gapCandleStub{}, nil, nil)

	s.runCandleGapDetection(context.Background(), nil, map[string]store.StockSymbolState{}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "success" || run.symbolsTotal != 0 {
		t.Errorf("空池應 success 且 total=0，得到 %q / %d", run.status, run.symbolsTotal)
	}
}

// 未啟用時**完全不執行，也不寫任何 job_runs 紀錄**。
//
// 寫了的話 /scheduler/status 會出現「disabled 卻有紀錄」的矛盾。
func TestCandleGapDetectionDisabledWritesNothing(t *testing.T) {
	verification := &gapVerificationStub{}
	s, jobRuns := newGapScheduler(verification, &gapReferenceStub{}, &gapCandleStub{}, nil,
		func(c *config.CandleGapDetectionConfig) { c.Enabled = false })

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	if len(jobRuns.started) != 0 || len(jobRuns.finished) != 0 {
		t.Fatalf("未啟用不得寫 job_runs 紀錄，started=%v finished=%v",
			jobRuns.started, jobRuns.finished)
	}
}

// 矩陣 #29b：四項必要依賴缺任一項就不算 ready。
//
// **CandleRepo 那一項最容易漏**：它對 parent 是合法的 nil（退回全量回補），
// 但對偵測是完全不能運作——沒有實際日期集合就算不出差集。
func TestCandleGapDetectionRequiresAllFourDependencies(t *testing.T) {
	base := func() *Scheduler {
		s, _ := newGapScheduler(&gapVerificationStub{}, &gapReferenceStub{},
			&gapCandleStub{}, &universeSymbolStub{}, nil)
		return s
	}
	if !base().candleGapDetectionReady() {
		t.Fatal("四項齊全時應 ready")
	}

	cases := map[string]func(*Scheduler){
		"verification": func(s *Scheduler) { s.candleVerification = nil },
		"reference":    func(s *Scheduler) { s.exchangeReference = nil },
		"stockSymbols": func(s *Scheduler) { s.evaluationUniverseSymbols = nil },
		"candles":      func(s *Scheduler) { s.evaluationUniverseCandles = nil },
	}
	for name, drop := range cases {
		t.Run("缺 "+name, func(t *testing.T) {
			s := base()
			drop(s)
			if s.candleGapDetectionReady() {
				t.Errorf("缺 %s 時不該 ready", name)
			}
			if s.candleGapDetectionEnabled() {
				t.Errorf("缺 %s 時不該視為啟用", name)
			}
		})
	}
}

// ── deferred 與對照源陳舊（矩陣 #31、#32、#6） ──────────────────────────────

// 矩陣 #31：缺漏日期**晚於** source_as_of → deferred。
//
// 對照源根本還沒發布那天，「查不到」不等於「無成交」。
// deferred 不告警、不更新 last_verified_at、不加失敗計數，
// 但**要更新 last_attempted_at**，且**該輪不因此 degraded**。
func TestCandleGapDetectionDefersDatesNewerThanSource(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		// 對照源只到倒數第二個交易日 → 最新那天要被 defer。
		sourceAsOf: days[1],
		traded:     map[string]map[string]bool{"2330": {}},
	}
	// 只缺最新那一天。
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0], days[1]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	if len(reference.queried) != 0 {
		t.Errorf("deferred 的候選不該送個股請求，實際查了 %v", reference.queried)
	}
	run := gapJobRun(t, jobRuns)
	if run.status != "success" {
		t.Errorf("正常的發布延遲不是異常，不得 degraded，得到 %q（%q）", run.status, run.errMsg)
	}
	batch := verification.lastBatch()
	if len(batch) != 1 || batch[0].LastResult != store.VerificationDeferred {
		t.Fatalf("應記成 deferred：%+v", batch)
	}
	// 三件事一次釘住：嘗試過（排序要前進）、沒驗成、不算失敗。
	if batch[0].LastAttemptedAt.IsZero() {
		t.Error("deferred 仍要更新 last_attempted_at，否則公平排序不會前進")
	}
	if !batch[0].LastVerifiedAt.IsZero() {
		t.Error("deferred 沒有驗到，不得更新 last_verified_at")
	}
	if batch[0].ConsecutiveFailures != 0 {
		t.Errorf("deferred 不是失敗，不得加計數，得到 %d", batch[0].ConsecutiveFailures)
	}
}

// 矩陣 #32：`lag == market_stale_days` **就要升級**（邊界必測）。
//
// 寫成 `>` 的實作會在這裡漏報，而 deferred 就會變成永遠不處理的黑洞。
func TestCandleGapDetectionEscalatesStaleSourceAtThreshold(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		// 對照源停在三天前 → lag = 2（(days[0], days[2]] 有 days[1]、days[2]），
		// 剛好等於預設 market_stale_days=2。
		sourceAsOf: days[0],
	}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" {
		t.Errorf("lag 等於門檻就要升級成 partial，得到 %q（%q）", run.status, run.errMsg)
	}
	if !strings.Contains(run.errMsg, "verification_unavailable") ||
		!strings.Contains(run.errMsg, "陳舊") {
		t.Errorf("error 欄要說明是對照源陳舊，得到 %q", run.errMsg)
	}
	// ⛔ **不得只是縮短視窗然後回報正常**——整批要落 unavailable。
	batch := verification.lastBatch()
	if len(batch) != 1 || batch[0].LastResult != store.VerificationUnavailable {
		t.Errorf("陳舊時整批應落 unavailable：%+v", batch)
	}
}

// lag = 1（今天的還沒發）在預設門檻 2 之下**不算過期**。
//
// 這條與上一條共同釘住邊界：少了它，一個把門檻寫成 `>=1` 的實作也會通過上一條。
func TestCandleGapDetectionToleratesOneTradingDayPublishLag(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		sourceAsOf: days[1], // lag = 1
		traded:     map[string]map[string]bool{},
	}
	candles := &gapCandleStub{dates: map[string][]string{"2330": days}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	if run := gapJobRun(t, jobRuns); run.status != "success" {
		t.Errorf("lag=1 是正常發布延遲，不得升級，得到 %q（%q）", run.status, run.errMsg)
	}
}

// 矩陣 #6：市場層級端點失敗同樣是 unavailable ＋ partial，**不得回報正常**。
func TestCandleGapDetectionUnavailableWhenMarketSourceFails(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{sourceAsOfErr: market.ErrVerificationUnavailable}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	if run.status != "partial" || !strings.Contains(run.errMsg, "verification_unavailable") {
		t.Errorf("對照源不可用要 partial ＋ unavailable，得到 %q / %q", run.status, run.errMsg)
	}
	if len(reference.queried) != 0 {
		t.Error("分不出 deferred 與 gap 時不該逐檔請求")
	}
}

// ── 其餘矩陣條目 ────────────────────────────────────────────────────────────

// 矩陣 #4：視窗跨月時**兩個月份都要查到**，不能只查起始月。
func TestCandleGapDetectionQueriesEveryMonthInWindow(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{}}
	// 直接測 verifyCandidates：跨月的判斷不依賴「今天是哪一天」才穩定。
	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)

	attempts := map[string]*gapAttemptAgg{}
	_, _, _ = s.verifyCandidates(context.Background(), []gapCandidate{
		{Symbol: "2330", Market: "上市", Date: "2026-07-31"},
		{Symbol: "2330", Market: "上市", Date: "2026-08-03"},
		{Symbol: "2330", Market: "上市", Date: "2026-08-04"},
	}, time.Now(), attempts, nil)

	// 同一檔跨兩個月＝兩次請求；**同月的多天只算一次**（去重鍵是 (symbol, month)）。
	if len(reference.queried) != 2 {
		t.Errorf("跨月要各查一次、同月只查一次，實際查了 %v", reference.queried)
	}
}

// 矩陣 #18、#19：**任一候選最遲在第 ceil(N/cap) 輪被嘗試**。
//
// 第 19 條是關鍵：**前 cap 個每輪都驗證失敗**時，其餘候選仍須在同一個上界內被嘗試——
// 這條專門守「排序鍵是 last_attempted_at 而不是 last_verified_at」那個決定。
// 若只在成功後更新，持續失敗的前幾個會永遠最舊、每輪繼續佔滿配額。
func TestCandleGapDetectionFairnessBoundWithPersistentFailures(t *testing.T) {
	const n, cap = 6, 2
	symbols := []string{"S1", "S2", "S3", "S4", "S5", "S6"}
	states := map[string]store.StockSymbolState{}
	for _, sym := range symbols {
		states[sym] = listedState("上市")
	}

	// state 由測試自己維護，模擬跨輪的持久化。
	persisted := map[string]store.CandleVerificationState{}
	verification := &gapVerificationStub{states: persisted}
	// **每次查詢都失敗**：驗證從不成功，last_verified_at 永遠不會前進。
	tradedErr := map[string]error{}
	for _, sym := range symbols {
		tradedErr[sym] = market.ErrVerificationUnavailable
	}
	reference := &gapReferenceStub{tradedErr: tradedErr}
	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil,
		func(c *config.CandleGapDetectionConfig) { c.CandidateCapPerRun = cap })

	candidates := make([]gapCandidate, 0, n)
	for _, sym := range symbols {
		candidates = append(candidates, gapCandidate{Symbol: sym, Market: "上市", Date: "2026-08-03"})
	}

	tried := map[string]bool{}
	rounds := (n + cap - 1) / cap // ceil(N/cap) = 3
	for r := 0; r < rounds; r++ {
		reference.queried = nil
		attempts := map[string]*gapAttemptAgg{}
		s.verifyCandidates(context.Background(), candidates,
			time.Now().Add(time.Duration(r)*time.Minute), attempts, persisted)
		for _, sym := range reference.queried {
			tried[sym] = true
		}
		// 寫回 state（模擬 RecordAttempts 成功）。
		for sym, agg := range attempts {
			persisted[sym] = store.CandleVerificationState{
				Symbol: sym, Timeframe: "1d",
				LastAttemptedAt: nullTimeAt(agg.lastAttempted),
				LastResult:      agg.result,
			}
		}
	}

	for _, sym := range symbols {
		if !tried[sym] {
			t.Errorf("%s 在 ceil(N/cap)=%d 輪內從未被嘗試——排序鍵用錯會產生飢餓", sym, rounds)
		}
	}
}

// 矩陣 #25：**連續兩輪同樣的缺口都要回報**。
//
// 未修復的缺口本來就該持續可見。把它壓成一次性通知，等於讓一個仍然存在的問題消失。
func TestCandleGapDetectionReportsPersistentGapEveryRound(t *testing.T) {
	days := recentTradingDays(3)
	persisted := map[string]store.CandleVerificationState{}
	verification := &gapVerificationStub{states: persisted}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{"2330": {days[1]: true}}}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0], days[2]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)
	pool := map[string]store.StockSymbolState{"2330": listedState("上市")}

	for r := 0; r < 2; r++ {
		jobRuns.started = nil
		jobRuns.finished = nil
		s.runCandleGapDetection(context.Background(), []string{"2330"}, pool, nil)

		run := gapJobRun(t, jobRuns)
		if run.status != "partial" || !strings.Contains(run.errMsg, "upstream_data_gap") {
			t.Fatalf("第 %d 輪應照常回報缺口，得到 %q / %q", r+1, run.status, run.errMsg)
		}
		// **兩輪都要寫回 state**——排序簿記要前進，否則它會永遠佔住 cap。
		if len(verification.lastBatch()) != 1 {
			t.Fatalf("第 %d 輪應寫回 state", r+1)
		}
		for _, a := range verification.lastBatch() {
			persisted[a.Symbol] = store.CandleVerificationState{
				Symbol: a.Symbol, Timeframe: a.Timeframe,
				LastAttemptedAt: nullTimeAt(a.LastAttemptedAt),
				LastResult:      a.LastResult,
			}
		}
	}
}

// ── coalesce 規則（矩陣 #27b、#38、#39） ────────────────────────────────────

func TestGapAttemptCoalesceRules(t *testing.T) {
	now := time.Now()
	prior := store.CandleVerificationState{ConsecutiveFailures: 3}

	cases := []struct {
		name    string
		results []struct {
			result    string
			succeeded bool
		}
		wantResult   string
		wantVerified bool
		wantFailures int
	}{
		{
			// 矩陣 #27b：跨月一成功、一 unavailable。
			// **部分成功仍是成功**——記成沒驗過會低估實際進度。
			name: "一成功一 unavailable",
			results: []struct {
				result    string
				succeeded bool
			}{
				{store.VerificationVerified, true},
				{store.VerificationUnavailable, false},
			},
			wantResult: store.VerificationUnavailable, wantVerified: true, wantFailures: 0,
		},
		{
			// 矩陣 #38：一個 unavailable ＋ 一個 deferred。
			// ⚠️ 寫成「全部 unavailable 才 +1」的話這裡永遠不會 +1，
			// 「這一檔一直驗不成功」在 state 上就看不出來。
			name: "unavailable ＋ deferred",
			results: []struct {
				result    string
				succeeded bool
			}{
				{store.VerificationUnavailable, false},
				{store.VerificationDeferred, false},
			},
			wantResult: store.VerificationUnavailable, wantVerified: false, wantFailures: 4,
		},
		{
			// 矩陣 #39：只有 deferred → 計數**不動**。
			// 正常的發布延遲不該把失敗計數推上去。
			name: "只有 deferred",
			results: []struct {
				result    string
				succeeded bool
			}{
				{store.VerificationDeferred, false},
			},
			wantResult: store.VerificationDeferred, wantVerified: false, wantFailures: 3,
		},
		{
			// gap 也是成功的驗證，計數要歸零。
			name: "gap 與 verified",
			results: []struct {
				result    string
				succeeded bool
			}{
				{store.VerificationVerified, true},
				{store.VerificationGap, true},
			},
			wantResult: store.VerificationGap, wantVerified: true, wantFailures: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attempts := map[string]*gapAttemptAgg{}
			for _, r := range tc.results {
				mergeAttempt(attempts, "2330", now, r.result, r.succeeded)
			}
			got := attempts["2330"].toAttempt(prior)

			if got.LastResult != tc.wantResult {
				t.Errorf("last_result = %q, 期望 %q（取最嚴重）", got.LastResult, tc.wantResult)
			}
			if got.LastVerifiedAt.IsZero() == tc.wantVerified {
				t.Errorf("last_verified_at 有值 = %v, 期望 %v", !got.LastVerifiedAt.IsZero(), tc.wantVerified)
			}
			if got.ConsecutiveFailures != tc.wantFailures {
				t.Errorf("consecutive_failures = %d, 期望 %d", got.ConsecutiveFailures, tc.wantFailures)
			}
			// last_attempted_at 一律更新——公平排序鍵要前進。
			if got.LastAttemptedAt.IsZero() {
				t.Error("last_attempted_at 一律要更新")
			}
		})
	}
}

// ── 註冊組合（矩陣 #29a、#29c、#29d） ───────────────────────────────────────

// newGapRegistrationScheduler 只為了測 Start() 的註冊判斷，不執行任何 job。
func newGapRegistrationScheduler(
	parentCron string, parentEnabled bool, gapEnabled bool, allDeps bool,
) *Scheduler {
	s := newStartTestScheduler(config.SREvaluationConfig{})
	var candles store.CandleRepo
	var symbols store.StockSymbolRepo
	var verification store.CandleVerificationRepo
	var reference market.ExchangeReference
	if allDeps {
		candles = &gapCandleStub{}
		symbols = &universeSymbolStub{}
		verification = &gapVerificationStub{}
		reference = &gapReferenceStub{}
	}
	s.SetEvaluationUniverse(&universeRepoStub{}, candles, symbols,
		config.EvaluationUniverseConfig{Enabled: parentEnabled, Cron: parentCron})
	s.SetCandleGapDetection(verification, reference, config.CandleGapDetectionConfig{
		Enabled: gapEnabled, AggregateRatio: 0.5, AggregateMinSymbols: 5,
		CandidateCapPerRun: 20, TimeoutSec: 300, LookbackTradingDays: 10,
		RequestIntervalMs: 500, MarketStaleDays: 2, CalendarTTLHours: 24,
		BreakerFailures: 5, BreakerCooldownMin: 60,
	})
	return s
}

func TestCandleGapDetectionRegistrationCombinations(t *testing.T) {
	const goodCron = "0 16 * * 1-5"

	t.Run("兩者都啟用且依賴齊全 → 兩個都註冊", func(t *testing.T) {
		s := newGapRegistrationScheduler(goodCron, true, true, true)
		s.Start()
		defer s.Stop()
		if !s.IsJobRegistered("evaluation_universe_sync") {
			t.Error("parent 應註冊")
		}
		if !s.IsJobRegistered(candleGapDetectionJob) {
			t.Error("偵測應註冊")
		}
	})

	// 矩陣 #29c：偵測關閉、依賴齊全、parent 正常註冊。
	//
	// **偵測不得被標記**——標了會讓 /scheduler/status 顯示 never_run ＋ stale
	// 而不是 disabled，那是假警報。
	t.Run("偵測關閉 → parent 註冊、偵測不註冊", func(t *testing.T) {
		s := newGapRegistrationScheduler(goodCron, true, false, true)
		s.Start()
		defer s.Stop()
		if !s.IsJobRegistered("evaluation_universe_sync") {
			t.Error("parent 應照常註冊")
		}
		if s.IsJobRegistered(candleGapDetectionJob) {
			t.Error("偵測關閉時不得標記為已註冊")
		}
	})

	// 矩陣 #29b（註冊層面）：啟用但依賴不齊 → 視為未註冊。
	//
	// **不得等到執行時才 nil panic**，比照 evaluationUniverse 的「未注入即不註冊」。
	t.Run("啟用但依賴不齊 → 偵測不註冊、parent 不受影響", func(t *testing.T) {
		s := newGapRegistrationScheduler(goodCron, true, true, false)
		s.Start()
		defer s.Stop()
		// parent 的註冊條件不受偵測影響——兩者是不同的需求，不要互相牽制。
		if !s.IsJobRegistered("evaluation_universe_sync") {
			t.Error("parent 的註冊條件不該被偵測牽制")
		}
		if s.IsJobRegistered(candleGapDetectionJob) {
			t.Error("依賴不齊時不得標記為已註冊")
		}
	})

	// 矩陣 #29d：parent 的 cron 字串打錯（AddFunc 失敗）→ **兩個都不標記**。
	//
	// parent 沒註冊成功，掛在它尾端的偵測也不可能執行。
	t.Run("parent cron 打錯 → 兩個都不註冊", func(t *testing.T) {
		s := newGapRegistrationScheduler("not a cron", true, true, true)
		s.Start()
		defer s.Stop()
		if s.IsJobRegistered("evaluation_universe_sync") {
			t.Error("cron 打錯時 parent 不該被標記")
		}
		if s.IsJobRegistered(candleGapDetectionJob) {
			t.Error("parent 沒註冊成功，偵測也不可能執行")
		}
	})

	// parent 關閉 → 偵測沒有自己的 cron，永遠不會執行，同樣不得標記。
	t.Run("parent 關閉 → 兩個都不註冊", func(t *testing.T) {
		s := newGapRegistrationScheduler(goodCron, false, true, true)
		s.Start()
		defer s.Stop()
		if s.IsJobRegistered("evaluation_universe_sync") {
			t.Error("parent 關閉不該註冊")
		}
		if s.IsJobRegistered(candleGapDetectionJob) {
			t.Error("parent 關閉時偵測永遠不會執行，不得標記")
		}
	})
}

// ── parent 的四條早退路徑（矩陣 #29a） ─────────────────────────────────────

// gapUniverseFailStub 讓 ListActive 失敗。
type gapUniverseFailStub struct {
	universeRepoStub
	err error
}

func (s *gapUniverseFailStub) ListActive(context.Context) ([]store.EvaluationUniverseEntry, error) {
	return nil, s.err
}

func TestCandleGapDetectionParentEarlyExits(t *testing.T) {
	newParent := func(universe store.EvaluationUniverseRepo, candles store.CandleRepo) (*Scheduler, *schedulerJobRunRepoStub, *gapVerificationStub) {
		jobRuns := &schedulerJobRunRepoStub{}
		verification := &gapVerificationStub{}
		reference := &gapReferenceStub{sourceAsOf: recentTradingDays(1)[0]}
		s := New(
			market.NewFetcher(&universeSourceStub{}, &universeCandleStub{}, zap.NewNop()),
			nil, nil, jobRuns, nil, nil, nil, "", nil, "", false,
			nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
		)
		s.SetEvaluationUniverse(universe, candles, &universeSymbolStub{},
			config.EvaluationUniverseConfig{Days: 10})
		s.SetCandleGapDetection(verification, reference, config.CandleGapDetectionConfig{
			Enabled: true, AggregateRatio: 0.5, AggregateMinSymbols: 5,
			CandidateCapPerRun: 20, TimeoutSec: 300, LookbackTradingDays: 3,
			RequestIntervalMs: 100, MarketStaleDays: 2, CalendarTTLHours: 24,
			BreakerFailures: 5, BreakerCooldownMin: 60,
		})
		return s, jobRuns, verification
	}

	// **拿不到清單＝跑了但驗不了**，那要看得見；靜默略過會讓狀態頁停在上一次的結果。
	t.Run("ListActive 失敗 → 偵測仍要記 partial", func(t *testing.T) {
		s, jobRuns, _ := newParent(&gapUniverseFailStub{err: errors.New("db down")}, &gapCandleStub{})
		s.runEvaluationUniverseSync(context.Background())

		var found bool
		for i, name := range jobRuns.started {
			if name != candleGapDetectionJob {
				continue
			}
			found = true
			run := jobRuns.finished[i]
			if run.status != "partial" {
				t.Errorf("應 partial，得到 %q", run.status)
			}
			if !strings.Contains(run.errMsg, "verification_unavailable") {
				t.Errorf("error 欄要帶 verification_unavailable，得到 %q", run.errMsg)
			}
		}
		if !found {
			t.Errorf("偵測要建立紀錄，實際 started=%v", jobRuns.started)
		}
	})

	// 空池不是錯誤，與 parent 的處理一致。
	t.Run("空池 → 偵測記 success", func(t *testing.T) {
		s, jobRuns, _ := newParent(&universeRepoStub{}, &gapCandleStub{})
		s.runEvaluationUniverseSync(context.Background())

		for i, name := range jobRuns.started {
			if name == candleGapDetectionJob && jobRuns.finished[i].status != "success" {
				t.Errorf("空池應 success，得到 %q", jobRuns.finished[i].status)
			}
		}
	})

	// repo 未注入：parent 根本沒開始，偵測不建立紀錄才與 parent 一致。
	t.Run("universe repo 未注入 → 兩者都不寫紀錄", func(t *testing.T) {
		s, jobRuns, _ := newParent(nil, &gapCandleStub{})
		s.runEvaluationUniverseSync(context.Background())

		if len(jobRuns.started) != 0 {
			t.Errorf("parent 未注入時不該有任何紀錄，得到 %v", jobRuns.started)
		}
	})

	// 防重入跳過：同理，那輪根本沒開始。
	t.Run("防重入跳過 → 兩者都不寫紀錄", func(t *testing.T) {
		s, jobRuns, _ := newParent(&universeRepoStub{}, &gapCandleStub{})
		s.universeSyncRunning.Store(true)
		defer s.universeSyncRunning.Store(false)

		s.runEvaluationUniverseSync(context.Background())

		if len(jobRuns.started) != 0 {
			t.Errorf("防重入跳過時不該有任何紀錄，得到 %v", jobRuns.started)
		}
	})
}

// 矩陣 #16：多個原因要以 "; " 合併，**原因不得互相覆蓋**。
func TestCandleGapDetectionJoinsMultipleErrorCauses(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		sourceAsOf: days[len(days)-1],
		openSource: map[string]bool{market.SourceTPExStockDay: true},
		// 上市那檔驗出真缺口，上櫃那檔因 breaker 被跳過。
		traded: map[string]map[string]bool{"2330": {days[1]: true}},
	}
	candles := &gapCandleStub{dates: map[string][]string{
		"2330": {days[0], days[2]},
		"6182": {days[0], days[2]},
	}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330", "6182"},
		map[string]store.StockSymbolState{
			"2330": listedState("上市"), "6182": listedState("上櫃"),
		}, nil)

	run := gapJobRun(t, jobRuns)
	if !strings.Contains(run.errMsg, "upstream_data_gap") {
		t.Errorf("缺口原因不得被吃掉，得到 %q", run.errMsg)
	}
	if !strings.Contains(run.errMsg, "breaker open") {
		t.Errorf("breaker 原因不得被吃掉，得到 %q", run.errMsg)
	}
	if !strings.Contains(run.errMsg, "; ") {
		t.Errorf("多個原因要以 \"; \" 合併，得到 %q", run.errMsg)
	}
}

// ── Review 回歸：cap 的單位是候選標的，不是月份請求（#2） ────────────────────

// cap=1 且候選跨兩個月時，**那一檔的兩個月要一次做完**，只佔一個額度。
//
// 按月份扣額度的話：cap 20 在跨月時只處理得到 10 檔；更嚴重的是可能
// **在同一 symbol 的月份中途停止**，於是拿一半的結果去做跨月 coalesce
// （只驗了 7 月就宣告 verified，8 月的缺口沒被看過）。
func TestCandleGapDetectionCapCountsCandidatesNotMonthlyRequests(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{}}
	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil,
		func(c *config.CandleGapDetectionConfig) { c.CandidateCapPerRun = 1 })

	attempts := map[string]*gapAttemptAgg{}
	s.verifyCandidates(context.Background(), []gapCandidate{
		{Symbol: "AAA", Market: "上市", Date: "2026-07-31"},
		{Symbol: "AAA", Market: "上市", Date: "2026-08-03"},
		{Symbol: "BBB", Market: "上市", Date: "2026-08-03"},
	}, time.Now(), attempts, nil)

	// AAA 佔掉唯一的額度，但它的**兩個月都要查**。
	if len(reference.queried) != 2 {
		t.Errorf("跨月的候選要一次做完兩個月，實際查了 %v", reference.queried)
	}
	for _, sym := range reference.queried {
		if sym != "AAA" {
			t.Errorf("cap=1 只該處理一個候選標的，實際查了 %v", reference.queried)
		}
	}
	// BBB 沒被處理——它會在下一輪排到前面。
	if _, ok := attempts["BBB"]; ok {
		t.Error("超出 cap 的候選不得被寫入 attempt")
	}
}

// ── Review 回歸：state 讀取失敗要 degraded 且不得覆寫累積計數（#4） ──────────

func TestCandleGapDetectionDegradesAndSkipsWriteWhenStateReadFails(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{loadErr: errors.New("db down")}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{"2330": {}}}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, jobRuns := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	run := gapJobRun(t, jobRuns)
	// 原本只記 log，job 仍會顯示 success——那會讓「排序停滯、同一批每輪都被選中」
	// 完全看不出來。
	if run.status != "partial" {
		t.Errorf("state 讀取失敗要 degraded，得到 %q（%q）", run.status, run.errMsg)
	}
	if !strings.Contains(run.errMsg, "verification_state_read_failed") {
		t.Errorf("error 欄要說明，得到 %q", run.errMsg)
	}
	// **不得寫回**：prior 是空的，寫下去會把既有的 consecutive_failures 抹成 0/1。
	if len(verification.recorded) != 0 {
		t.Errorf("讀不到 prior 就不該寫回，實際寫了 %v", verification.recorded)
	}
}

// prior 讀得到時，consecutive_failures 要在既有值上累加，不得從 0 起算。
func TestCandleGapDetectionAccumulatesFailuresFromPriorState(t *testing.T) {
	days := recentTradingDays(3)
	verification := &gapVerificationStub{states: map[string]store.CandleVerificationState{
		"2330": {Symbol: "2330", Timeframe: "1d", ConsecutiveFailures: 4,
			LastResult: store.VerificationUnavailable},
	}}
	reference := &gapReferenceStub{
		tradedErr: map[string]error{"2330": market.ErrVerificationUnavailable},
	}
	candles := &gapCandleStub{dates: map[string][]string{"2330": {days[0]}}}
	s, _ := newGapScheduler(verification, reference, candles, nil, nil)

	s.runCandleGapDetection(context.Background(), []string{"2330"},
		map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

	batch := verification.lastBatch()
	if len(batch) != 1 {
		t.Fatalf("應寫回一筆，得到 %+v", batch)
	}
	if batch[0].ConsecutiveFailures != 5 {
		t.Errorf("應在既有的 4 上累加成 5，得到 %d——從 0 起算會讓「一直驗不成功」看不出來",
			batch[0].ConsecutiveFailures)
	}
}
