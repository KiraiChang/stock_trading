package main

import (
	"context"
	"fmt"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/api"
	"github.com/trading/backend/internal/api/ws"
	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/indicator"
	applog "github.com/trading/backend/internal/logging"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

func main() {
	log, cleanup, err := applog.New("backend")
	if err != nil {
		fmt.Fprintf(os.Stderr, "persistent logger init failed: %v\n", err)
		fallback, ferr := zap.NewProduction()
		if ferr != nil {
			fmt.Fprintf(os.Stderr, "fallback logger init failed: %v\n", ferr)
			os.Exit(1)
		}
		log = fallback
		cleanup = func() {
			_ = log.Sync()
		}
	}
	defer cleanup()

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
	candleRepo := store.NewCandleRepo(db)
	indicatorRepo := store.NewIndicatorRepo(db)
	signalRepo := store.NewSignalRepo(db)
	watchlistRepo := store.NewWatchlistRepo(db)
	backtestRepo := store.NewBacktestRepo(db)
	userRepo := store.NewUserRepo(db)
	jobRunRepo := store.NewJobRunRepo(db)
	analysisRepo := store.NewAnalysisRepo(db)
	srZoneRepo := store.NewSRZoneRepo(db)
	srScoringTrainJobRepo := store.NewSRScoringTrainJobRepo(db)
	institutionalTradeRepo := store.NewInstitutionalTradeRepo(db)
	marginTradeRepo := store.NewMarginTradeRepo(db)
	brokerTradeRepo := store.NewBrokerTradeRepo(db)
	chipScoreRepo := store.NewChipScoreRepo(db)
	chipSyncJobRepo := store.NewChipSyncJobRepo(db)
	positionRepo := store.NewPositionRepo(db)

	// Engines
	indEngine := indicator.NewEngine(candleRepo, indicatorRepo, rdb, log)
	sigEngine := signal.NewEngine(candleRepo, signalRepo, rdb, indEngine, chipScoreRepo, log)

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
	fetcher := market.NewFetcher(finmindClient, candleRepo, log)

	// 籌碼分析：FinMind 目前只支援三大法人與融資融券，券商分點是 stub
	// （market.ErrBrokerDataUnsupported），broker_score 會 fallback 為中性。
	chipSyncer := chip.NewSyncer(finmindClient, institutionalTradeRepo, marginTradeRepo, brokerTradeRepo, chipScoreRepo, candleRepo, log)

	// Fugle（富果）即時行情，與 FinMind 並行；Enabled 為 false 時完全不掛載，
	// 行為與導入前一致。Tier 1（REST 廣度掃描）／Tier 2（WebSocket 熱點）的
	// 排程整合（round-robin 掃描、熱點名額晉升/降級）尚未接上 scheduler，
	// 待用 cmd/fugle-check 驗證延遲與推送格式後再補上（見計畫文件）。
	var fugleStreamClient *market.FugleStreamClient
	fugleStreamCtx, cancelFugleStream := context.WithCancel(context.Background())
	defer cancelFugleStream()
	if cfg.Fugle.Enabled {
		if cfg.Fugle.APIKey == "" || cfg.Fugle.APIKey == "YOUR_FUGLE_API_KEY" {
			log.Warn("fugle api_key 未設定，Fugle 即時行情請求會失敗；請設定環境變數 FUGLE_API_KEY")
		}
		fugleQuoteClient := market.NewFugleQuoteClient(cfg.Fugle)
		fugleStreamClient = market.NewFugleStreamClient(cfg.Fugle, log)
		fugleStreamClient.Start(fugleStreamCtx)
		fetcher.SetFugle(fugleQuoteClient, fugleStreamClient)
		log.Info("fugle enabled", zap.Int("quote_rate_limit", cfg.Fugle.QuoteRateLimit), zap.Int("max_subscriptions", cfg.Fugle.MaxSubscriptions))
	}

	// Yahoo 盤中資料源（非官方 API），作為 Tier-1 批次盤中源，掛上後 scheduler
	// 的盤中 job 會優先走 Yahoo 批次（免 token），未掛載時退回 FinMind 分K。
	// Enabled=false（預設）時完全不掛載，行為與導入前一致。
	if cfg.Yahoo.Enabled {
		yahooClient := market.NewYahooQuoteClient(cfg.Yahoo)
		fetcher.SetIntradaySource(yahooClient)
		log.Info("yahoo intraday enabled", zap.Int("rate_limit", cfg.Yahoo.RateLimit), zap.Int("batch_size", cfg.Yahoo.BatchSize))
	}

	// Scheduler（先建立好讓 API Server 能掛上手動觸發端點，Start() 留到最後才呼叫）
	srZoneVerifier := analysis.NewSRZoneVerifier(srZoneRepo, candleRepo)
	sched := scheduler.New(fetcher, sigEngine, watchlistRepo, jobRunRepo, srZoneRepo, srZoneVerifier, chipSyncer, cfg.Chip.Sync.Cron, cfg.FinMind.IntradayEnabled, log)

	// API Server（含 WebSocket Hub）
	positionConfig := portfolio.Config{
		MaxPositionValue:         cfg.PositionAnalysis.MaxPositionValue,
		MaxRiskAmount:            cfg.PositionAnalysis.MaxRiskAmount,
		AddOnRatio:               cfg.PositionAnalysis.AddOnRatio,
		MinRiskRewardRatio:       cfg.PositionAnalysis.MinRiskRewardRatio,
		BreakoutTargetRR:         cfg.PositionAnalysis.BreakoutTargetRR,
		TakeProfitReductionRatio: cfg.PositionAnalysis.TakeProfitReductionRatio,
		SRReuseMaxAge:            time.Duration(cfg.PositionAnalysis.SRReuseMaxAgeHours) * time.Hour,
	}
	srv := api.NewServer(db, candleRepo, indicatorRepo, indEngine, sigEngine, signalRepo, watchlistRepo, backtestRepo, jobRunRepo, analysisRepo, srZoneRepo, srScoringTrainJobRepo, srZoneVerifier, btManager, analysisClient, fetcher, sched, userRepo, institutionalTradeRepo, marginTradeRepo, brokerTradeRepo, chipScoreRepo, chipSyncJobRepo, chipSyncer, positionRepo, positionConfig, cfg.Chip.Sync.HistoryTradingDays, cfg.Auth.JWTSecret, log)

	// 注入 WebSocket broadcast
	sigEngine.BroadcastFn = func(sym string, sig *store.Signal) {
		srv.Hub().Broadcast(ws.Event{Type: "signal", Symbol: sym, Data: sig})
	}

	go sched.Start()

	srvErrCh := make(chan error, 1)
	go func() {
		srvErrCh <- srv.Run(":" + cfg.Server.Port)
	}()

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-srvErrCh:
		if err != nil {
			log.Error("server error", zap.Error(err))
		}
	case <-sigCh:
		log.Info("shutdown signal received")
	}

	// 收到關閉訊號時先停止 Fugle stream 的重連迴圈並送出正常的 WS close
	// handshake，讓 Fugle 伺服器立即釋放唯一的連線名額（免費方案僅允許
	// 1 條連線），避免下次啟動時因舊連線名額未釋放而收到
	// "Maximum number of connections reached" 錯誤。
	if fugleStreamClient != nil {
		cancelFugleStream()
		if err := fugleStreamClient.Close(); err != nil {
			log.Warn("fugle stream close failed", zap.Error(err))
		}
	}
}
