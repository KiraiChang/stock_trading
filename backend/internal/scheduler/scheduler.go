package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type Scheduler struct {
	fetcher   *market.Fetcher
	signalEng *signal.Engine
	watchlist store.WatchlistRepo
	jobRuns   store.JobRunRepo
	log       *zap.Logger
	cron      *cron.Cron
}

func New(
	fetcher *market.Fetcher,
	signalEng *signal.Engine,
	watchlist store.WatchlistRepo,
	jobRuns store.JobRunRepo,
	log *zap.Logger,
) *Scheduler {
	return &Scheduler{
		fetcher:   fetcher,
		signalEng: signalEng,
		watchlist: watchlist,
		jobRuns:   jobRuns,
		log:       log,
		cron:      cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
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

	// 收盤後：拉日K + 完整掃描
	s.cron.AddFunc("0 14 * * 1-5", func() {
		s.runDailyClose()
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

func (s *Scheduler) runDailyClose() {
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
}
