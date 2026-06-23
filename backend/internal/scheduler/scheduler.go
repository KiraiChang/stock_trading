package scheduler

import (
	"context"

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
	log       *zap.Logger
	cron      *cron.Cron
}

func New(
	fetcher *market.Fetcher,
	signalEng *signal.Engine,
	watchlist store.WatchlistRepo,
	log *zap.Logger,
) *Scheduler {
	return &Scheduler{
		fetcher:   fetcher,
		signalEng: signalEng,
		watchlist: watchlist,
		log:       log,
		cron:      cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
	}
}

func (s *Scheduler) Start() {
	// 盤前初始化
	s.cron.AddFunc("50 8 * * 1-5", func() {
		s.log.Info("pre-market job started")
	})

	// 盤中：每 5 分鐘拉取分K + 計算指標 + Signal 掃描
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

func (s *Scheduler) runIntradayJob() {
	ctx := context.Background()
	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		return
	}

	today := timeutil.TodayTaipei()
	for _, sym := range symbols {
		if err := s.fetcher.FetchAndStoreMinute(ctx, sym, today); err != nil {
			s.log.Warn("intraday fetch failed", zap.String("symbol", sym), zap.Error(err))
		}
		s.signalEng.Evaluate(ctx, sym, "1m")
	}
}

func (s *Scheduler) runDailyClose() {
	ctx := context.Background()
	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		return
	}

	today := timeutil.TodayTaipei()
	for _, sym := range symbols {
		if err := s.fetcher.FetchAndStoreDaily(ctx, sym, today); err != nil {
			s.log.Warn("daily fetch failed", zap.String("symbol", sym), zap.Error(err))
		}
		s.signalEng.Evaluate(ctx, sym, "1d")
	}
	s.log.Info("daily close job completed", zap.Int("symbols", len(symbols)))
}
