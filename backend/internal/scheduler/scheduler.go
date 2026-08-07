package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// srZoneVerifyLimit 每次收盤驗證最多處理幾筆最近的 SR zone 分析，避免隨著
// 歷史分析越積越多，這個 job 的執行時間跟著無上限成長（見 RunDailyClose）。
const srZoneVerifyLimit = 50

// stockSymbolSyncTimeout 是整個 stock_symbol_sync job 的上限（抓取兩個來源 + 寫入快照）。
// 單次 HTTP 請求另有 client timeout（預設 300 秒／來源），這層是最後防線：來源異常慢或
// DB 卡住時，job 不會無限期停在 running（job_runs 的 stale 判定要 26 小時才會亮）。
// 20 分鐘 = 2 來源 × 300 秒 + 來源間隔 + 寫入快照，仍有餘裕。
const stockSymbolSyncTimeout = 20 * time.Minute

type Scheduler struct {
	fetcher          *market.Fetcher
	signalEng        *signal.Engine
	watchlist        store.WatchlistRepo
	jobRuns          store.JobRunRepo
	srZoneRepo       store.SRZoneRepo
	srZoneVerifier   *analysis.SRZoneVerifier
	chipSyncer       *chip.Syncer
	chipSyncCron     string
	stockSyncer      *market.StockSymbolSyncer
	stockSyncCron    string
	stockSyncEnabled bool
	analysisClient   *analysis.Client
	srEvaluationJobs store.SREvaluationJobRepo
	chipScores       store.ChipScoreRepo
	modelGovernance  store.SRModelGovernanceRepo
	srEvaluation     config.SREvaluationConfig
	intradayEnabled  bool
	log              *zap.Logger
	cron             *cron.Cron
}

func New(
	fetcher *market.Fetcher,
	signalEng *signal.Engine,
	watchlist store.WatchlistRepo,
	jobRuns store.JobRunRepo,
	srZoneRepo store.SRZoneRepo,
	srZoneVerifier *analysis.SRZoneVerifier,
	chipSyncer *chip.Syncer,
	chipSyncCron string,
	stockSyncer *market.StockSymbolSyncer,
	stockSyncCron string,
	stockSyncEnabled bool,
	analysisClient *analysis.Client,
	srEvaluationJobs store.SREvaluationJobRepo,
	chipScores store.ChipScoreRepo,
	modelGovernance store.SRModelGovernanceRepo,
	srEvaluation config.SREvaluationConfig,
	intradayEnabled bool,
	log *zap.Logger,
) *Scheduler {
	return &Scheduler{
		fetcher:          fetcher,
		signalEng:        signalEng,
		watchlist:        watchlist,
		jobRuns:          jobRuns,
		srZoneRepo:       srZoneRepo,
		srZoneVerifier:   srZoneVerifier,
		chipSyncer:       chipSyncer,
		chipSyncCron:     chipSyncCron,
		stockSyncer:      stockSyncer,
		stockSyncCron:    stockSyncCron,
		stockSyncEnabled: stockSyncEnabled,
		analysisClient:   analysisClient,
		srEvaluationJobs: srEvaluationJobs,
		chipScores:       chipScores,
		modelGovernance:  modelGovernance,
		srEvaluation:     srEvaluation,
		intradayEnabled:  intradayEnabled,
		log:              log,
		cron:             cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
	}
}

func (s *Scheduler) Start() {
	// 盤前初始化：補齊近 5 天日K 缺口 + 預熱日線指標
	s.cron.AddFunc("50 8 * * 1-5", func() {
		s.runPreMarket()
	})

	// 盤中：每 5 分鐘拉取分K + 計算指標 + Signal 掃描（IsMarketOpen 守衛 13:30 收盤）
	s.cron.AddFunc("*/5 9-13 * * 1-5", func() {
		s.runIntradayJob()
	})

	// 收盤後：拉日K + 完整掃描。收盤是 13:30，這裡刻意等到 15:00 才拉，
	// 是因為 FinMind TaiwanStockPrice 當天日K不會在收盤當下就立刻發布——
	// 曾經在 14:00 整拉到 count=0（請求成功但資料還沒發布，BulkInsert 對
	// 空陣列直接視為成功，job_runs 也顯示 success，不會有任何錯誤訊號）。
	// 15:00 給 FinMind 更多緩衝時間，仍抓空的話可用 RunDailyClose 手動重拉
	// （見 handler.SchedulerHandler.RunDailyClose）。
	s.cron.AddFunc("0 15 * * 1-5", func() {
		s.RunDailyClose()
	})

	// 籌碼採集：與 15:00 收盤掃描解耦，改為傍晚獨立排程（預設 21:00，見
	// config chip.sync.cron）。FinMind 法人資料收盤後傍晚、融資融券更要晚間
	// 才由 TWSE 發布，若沿用 15:00 會抓到空資料、只能靠隔天 lookback 回補，
	// 造成資料庫永遠落後一天（見 docs/chip-analysis-design.md 第8節）。
	if _, err := s.cron.AddFunc(s.chipSyncCron, func() {
		s.runChipDailySync(context.Background())
	}); err != nil {
		s.log.Error("chip sync cron register failed", zap.String("cron", s.chipSyncCron), zap.Error(err))
	}

	if s.stockSyncEnabled && s.stockSyncer != nil {
		if _, err := s.cron.AddFunc(s.stockSyncCron, func() {
			s.RunStockSymbolSync()
		}); err != nil {
			s.log.Error("stock symbol sync cron register failed", zap.String("cron", s.stockSyncCron), zap.Error(err))
		}
	}

	if s.srEvaluation.Enabled {
		if _, err := s.cron.AddFunc(s.srEvaluation.Cron, func() {
			s.RunSREvaluation()
		}); err != nil {
			s.log.Error("sr evaluation cron register failed", zap.String("cron", s.srEvaluation.Cron), zap.Error(err))
		}
	}

	s.cron.Start()
	s.log.Info("scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// startRun 記錄一筆排程執行紀錄，失敗時只記 log（不影響排程本身執行）
func (s *Scheduler) startRun(ctx context.Context, jobName string) uint64 {
	runID, err := s.jobRuns.Start(ctx, jobName)
	if err != nil {
		s.log.Error("job_runs start failed", zap.String("job", jobName), zap.Error(err))
	}
	return runID
}

// finishRun 依失敗數量換算 status 並寫回執行紀錄
func (s *Scheduler) finishRun(ctx context.Context, runID uint64, jobName string, total, failed int, lastErr string) {
	status := "success"
	switch {
	case total > 0 && failed >= total:
		status = "failed"
	case failed > 0:
		status = "partial"
	}
	if err := s.jobRuns.Finish(ctx, runID, status, total, failed, lastErr); err != nil {
		s.log.Error("job_runs finish failed", zap.String("job", jobName), zap.Error(err))
	}
}

func (s *Scheduler) runPreMarket() {
	ctx := context.Background()

	// 只保留當天的排程執行紀錄，開盤前先清掉前幾天的舊資料
	if n, err := s.jobRuns.DeleteBefore(ctx, timeutil.TodayTaipei()); err != nil {
		s.log.Warn("job_runs cleanup failed", zap.Error(err))
	} else if n > 0 {
		s.log.Info("job_runs cleanup done", zap.Int64("deleted", n))
	}

	runID := s.startRun(ctx, "pre_market")
	s.log.Info("pre-market job started")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "pre_market", 0, 0, err.Error())
		return
	}

	// 補齊近 5 天日K（涵蓋週末 / 假日缺口），BulkInsert 有 UNIQUE 保護不會重複
	failed := s.fetcher.BackfillHistory(ctx, symbols, 5, nil)

	// 預熱日線指標，讓第一根分K掃描前就有 MA / RSI / MACD 基準值
	for _, sym := range symbols {
		s.signalEng.Evaluate(ctx, sym, "1d")
	}
	s.log.Info("pre-market job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))

	lastErr := ""
	if failed > 0 {
		lastErr = "backfill failed for some symbols"
	}
	s.finishRun(ctx, runID, "pre_market", len(symbols), failed, lastErr)
}

func (s *Scheduler) runIntradayJob() {
	if !timeutil.IsMarketOpen(time.Now()) {
		return
	}
	// 有掛載批次盤中源（Yahoo）時優先走批次路徑（免 token）；未掛載才退回 FinMind 分K。
	if s.fetcher.HasIntradaySource() {
		s.runIntradayBatch()
		return
	}
	if !s.intradayEnabled {
		// finmind.intraday_enabled=false（預設）且無批次源：帳號等級不足以使用
		// TaiwanStockKBar dataset，不建立 job_run 紀錄，避免每 5 分鐘
		// 洗一筆「skipped」進資料庫；升級帳號或啟用 Yahoo 後即可恢復
		s.log.Debug("intraday job skipped: no batch source and finmind.intraday_enabled=false")
		return
	}

	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "intraday", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for i, sym := range symbols {
		if err := s.fetcher.FetchAndStoreMinute(ctx, sym, today); err != nil {
			if errors.Is(err, market.ErrInsufficientTier) {
				// 帳號等級不足是整個 token 的限制，對其他 symbol 重試也一定會失敗，
				// 記一次 log 後整輪跳過，避免每 5 分鐘對 watchlist 每檔股票都打一次注定失敗的請求
				s.log.Warn("intraday job skipped: finmind token tier insufficient", zap.Error(err))
				if ferr := s.jobRuns.Finish(ctx, runID, "skipped", len(symbols), len(symbols)-i, err.Error()); ferr != nil {
					s.log.Error("job_runs finish failed", zap.String("job", "intraday"), zap.Error(ferr))
				}
				return
			}
			s.log.Warn("intraday fetch failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
		s.signalEng.Evaluate(ctx, sym, "1m")
	}
	s.finishRun(ctx, runID, "intraday", len(symbols), failed, lastErr)
}

// runIntradayBatch 為批次盤中源（Yahoo）版本的盤中 job：把 watchlist 依 batch_size
// 分批、每批一次請求拉多檔 1 分K 並寫入 candles，再逐檔跑 signal 掃描。
func (s *Scheduler) runIntradayBatch() {
	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "intraday", 0, 0, err.Error())
		return
	}

	batchSize := s.fetcher.IntradayBatchSize()
	if batchSize <= 0 {
		batchSize = len(symbols)
	}

	failed := 0
	lastErr := ""
	for start := 0; start < len(symbols); start += batchSize {
		end := start + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[start:end]
		if _, err := s.fetcher.FetchAndStoreIntradayBatch(ctx, batch); err != nil {
			// 批次請求失敗（例如 Yahoo 被限流/封鎖）：記錄後續跑其他批次。
			// 未來的 Yahoo→FinMind fallback（T-008/T-031）會在此改為回退補資料。
			s.log.Warn("intraday batch fetch failed", zap.Int("from", start), zap.Int("size", len(batch)), zap.Error(err))
			failed += len(batch)
			lastErr = err.Error()
		}
	}

	for _, sym := range symbols {
		s.signalEng.Evaluate(ctx, sym, "1m")
	}
	s.finishRun(ctx, runID, "intraday", len(symbols), failed, lastErr)
}

// RunDailyClose 執行「收盤後拉日K + 完整掃描」的邏輯，供 cron 排程與
// 人工手動觸發（例如排程時間點 FinMind 剛好還沒發布資料時）共用同一份實作。
func (s *Scheduler) RunDailyClose() {
	ctx := context.Background()
	runID := s.startRun(ctx, "daily_close")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "daily_close", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for _, sym := range symbols {
		if err := s.fetcher.FetchAndStoreDaily(ctx, sym, today); err != nil {
			s.log.Warn("daily fetch failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
		s.signalEng.Evaluate(ctx, sym, "1d")
	}
	s.log.Info("daily close job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "daily_close", len(symbols), failed, lastErr)

	// SR zone 驗證是獨立的 job_run 紀錄，失敗不影響上面已經完成的 daily_close
	// 結果——兩者依序執行但彼此獨立記錄，其中一個出問題不會讓另一個也跟著
	// 判定失敗。
	s.runSRZoneVerification(ctx)

	// 註：籌碼同步（chip_daily_sync）已從此處解耦，改由傍晚獨立 cron 觸發
	// （見 Start() 的 s.chipSyncCron）。法人/融資融券資料比日K更晚發布，
	// 15:00 收盤這批太早，沿用會抓到空資料造成落後一天。
}

// runChipDailySync 對 watchlist 全部股票執行當日籌碼同步（chip.Syncer.SyncDaily），
// 完全比照 runSRZoneVerification 的結構：單筆失敗只記錄、不中斷其他股票的同步。
func (s *Scheduler) runChipDailySync(ctx context.Context) {
	runID := s.startRun(ctx, "chip_daily_sync")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "chip_daily_sync", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for _, sym := range symbols {
		if err := s.chipSyncer.SyncDaily(ctx, sym, today); err != nil {
			s.log.Warn("chip daily sync failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
	}
	s.log.Info("chip daily sync job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "chip_daily_sync", len(symbols), failed, lastErr)
}

func (s *Scheduler) RunStockSymbolSync() {
	s.runStockSymbolSync(context.Background())
}

func (s *Scheduler) runStockSymbolSync(ctx context.Context) {
	runID := s.startRun(ctx, "stock_symbol_sync")
	if s.stockSyncer == nil {
		errMsg := "stock symbol syncer is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "stock_symbol_sync", 0, 1, errMsg)
		return
	}

	// 只有同步本身套 timeout；startRun / finishRun 仍用原本的 ctx，
	// 否則 sync 逾時後連「把這次 job 標記成失敗」的 DB 寫入都會一起被取消。
	syncCtx, cancel := context.WithTimeout(ctx, stockSymbolSyncTimeout)
	defer cancel()

	result, err := s.stockSyncer.Sync(syncCtx, time.Now().In(timeutil.TaipeiTZ))
	if err != nil {
		s.log.Error("stock symbol sync failed", zap.Error(err))
		s.finishRun(ctx, runID, "stock_symbol_sync", 0, 1, err.Error())
		return
	}
	s.finishRun(ctx, runID, "stock_symbol_sync", result.Seen, 0, "")
}

// runSRZoneVerification 對最近 srZoneVerifyLimit 筆 SR zone 分析重新驗證
// zone 有沒有被突破（見 internal/analysis/sr_zone_verifier.go）。跟
// indicator/signal 排程一樣，單筆驗證失敗只記錄、不中斷其他分析的驗證。
func (s *Scheduler) runSRZoneVerification(ctx context.Context) {
	runID := s.startRun(ctx, "sr_zone_verify")

	analyses, err := s.srZoneRepo.List(ctx, "", srZoneVerifyLimit)
	if err != nil {
		s.log.Error("sr zone list failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_zone_verify", 0, 0, err.Error())
		return
	}

	failed := 0
	lastErr := ""
	for _, a := range analyses {
		if _, _, err := s.srZoneVerifier.Verify(ctx, a.ID); err != nil {
			s.log.Warn("sr zone verify failed", zap.Uint64("analysis_id", a.ID), zap.String("symbol", a.Symbol), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
	}
	s.log.Info("sr zone verification job completed", zap.Int("analyses", len(analyses)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "sr_zone_verify", len(analyses), failed, lastErr)
}

func (s *Scheduler) RunSREvaluation() {
	s.runSREvaluation(context.Background())
}

func (s *Scheduler) runSREvaluation(ctx context.Context) {
	runID := s.startRun(ctx, "sr_evaluation")

	symbols, err := s.srEvaluationSymbols(ctx)
	if err != nil {
		s.log.Error("sr evaluation symbols failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", 1, 1, err.Error())
		return
	}
	if len(symbols) == 0 {
		errMsg := "sr evaluation symbols is empty"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", 1, 1, errMsg)
		return
	}
	if s.analysisClient == nil {
		errMsg := "sr evaluation analysis client is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), errMsg)
		return
	}
	if s.srEvaluationJobs == nil {
		errMsg := "sr evaluation job repo is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), errMsg)
		return
	}

	request := s.srEvaluationRequest(symbols)
	analysis.PopulateSREvaluationReplayContext(ctx, &request, s.chipScores, s.modelGovernance, s.log)

	symbolsJSON, err := json.Marshal(symbols)
	if err != nil {
		s.log.Error("sr evaluation symbols marshal failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	jobID := analysis.NewEvaluationJobID()
	mode := "evaluation"
	if request.DecisionReplay {
		mode = "decision_replay"
	}
	if _, err := s.srEvaluationJobs.Create(ctx, &store.SREvaluationJob{
		JobID:         jobID,
		Status:        "pending",
		Symbols:       string(symbolsJSON),
		Timeframe:     request.Timeframe,
		FetchLimit:    request.Limit,
		Mode:          mode,
		WriteDB:       request.WriteDB,
		ReplayMaxRows: request.ReplayMaxRows,
	}); err != nil {
		s.log.Error("sr evaluation job create failed", zap.String("job_id", jobID), zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}
	if err := s.srEvaluationJobs.MarkRunning(ctx, jobID); err != nil {
		s.log.Error("sr evaluation job mark running failed", zap.String("job_id", jobID), zap.Error(err))
	}

	report, err := s.analysisClient.RunSREvaluation(ctx, request)
	if err != nil {
		s.log.Error("sr evaluation failed", zap.String("job_id", jobID), zap.Error(err))
		if markErr := s.srEvaluationJobs.MarkFailed(ctx, jobID, err.Error()); markErr != nil {
			s.log.Error("sr evaluation job mark failed failed", zap.String("job_id", jobID), zap.Error(markErr))
		}
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		s.log.Error("sr evaluation report marshal failed", zap.String("job_id", jobID), zap.Error(err))
		reportJSON = []byte("null")
	}
	if err := s.srEvaluationJobs.MarkDone(
		ctx,
		jobID,
		store.RawJSON(reportJSON),
		analysis.StringFromReport(report, "run_id"),
		analysis.StringFromReport(report, "schema_version"),
		analysis.StringFromReport(report, "pipeline_version"),
		analysis.IntFromReport(report, "rows"),
		analysis.IntFromReport(report, "sources"),
	); err != nil {
		s.log.Error("sr evaluation job mark done failed", zap.String("job_id", jobID), zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	s.log.Info("sr evaluation job completed", zap.String("job_id", jobID), zap.Int("symbols", len(symbols)))
	s.finishRun(ctx, runID, "sr_evaluation", len(symbols), 0, "")
}

func (s *Scheduler) srEvaluationSymbols(ctx context.Context) ([]string, error) {
	if len(s.srEvaluation.Symbols) > 0 {
		symbols := make([]string, 0, len(s.srEvaluation.Symbols))
		for _, symbol := range s.srEvaluation.Symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				symbols = append(symbols, symbol)
			}
		}
		return symbols, nil
	}
	return s.watchlist.Symbols(ctx)
}

func (s *Scheduler) srEvaluationRequest(symbols []string) analysis.SREvaluationRequest {
	timeframe := strings.TrimSpace(s.srEvaluation.Timeframe)
	if timeframe == "" {
		timeframe = "1d"
	}
	limit := s.srEvaluation.Limit
	if limit <= 0 {
		limit = 1500
	}
	replayMaxRows := s.srEvaluation.ReplayMaxRows
	if s.srEvaluation.DecisionReplay && replayMaxRows <= 0 {
		replayMaxRows = 200
	}
	if !s.srEvaluation.DecisionReplay {
		replayMaxRows = 0
	}
	return analysis.SREvaluationRequest{
		Symbols:        symbols,
		Timeframe:      timeframe,
		Limit:          limit,
		WriteDB:        s.srEvaluation.WriteDB,
		DecisionReplay: s.srEvaluation.DecisionReplay,
		ReplayMaxRows:  replayMaxRows,
	}
}

