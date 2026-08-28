package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// candleGapDetectionJob 是偵測自己的 job_runs 名稱。
//
// **獨立紀錄，不共用 evaluation_universe_sync 的**：偵測判 partial 時不該污染回補的狀態，
// 兩者要分得開（比照 sr_zone_verify 掛在 daily_close 尾端卻寫自己的紀錄）。
const candleGapDetectionJob = "candle_gap_detection"

// SetCandleGapDetection 注入缺漏偵測的依賴與設定。
// 現況說明見 `docs/architecture.md`「日 K 缺漏偵測」（原記於 issue.md I-091，已收斂）。
//
// **cfg 在這裡就正規化完畢**，之後的程式碼一律信任已正規化的值，不再各自防禦。
// 形狀比照 corporateActionCron()：非法值退回預設 ＋ 記 Error。
//
// ⚠️ 另外兩項必要依賴（StockSymbolRepo 與 CandleRepo）由 SetEvaluationUniverse 注入，
// 本函式不重複收——那兩個是 parent 本來就有的東西，各自注入會讓兩邊可能不一致。
func (s *Scheduler) SetCandleGapDetection(
	verification store.CandleVerificationRepo,
	reference market.ExchangeReference,
	cfg config.CandleGapDetectionConfig,
) {
	s.candleVerification = verification
	s.exchangeReference = reference
	s.candleGapCfg = NormalizeCandleGapDetectionConfig(cfg, s.log)
}

// candleGapDetectionReady 回報偵測的**四項必要依賴**是否齊全。
//
// ⚠️ **CandleRepo 這一項最容易漏**：它對 parent 是**合法的 nil**
// （dropSymbolsSyncedToday 未注入時退回全量抓取，見 T-062），但**對偵測完全不能運作**
// ——沒有實際日期集合就算不出差集。沿用 parent 的判斷會讓偵測在 CandleRepo=nil 時
// 被標成已註冊卻永遠產不出結果。
//
// StockSymbolRepo 同理：缺了對 parent 只是少過濾下市標的（fail-open 可接受），
// 對偵測則是**決定不了要打 TWSE 還是 TPEx 端點**。
func (s *Scheduler) candleGapDetectionReady() bool {
	return s.candleVerification != nil &&
		s.exchangeReference != nil &&
		s.evaluationUniverseSymbols != nil &&
		s.evaluationUniverseCandles != nil
}

// candleGapDetectionEnabled 是**有效**啟用條件。
//
// 偵測沒有自己的 cron，所以 parent 沒被註冊時它永遠不會執行。標成 registered 會讓
// /scheduler/status 顯示 never_run ＋ stale——那是假警報。
func (s *Scheduler) candleGapDetectionEnabled() bool {
	return s.candleGapCfg.Enabled && s.candleGapDetectionReady()
}

// monthKey 是逐檔核對的去重鍵：**同一檔在同一個月的多天缺口只需要一次請求**
// （TWSE 與 TPEx 的個股端點都是按月回傳）。
type monthKey struct {
	symbol string
	market string
	year   int
	month  time.Month
}

// gapCandidate 是一個「我們沒有、但可能該有」的 (symbol, 日期)。
type gapCandidate struct {
	Symbol string
	Market string
	Date   string
}

// reportCandleGapDetectionUnavailable 在 parent 早退到「連候選清單都拿不到」時，
// 仍替偵測留下一筆 partial 紀錄。
//
// **不是所有早退都要這樣做**：repo 未注入與防重入跳過那兩條，parent 根本沒開始，
// 偵測不建立紀錄才與 parent 一致；而「拿不到清單」是**跑了但驗不了**，那要看得見。
func (s *Scheduler) reportCandleGapDetectionUnavailable(parent context.Context, reason string) {
	if !s.candleGapDetectionEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(
		context.WithoutCancel(parent), time.Duration(s.candleGapCfg.TimeoutSec)*time.Second)
	defer cancel()

	runID := s.startRun(ctx, candleGapDetectionJob)
	s.finishRunDegraded(ctx, runID, candleGapDetectionJob, 0, 0,
		"verification_unavailable: "+reason, true)
}

// runCandleGapDetection 掛在 runEvaluationUniverseSync 尾端執行。
//
// **獨立於回補流程**：它直接問資料庫「池內每一檔在最近 N 個交易日裡實際有哪幾天的
// K 棒」，與本輪有沒有抓它無關。只看最新日期或本輪筆數都抓不到**視窗中段的洞**，
// 而那正是 T-062 的跳過最佳化之後的主要盲點。
//
// **用自己的 context**，不沿用回補的——回補逾時不該讓偵測連帶失效。
func (s *Scheduler) runCandleGapDetection(
	parent context.Context, symbols []string, states map[string]store.StockSymbolState,
	statesErr error,
) {
	if !s.candleGapDetectionEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(
		context.WithoutCancel(parent), time.Duration(s.candleGapCfg.TimeoutSec)*time.Second)
	defer cancel()

	runID := s.startRun(ctx, candleGapDetectionJob)
	var errParts []string
	degraded := false

	// ⚠️ **StatesBySymbols 失敗時，回補與偵測的收斂不同**：回補是全量重抓（多抓幾檔，溫和），
	// 偵測則是**完全失去 market routing**，決定不了個股核對端點。驗不了卻記 success
	// 正是本筆要消滅的誤導。
	if statesErr != nil {
		s.finishRunDegraded(ctx, runID, candleGapDetectionJob, len(symbols), 0,
			"verification_unavailable: 取不到證券主檔，無法決定核對端點", true)
		return
	}
	if len(symbols) == 0 {
		// 空池不是錯誤，與 parent 的處理一致。
		s.finishRun(ctx, runID, candleGapDetectionJob, 0, 0, "")
		return
	}

	candidates, cal, expected, calErr := s.collectGapCandidates(ctx, symbols, states)
	if calErr != nil {
		s.finishRunDegraded(ctx, runID, candleGapDetectionJob, len(symbols), 0,
			"verification_unavailable: "+calErr.Error(), true)
		return
	}
	if len(candidates) == 0 {
		s.log.Info("candle gap detection done", zap.Int("pool", len(symbols)), zap.Int("candidates", 0))
		s.finishRun(ctx, runID, candleGapDetectionJob, len(symbols), 0, "")
		return
	}

	now := time.Now()
	attempts := make(map[string]*gapAttemptAgg, len(candidates))

	// ── 〇、對照源的陳舊度 ──────────────────────────────────────────────────
	//
	// **市場層級端點比個股端點慢**（實測 2026-08-26 14:13：STOCK_DAY 已含當日、
	// FMTQIK 還停在前一日）。所以要先知道對照源涵蓋到哪一天，才分得出
	// 「上游真的漏了」與「對照源還沒發布到那天」。
	sourceAsOf, staleErr := s.marketSourceAsOf(ctx, cal, expected)
	if staleErr != nil {
		// ⛔ **不得只是縮短掃描視窗然後回報一切正常**：那會讓所有比它新的缺口
		// 都不被檢查，而且不會被歸類為「驗證不可用」——偵測機制本身靜默失效。
		for _, c := range candidates {
			mergeAttempt(attempts, c.Symbol, now, store.VerificationUnavailable, false)
		}
		if prior, err := s.candleVerification.LoadStates(
			ctx, evaluationUniverseTimeframe, symbols); err == nil {
			// 讀得到才寫——讀不到就寧可不寫，理由同下方主路徑。
			_ = s.recordAttempts(ctx, attempts, prior)
		} else {
			s.log.Error("candle verification state 讀取失敗，本輪不寫回", zap.Error(err))
		}
		s.finishRunDegraded(ctx, runID, candleGapDetectionJob, len(symbols), 0,
			"verification_unavailable: "+staleErr.Error(), true)
		return
	}

	// **缺漏日期晚於 source_as_of → deferred**：對照源根本還沒發布那天，
	// 「查不到」不等於「無成交」。deferred 不告警、不更新 last_verified_at、
	// 不加失敗計數，但**要更新 last_attempted_at**（確實嘗試過，公平排序要前進），
	// 且**不讓該輪 degraded**——正常的發布延遲不是異常。
	pending := make([]gapCandidate, 0, len(candidates))
	deferred := 0
	for _, c := range candidates {
		if c.Date > sourceAsOf {
			deferred++
			mergeAttempt(attempts, c.Symbol, now, store.VerificationDeferred, false)
			continue
		}
		pending = append(pending, c)
	}
	candidates = pending

	// ── 〇之二、公平排序簿記 ────────────────────────────────────────────────
	//
	// ⚠️ **整輪只讀一次**：這份 state 同時給排序與 consecutive_failures 用。
	// 讀兩次除了多一次查詢，還會讓兩處在中途失敗時看到不一致的東西。
	//
	// ⛔ **讀失敗不能只記 log**：退化成「全部視為從未嘗試」會讓同一批 symbol 每輪都被
	// 選中（其他候選餓死），而 job 仍顯示 success；而且寫回時若把缺少的 prior 當成 0，
	// 會把既有的 consecutive_failures 覆寫掉，「這一檔一直驗不成功」就此消失。
	// **所以讀失敗要 degraded，而且該輪不寫回 state**——寧可不寫，也不要寫錯的累積值。
	priorStates, stateErr := s.candleVerification.LoadStates(
		ctx, evaluationUniverseTimeframe, symbols)
	if stateErr != nil {
		s.log.Error("candle verification state 讀取失敗", zap.Error(stateErr))
		errParts = append(errParts, "verification_state_read_failed")
		degraded = true
		priorStates = map[string]store.CandleVerificationState{}
	}

	// ── 一、aggregate 短路 ──────────────────────────────────────────────────
	//
	// **最該被抓到的情境，剛好也是請求量最大的情境**：上游某天整批漏給 → 全池都有候選
	// → 逐檔逐月查個股端點 → 單輪最多 135 × 月份數 個請求打向交易所。
	// 全池同一天缺漏是**來源層級**的訊號，不需要逐檔確認。
	remaining, aggregated := s.applyAggregateShortCircuit(candidates, states, now, attempts)
	for _, msg := range aggregated {
		errParts = append(errParts, msg)
		degraded = true
	}

	// ── 二、逐檔核對（公平排序 ＋ 掃描取 cap） ──────────────────────────────
	gapCount, unavailable, skippedByBreaker := s.verifyCandidates(ctx, remaining, now, attempts, priorStates)
	if gapCount > 0 {
		errParts = append(errParts, fmt.Sprintf("upstream_data_gap: %d 筆缺漏已確認", gapCount))
		degraded = true
	}
	if unavailable > 0 {
		errParts = append(errParts, fmt.Sprintf("verification_unavailable: %d 筆驗不了", unavailable))
		degraded = true
	}
	// ⛔ **只要有任何候選因 breaker 被跳過，該輪就是 partial**。原本只寫「整輪完全沒驗到
	// 才記 partial」留了一個洞：部分候選被跳過但其他驗成功時會顯示 success，
	// 又變成「有一部分驗不了但看起來正常」。
	if skippedByBreaker > 0 {
		errParts = append(errParts,
			fmt.Sprintf("verification_unavailable: breaker open, %d 檔未驗", skippedByBreaker))
		degraded = true
	}

	// ── 三、寫回公平排序簿記 ────────────────────────────────────────────────
	// **讀失敗時不寫回**：prior 是空的，寫下去會把既有的 consecutive_failures 抹掉。
	if stateErr == nil {
		if err := s.recordAttempts(ctx, attempts, priorStates); err != nil {
			// **不中斷該輪**，但要看得見：更新若永遠失敗，排序鍵不會前進而退回飢餓，
			// 而飢餓避免的上界 ceil(N/cap) 正是以「寫入成功」為前提。
			errParts = append(errParts, "verification_state_write_failed")
			degraded = true
		}
	}

	s.log.Info("candle gap detection done",
		zap.Int("pool", len(symbols)), zap.Int("candidates", len(candidates)),
		zap.Int("gap", gapCount), zap.Int("unavailable", unavailable),
		zap.Int("deferred", deferred), zap.Int("breaker_skipped", skippedByBreaker),
		zap.String("source_as_of", sourceAsOf))

	// **error 欄要合併不得覆蓋**：finishRunDegraded 原樣傳遞 lastErr、只改 status，
	// 所以合併是呼叫端的責任（比照 corporate_action_sync 的 strings.Join(errParts, "; ")）。
	s.finishRunDegraded(ctx, runID, candleGapDetectionJob,
		len(symbols), 0, strings.Join(errParts, "; "), degraded)
}

// collectGapCandidates 算出「預期交易日 − 實際有 K 棒的日期」的差集。
func (s *Scheduler) collectGapCandidates(
	ctx context.Context, symbols []string, states map[string]store.StockSymbolState,
) ([]gapCandidate, *market.TradingCalendar, []string, error) {
	today := timeutil.TodayTaipei()

	// **預期集合來自靜態年度日曆**，不是市場層級端點——後者會有發布延遲，而且
	// 「成功但陳舊」無法自證。日曆整年預先公布，不會停滯。
	//
	// **兩市場共用同一份日曆**（具名假設）：台灣兩市場的開休市日實務上一致。
	// 單一市場臨時休市時該市場當天會整批落進 aggregate 告警，那是可接受的降級。
	// **跨年視窗要載入兩個年度**：回看視窗在年初會跨到前一年，只載當年日曆的話，
	// 去年 12 月的假日會被當成交易日 → 那幾天全部誤報成缺 K。
	// 用寬鬆的日曆天估算涵蓋範圍（交易日 ≈ 日曆天的 5/7，取 2 倍再加緩衝一定夠）。
	earliest := today.AddDate(0, 0, -(s.candleGapCfg.LookbackTradingDays*2 + 10))
	cals := make([]*market.TradingCalendar, 0, 2)
	for year := earliest.Year(); year <= today.Year(); year++ {
		c, err := s.exchangeReference.TradingCalendar(ctx, year)
		if err != nil {
			return nil, nil, nil, err
		}
		cals = append(cals, c)
	}
	cal := market.NewMergedTradingCalendar(cals...)
	expected := s.expectedTradingDays(cal, today)
	if len(expected) == 0 {
		return nil, nil, nil, errors.New("預期交易日集合為空")
	}
	// 視窗左界取預期集合的第一天，右界是今天（半開區間要 +1 天）。
	from, err := time.ParseInLocation("2006-01-02", expected[0], timeutil.TaipeiTZ)
	if err != nil {
		return nil, nil, nil, err
	}

	actual, err := s.evaluationUniverseCandles.CandleDatesInRange(
		ctx, symbols, evaluationUniverseTimeframe, from, today.AddDate(0, 0, 1))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("實際日期集合查詢失敗: %w", err)
	}

	out := make([]gapCandidate, 0, 16)
	for _, symbol := range symbols {
		have := make(map[string]bool, len(actual[symbol]))
		for _, d := range actual[symbol] {
			have[d] = true
		}
		for _, d := range expected {
			if have[d] {
				continue
			}
			out = append(out, gapCandidate{Symbol: symbol, Market: states[symbol].Market, Date: d})
		}
	}
	return out, cal, expected, nil
}

// marketSourceAsOf 取得對照源涵蓋到哪一天，並判定它是不是「成功但陳舊」。
//
// ⛔ **不能單獨拿回傳的最後一個交易日當稽核右界**：端點若回應成功、格式正常、
// 內容卻停滯數日，視窗會跟著一起倒退，所有比它新的缺口都不會被檢查，
// **而且不會被歸類為「驗證不可用」**——偵測機制本身靜默失效，
// 正是本筆要防的失效模式在偵測器上重演。
//
// 所以要有一個**不依賴該端點**的基準：用年度日曆推導「今天為止的預期最後交易日」，
// 兩者相減得到落後的**交易日數**。
//
//	lag = |{ 交易日 d : source_as_of < d <= expected_last_trading_day }|
//
// ⚠️ **單位是交易日不是日曆日**：週五的資料到週一才更新，日曆日差是 3、
// 實際只落後一個交易時段。用日曆日會讓每個週一都誤報一次。
//
// ⚠️ **比較符號是 `>=` 不是 `>`**：寫成 `>` 會讓剛好等於門檻時漏報。
// 預設 market_stale_days=2 的語意因此是「容忍一個交易日的發布延遲」——
// lag=1（今天的還沒發）不算過期，lag>=2（連昨天的都沒有）才算。
func (s *Scheduler) marketSourceAsOf(
	ctx context.Context, cal *market.TradingCalendar, expected []string,
) (string, error) {
	sourceAsOf, err := s.exchangeReference.MarketLastTradingDate(ctx)
	if err != nil {
		return "", fmt.Errorf("市場層級對照源不可用: %w", err)
	}
	asOf, err := time.ParseInLocation("2006-01-02", sourceAsOf, timeutil.TaipeiTZ)
	if err != nil {
		return "", fmt.Errorf("市場層級對照源日期無法解析 %q", sourceAsOf)
	}
	// expected 是升冪的預期交易日，最後一個就是「今天為止應有的最後一個交易日」。
	expectedLast, err := time.ParseInLocation("2006-01-02", expected[len(expected)-1], timeutil.TaipeiTZ)
	if err != nil {
		return "", err
	}
	lag := cal.TradingDaysBetween(asOf, expectedLast)
	if lag >= s.candleGapCfg.MarketStaleDays {
		return "", fmt.Errorf(
			"市場層級對照源陳舊: source_as_of=%s 落後 %d 個交易日（門檻 %d）",
			sourceAsOf, lag, s.candleGapCfg.MarketStaleDays)
	}
	return sourceAsOf, nil
}

// recordAttempts 把本輪的結論寫回公平排序簿記。
//
// **prior 由呼叫端傳入**（整輪只讀一次的那份）：consecutive_failures 是「連續」的計數，
// 沒有前值就只能從 0 起算，而那會讓「這一檔一直驗不成功」永遠看不出來。
func (s *Scheduler) recordAttempts(
	ctx context.Context, attempts map[string]*gapAttemptAgg,
	prior map[string]store.CandleVerificationState,
) error {
	if len(attempts) == 0 {
		return nil
	}
	batch := make([]store.VerificationAttempt, 0, len(attempts))
	for _, symbol := range sortedKeys(attempts) {
		batch = append(batch, attempts[symbol].toAttempt(prior[symbol]))
	}
	if err := s.candleVerification.RecordAttempts(ctx, batch); err != nil {
		s.log.Error("candle verification state 寫入失敗", zap.Error(err))
		return err
	}
	return nil
}

// expectedTradingDays 由日曆往回數出 lookback_trading_days 個交易日（升冪）。
//
// **單位是交易日不是日曆日**：10 個日曆日只含約 6～7 個交易日，兩者跨週末時涵蓋範圍不同。
// 連假因此不會縮短實際涵蓋的交易日數。
func (s *Scheduler) expectedTradingDays(cal *market.TradingCalendar, today time.Time) []string {
	want := s.candleGapCfg.LookbackTradingDays
	days := make([]string, 0, want)
	// **從昨天開始往回**：今天的日 K 在 16:00 這輪本來就可能還沒寫進去，
	// 把當天算進預期集合會讓每一輪都誤報整池。
	for d := today.AddDate(0, 0, -1); len(days) < want; d = d.AddDate(0, 0, -1) {
		if cal.IsTradingDay(d) {
			days = append(days, d.Format("2006-01-02"))
		}
		// 防呆：日曆若異常把整段都標成非交易日，不要無限往回走。
		if today.Sub(d) > 200*24*time.Hour {
			break
		}
	}
	sort.Strings(days)
	return days
}

// applyAggregateShortCircuit 把「單一 (market, date) 缺漏比例達標」的分組短路掉。
//
// **判定維度是 `(market, 缺漏日期)`，不是缺口總筆數**：拿總筆數當門檻的話，
// **不同日期的零星缺口累加起來也會過門檻**，被誤判成單日的來源層級缺漏。
//
// ⛔ **比例還不夠，必須加最小母體門檻**：某市場有效池只剩 1 檔時，那一檔合法停止買賣
// 比例就是 100%，會被短路成來源級告警——直接違反「2867 這類不得告警」的要求。
func (s *Scheduler) applyAggregateShortCircuit(
	candidates []gapCandidate, states map[string]store.StockSymbolState,
	now time.Time, attempts map[string]*gapAttemptAgg,
) ([]gapCandidate, []string) {
	// 各市場的有效池大小＝分母。
	poolByMarket := make(map[string]int, 4)
	for _, st := range states {
		poolByMarket[st.Market]++
	}

	type groupKey struct{ market, date string }
	groups := make(map[groupKey]map[string]bool, len(candidates))
	for _, c := range candidates {
		k := groupKey{c.Market, c.Date}
		if groups[k] == nil {
			groups[k] = make(map[string]bool)
		}
		groups[k][c.Symbol] = true
	}

	shorted := make(map[groupKey]bool, len(groups))
	var messages []string
	for k, symbols := range groups {
		pool := poolByMarket[k.market]
		// 小母體強制逐檔——成本可忽略，而誤報的代價是使用者從此忽略告警。
		if pool < s.candleGapCfg.AggregateMinSymbols {
			continue
		}
		ratio := float64(len(symbols)) / float64(pool)
		// **門檻是 >= 不是 >**，明文定死避免實作各自解讀。
		if ratio < s.candleGapCfg.AggregateRatio {
			continue
		}
		shorted[k] = true
		messages = append(messages, fmt.Sprintf(
			"upstream_data_gap: 市場層級缺漏 market=%s date=%s %d/%d 檔",
			k.market, k.date, len(symbols), pool))
		// **短路的那批也要寫 state**——理由是**公平排序的簿記**，不是去重告警。
		// 它們日後仍會是候選（缺口部分修復後 aggregate 不再觸發），不更新
		// last_attempted_at 的話會永遠排在最前面並佔滿 cap，把其他候選餓死。
		for symbol := range symbols {
			mergeAttempt(attempts, symbol, now, store.VerificationGap, true)
		}
	}
	if len(shorted) == 0 {
		return candidates, messages
	}

	remaining := make([]gapCandidate, 0, len(candidates))
	for _, c := range candidates {
		if shorted[groupKey{c.Market, c.Date}] {
			continue
		}
		remaining = append(remaining, c)
	}
	// 排序後的訊息比較好讀，也讓測試不必處理 map 迭代順序。
	sort.Strings(messages)
	return remaining, messages
}

// symbolCandidate 是一個候選標的**在本輪要驗的全部月份**。
//
// ⚠️ **cap 的單位是候選標的，不是 HTTP 請求**（契約明訂）。所以扣額度必須以這個結構
// 為單位——一個 symbol 的所有月份要嘛全做、要嘛全不做。
//
// 按月份扣額度會壞掉三件事：cap=20 在跨月時只處理得到 10 檔；
// **可能在同一 symbol 的月份中途停止**，於是拿一半的結果去做跨月 coalesce；
// 以及 ceil(N/cap) 的公平上界不再成立（它算的是候選數）。
type symbolCandidate struct {
	symbol string
	market string
	// months 依 (year, month) 升冪，每個月份帶上該月缺漏的日期。
	months []monthCandidate
}

type monthCandidate struct {
	year  int
	month time.Month
	dates []string
}

// verifyCandidates 依公平排序**掃描**候選並逐檔核對，回傳（確認缺口數, 驗不了數, 被 breaker 跳過數, state 是否可用）。
//
// ⚠️ **掃描而不是預先截斷**：若先取前 cap 個、處理到一半某來源才斷路，剩下的只會被跳過，
// **清單之外的其他來源候選不會自動遞補**，該輪等於只驗了幾個。所以要算出完整排序清單
// 再逐項掃描，**實際 attempt 數**達到 cap 才停。
func (s *Scheduler) verifyCandidates(
	ctx context.Context, candidates []gapCandidate,
	now time.Time, attempts map[string]*gapAttemptAgg,
	states map[string]store.CandleVerificationState,
) (gapCount, unavailable, skippedByBreaker int) {
	if len(candidates) == 0 {
		return 0, 0, 0
	}
	order := groupCandidatesBySymbol(candidates)
	sortSymbolCandidatesByFairness(order, states)

	attempted := 0
	for _, sc := range order {
		if attempted >= s.candleGapCfg.CandidateCapPerRun {
			break
		}
		source := sourceForMarket(sc.market)
		// **breaker 已開 → 跳過，不計入 cap、不更新 last_attempted_at**，繼續往後掃，
		// 由其他來源的候選補滿 cap。
		if source != "" && s.exchangeReference.IsSourceOpen(source) {
			skippedByBreaker++
			continue
		}
		// **一個候選佔一個額度，不論它跨幾個月**。
		attempted++

		// 同一個 symbol 的所有月份一次做完，之後才 coalesce——中途停止會讓結論建立在
		// 不完整的資料上（例如只驗了 7 月就宣告 verified，而 8 月的缺口沒被看過）。
		for _, mc := range sc.months {
			traded, err := s.exchangeReference.StockTradedDates(ctx, sc.symbol, sc.market, mc.year, mc.month)
			if err != nil {
				unavailable++
				mergeAttempt(attempts, sc.symbol, now, store.VerificationUnavailable, false)
				continue
			}
			// **缺口＝交易所有成交、我們沒有**。交易所那天也沒成交就是正常，不告警。
			hasGap := false
			for _, d := range mc.dates {
				if traded[d] {
					hasGap = true
					gapCount++
				}
			}
			result := store.VerificationVerified
			if hasGap {
				result = store.VerificationGap
			}
			// gap 也是**成功的驗證**，只是結論是壞消息。
			mergeAttempt(attempts, sc.symbol, now, result, true)
		}
	}
	return gapCount, unavailable, skippedByBreaker
}

// groupCandidatesBySymbol 把候選整理成「每檔要驗哪幾個月、每個月缺哪幾天」。
//
// **(symbol, month) 去重**：同一檔在同一個月的多天缺口只需要一次請求
// （TWSE 與 TPEx 的個股端點都是按月回傳）。
func groupCandidatesBySymbol(candidates []gapCandidate) []*symbolCandidate {
	bySymbol := make(map[string]*symbolCandidate, len(candidates))
	order := make([]*symbolCandidate, 0, len(candidates))
	monthIdx := make(map[monthKey]int, len(candidates))

	for _, c := range candidates {
		day, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			continue
		}
		sc, ok := bySymbol[c.Symbol]
		if !ok {
			sc = &symbolCandidate{symbol: c.Symbol, market: c.Market}
			bySymbol[c.Symbol] = sc
			order = append(order, sc)
		}
		k := monthKey{c.Symbol, c.Market, day.Year(), day.Month()}
		idx, ok := monthIdx[k]
		if !ok {
			sc.months = append(sc.months, monthCandidate{year: day.Year(), month: day.Month()})
			idx = len(sc.months) - 1
			monthIdx[k] = idx
		}
		sc.months[idx].dates = append(sc.months[idx].dates, c.Date)
	}
	for _, sc := range order {
		sort.Slice(sc.months, func(i, j int) bool {
			if sc.months[i].year != sc.months[j].year {
				return sc.months[i].year < sc.months[j].year
			}
			return sc.months[i].month < sc.months[j].month
		})
	}
	return order
}

// sortSymbolCandidatesByFairness 依公平排序鍵排出處理順序。
//
// 排序鍵：**(有沒有 state：無者優先, last_attempted_at 由舊到新, symbol)**。
//
// ⚠️ **排序鍵是 last_attempted_at 不是 last_verified_at**：只在成功後更新的話，
// 持續失敗的前 cap 個會永遠保持最舊、每輪繼續佔滿配額，後面的候選還是永遠輪不到——
// 那正好是這條規則要解決的問題本身。
//
// ⚠️ **合併與排序都在 Go 端做，不靠 SQL**：「沒有列」不等於「欄位為 NULL」，
// 首次出現的候選根本不會被 SELECT 回傳，`NULLS FIRST` 解不了；而且 MySQL 不支援它。
//
// **symbol 決勝**讓順序是決定性的——否則同一批候選每輪的順序都不同，
// ceil(N/cap) 的上界就無從論證，測試也會不穩定。
func sortSymbolCandidatesByFairness(
	order []*symbolCandidate, states map[string]store.CandleVerificationState,
) {
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		sa, oka := states[a.symbol]
		sb, okb := states[b.symbol]
		if oka != okb {
			return !oka // 沒有 state 的排前面
		}
		if oka {
			at, bt := sa.LastAttemptedAt, sb.LastAttemptedAt
			if at.Valid != bt.Valid {
				return !at.Valid // 有列但從未嘗試過的同樣排前面
			}
			if at.Valid && !at.Time.Equal(bt.Time) {
				return at.Time.Before(bt.Time)
			}
		}
		return a.symbol < b.symbol
	})
}

// sourceForMarket 把市場別對到 breaker 的來源代號。
func sourceForMarket(mkt string) string {
	switch {
	case strings.Contains(mkt, "上櫃"):
		return market.SourceTPExStockDay
	case strings.Contains(mkt, "上市"):
		return market.SourceTWSEStockDay
	default:
		return ""
	}
}

// gapAttemptAgg 是「本輪這個 symbol 的所有結果」的累加器。
//
// **不能只累加 last_result**：coalesce 規則對四個欄位各有各的算法，
// 而 consecutive_failures 的判準是「**沒有任何成功，且至少一個 unavailable**」——
// 那需要同時記住「有沒有成功過」與「有沒有真的失敗過」，單一個字串表達不了。
type gapAttemptAgg struct {
	symbol         string
	lastAttempted  time.Time
	verifiedAt     time.Time
	result         string
	anySuccess     bool
	anyUnavailable bool
}

// toAttempt 依 coalesce 規則產生要寫回的那一筆。
//
//	last_attempted_at      本輪最後一次 attempt 的時間
//	last_verified_at       只要有任何一次成功驗證（verified 或 gap）就更新
//	last_result            取最嚴重：unavailable > gap > deferred > verified
//	consecutive_failures   有任何成功 → 歸零；沒有任何成功且至少一個 unavailable → +1；
//	                       其餘（只有 deferred）不動
//
// ⚠️ **不能寫成「全部 unavailable 才 +1」**：一個月份請求真的失敗、另一個月份 deferred 時，
// 整體 last_result 是 unavailable 但「全部 unavailable」不成立，
// 於是這個 symbol 的 consecutive_failures 永遠不會增加，
// 「這一檔一直驗不成功」在 state 上就看不出來。
func (a *gapAttemptAgg) toAttempt(prior store.CandleVerificationState) store.VerificationAttempt {
	out := store.VerificationAttempt{
		Symbol:          a.symbol,
		Timeframe:       evaluationUniverseTimeframe,
		LastAttemptedAt: a.lastAttempted,
		LastResult:      a.result,
	}
	// **零值代表本輪沒有任何成功驗證**，repo 會用 COALESCE 保留資料庫既有的值，
	// 不會把先前的成功時間抹掉。
	if a.anySuccess {
		out.LastVerifiedAt = a.verifiedAt
	}
	switch {
	case a.anySuccess:
		out.ConsecutiveFailures = 0
	case a.anyUnavailable:
		out.ConsecutiveFailures = prior.ConsecutiveFailures + 1
	default:
		// 只有 deferred：正常的發布延遲不該把失敗計數推上去。
		out.ConsecutiveFailures = prior.ConsecutiveFailures
	}
	return out
}

// mergeAttempt 把一次結果併進該 symbol 的累加器。
func mergeAttempt(
	attempts map[string]*gapAttemptAgg,
	symbol string, now time.Time, result string, succeeded bool,
) {
	cur, ok := attempts[symbol]
	if !ok {
		cur = &gapAttemptAgg{symbol: symbol}
		attempts[symbol] = cur
	}
	cur.lastAttempted = now
	if succeeded {
		cur.verifiedAt = now
		cur.anySuccess = true
	}
	if result == store.VerificationUnavailable {
		cur.anyUnavailable = true
	}
	if cur.result == "" || verificationSeverity(result) > verificationSeverity(cur.result) {
		cur.result = result
	}
}

func verificationSeverity(result string) int {
	switch result {
	case store.VerificationUnavailable:
		return 3
	case store.VerificationGap:
		return 2
	case store.VerificationDeferred:
		return 1
	case store.VerificationVerified:
		return 0
	}
	return -1
}

// sortedKeys 讓寫回批次的順序是決定性的——測試與 log 都好讀，
// 也避免 map 迭代順序讓兩次執行的錯誤訊息不同。
func sortedKeys(m map[string]*gapAttemptAgg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
