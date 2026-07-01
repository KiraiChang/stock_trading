package main

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/api"
	"github.com/trading/backend/internal/api/ws"
	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", zap.Error(err))
		os.Exit(1)
	}

	// 資料庫（sqlite 開發 / mysql 生產）
	db, err := store.NewDB(cfg.Database)
	if err != nil {
		log.Error("database connect failed",
			zap.String("driver", cfg.Database.Driver),
			zap.Error(err),
		)
		os.Exit(1)
	}
	log.Info("database connected", zap.String("driver", cfg.Database.Driver))

	// 自動套用 migrations
	if err := database.RunMigrations(context.Background(), db, cfg.Database.Driver, log); err != nil {
		log.Error("migrations failed", zap.Error(err))
		os.Exit(1)
	}

	// Redis（選填；addr 空白時為 no-op）
	rdb := store.NewRedis(cfg.Redis)
	if rdb.Enabled() {
		if err := rdb.Ping(context.Background()); err != nil {
			log.Warn("redis connect failed, cache disabled", zap.Error(err))
			rdb = store.NewRedis(store.DisabledRedisConfig())
		} else {
			log.Info("redis connected", zap.String("addr", cfg.Redis.Addr))
		}
	} else {
		log.Info("redis disabled (no addr configured)")
	}

	// Repos
	candleRepo    := store.NewCandleRepo(db)
	indicatorRepo := store.NewIndicatorRepo(db)
	signalRepo    := store.NewSignalRepo(db)
	watchlistRepo := store.NewWatchlistRepo(db)
	backtestRepo  := store.NewBacktestRepo(db)
	userRepo      := store.NewUserRepo(db)
	jobRunRepo    := store.NewJobRunRepo(db)

	// Engines
	indEngine := indicator.NewEngine(candleRepo, indicatorRepo, rdb, log)
	sigEngine := signal.NewEngine(candleRepo, signalRepo, rdb, indEngine, log)

	// Backtest manager
	btManager := backtest.NewManager(backtestRepo, cfg.Python.ServiceURL, log)

	// FinMind Client + Fetcher
	if cfg.FinMind.APIKey == "" || cfg.FinMind.APIKey == "YOUR_FINMIND_API_KEY" {
		log.Warn("finmind api_key 未設定，FinMind 請求極可能失敗（http 422 或空白錯誤訊息）；請設定環境變數 FINMIND_API_KEY")
	}
	finmindClient := market.NewFinMindClient(cfg.FinMind)
	fetcher       := market.NewFetcher(finmindClient, candleRepo, log)

	// API Server（含 WebSocket Hub）
	srv := api.NewServer(db, candleRepo, indicatorRepo, signalRepo, watchlistRepo, backtestRepo, jobRunRepo, btManager, fetcher, userRepo, cfg.Auth.JWTSecret, log)

	// 注入 WebSocket broadcast
	sigEngine.BroadcastFn = func(sym string, sig *store.Signal) {
		srv.Hub().Broadcast(ws.Event{Type: "signal", Symbol: sym, Data: sig})
	}

	// Scheduler
	sched := scheduler.New(fetcher, sigEngine, watchlistRepo, jobRunRepo, log)
	go sched.Start()

	if err := srv.Run(":" + cfg.Server.Port); err != nil {
		log.Error("server error", zap.Error(err))
	}
}
