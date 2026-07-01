package main

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
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
	analysisRepo  := store.NewAnalysisRepo(db)

	// Engines
	indEngine := indicator.NewEngine(candleRepo, indicatorRepo, rdb, log)
	sigEngine := signal.NewEngine(candleRepo, signalRepo, rdb, indEngine, log)

	// Backtest manager
	btManager := backtest.NewManager(backtestRepo, cfg.Python.ServiceURL, log)

	// 個股分析：實際計算委由 Python（重用 backtest/modular 的模組化策略元件），
	// Go 只負責呼叫、持久化與驗證；PYTHON_SERVICE_URL 未設定時呼叫會回錯誤訊息
	analysisClient := analysis.NewClient(cfg.Python.ServiceURL)

	// FinMind Client + Fetcher
	if cfg.FinMind.APIKey == "" || cfg.FinMind.APIKey == "YOUR_FINMIND_API_KEY" {
		log.Warn("finmind api_key 未設定，FinMind 請求極可能失敗（http 422 或空白錯誤訊息）；請設定環境變數 FINMIND_API_KEY")
	}
	finmindClient := market.NewFinMindClient(cfg.FinMind)
	fetcher       := market.NewFetcher(finmindClient, candleRepo, log)

	// Fugle（富果）即時行情，與 FinMind 並行；Enabled 為 false 時完全不掛載，
	// 行為與導入前一致。Tier 1（REST 廣度掃描）／Tier 2（WebSocket 熱點）的
	// 排程整合（round-robin 掃描、熱點名額晉升/降級）尚未接上 scheduler，
	// 待用 cmd/fugle-check 驗證延遲與推送格式後再補上（見計畫文件）。
	if cfg.Fugle.Enabled {
		if cfg.Fugle.APIKey == "" || cfg.Fugle.APIKey == "YOUR_FUGLE_API_KEY" {
			log.Warn("fugle api_key 未設定，Fugle 即時行情請求會失敗；請設定環境變數 FUGLE_API_KEY")
		}
		fugleQuoteClient := market.NewFugleQuoteClient(cfg.Fugle)
		fugleStreamClient := market.NewFugleStreamClient(cfg.Fugle, log)
		fugleStreamClient.Start(context.Background())
		fetcher.SetFugle(fugleQuoteClient, fugleStreamClient)
		log.Info("fugle enabled", zap.Int("quote_rate_limit", cfg.Fugle.QuoteRateLimit), zap.Int("max_subscriptions", cfg.Fugle.MaxSubscriptions))
	}

	// API Server（含 WebSocket Hub）
	srv := api.NewServer(db, candleRepo, indicatorRepo, indEngine, sigEngine, signalRepo, watchlistRepo, backtestRepo, jobRunRepo, analysisRepo, btManager, analysisClient, fetcher, userRepo, cfg.Auth.JWTSecret, log)

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
