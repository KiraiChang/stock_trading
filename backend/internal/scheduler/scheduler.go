package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// srZoneVerifyLimit 每次收盤驗證最多處理幾筆最近的 SR zone 分析，避免隨著
// 歷史分析越積越多，這個 job 的執行時間跟著無上限成長（見 RunDailyClose）。
const srZoneVerifyLimit = 50

type Scheduler struct {
	fetcher         *market.Fetcher
	signalEng       *signal.Engine
	watchlist       store.WatchlistRepo
	jobRuns         store.JobRunRepo
	srZoneRepo      store.SRZoneRepo
	srZoneVerifier  *analysis.SRZoneVerifier
	intradayEnabled bool
	log             *zap.Logger
	cron            *cron.Cron
}

func New(
	fetcher *market.Fetcher,
	signalEng *signal.Engine,
	watchlist store.WatchlistRepo,
	jobRuns store.JobRunRepo,
	srZoneRepo store.SRZoneRepo,
	srZoneVerifier *analysis.SRZoneVerifier,
	intradayEnabled bool,
	log *zap.Logger,
) *Scheduler {
	return &Scheduler{
		fetcher:         fetcher,
		signalEng:       signalEng,
		watchlist:       watchlist,
		jobRuns:         jobRuns,
		srZoneRepo:      srZoneRepo,
		srZoneVerifier:  srZoneVerifier,
		intradayEnabled: intradayEnabled,
		log:             log,
		cron:            cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
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
	failed := s.fetcher.BackfillHistory(ctx, symbols, 5)

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
	if !s.intradayEnabled {
		// finmind.intraday_enabled=false（預設）：帳號等級不足以使用
		// TaiwanStockKBar dataset，不建立 job_run 紀錄，避免每 5 分鐘
		// 洗一筆「skipped」進資料庫；升級帳號後改設定即可恢復
		s.log.Debug("intraday job skipped: finmind.intraday_enabled=false")
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
