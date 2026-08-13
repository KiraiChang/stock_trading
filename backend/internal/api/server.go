package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/api/handler"
	"github.com/trading/backend/internal/api/middleware"
	"github.com/trading/backend/internal/api/ws"
	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/ui"
)

type Server struct {
	router *gin.Engine
	hub    *ws.Hub
	log    *zap.Logger
}

func NewServer(
	db *sqlx.DB,
	candleRepo store.CandleRepo,
	indicatorRepo store.IndicatorRepo,
	indEngine *indicator.Engine,
	sigEngine *signal.Engine,
	signalRepo store.SignalRepo,
	watchlistRepo store.WatchlistRepo,
	stockSymbolRepo store.StockSymbolRepo,
	backtestRepo store.BacktestRepo,
	jobRunRepo store.JobRunRepo,
	analysisRepo store.AnalysisRepo,
	srZoneRepo store.SRZoneRepo,
	srScoringTrainJobRepo store.SRScoringTrainJobRepo,
	srZoneVerifier *analysis.SRZoneVerifier,
	btManager *backtest.Manager,
	analysisClient *analysis.Client,
	fetcher *market.Fetcher,
	sched *scheduler.Scheduler,
	userRepo store.UserRepo,
	institutionalTradeRepo store.InstitutionalTradeRepo,
	marginTradeRepo store.MarginTradeRepo,
	brokerTradeRepo store.BrokerTradeRepo,
	chipScoreRepo store.ChipScoreRepo,
	chipSyncJobRepo store.ChipSyncJobRepo,
	chipSyncer *chip.Syncer,
	marketBackfillJobRepo store.MarketBackfillJobRepo,
	positionRepo store.PositionRepo,
	positionConfig portfolio.Config,
	chipHistoryTradingDays int,
	jwtSecret string,
	log *zap.Logger,
) *Server {
	hub := ws.NewHub(log)
	go hub.Run()

	r := gin.New()
	r.Use(middleware.Logger(log), middleware.CORS(), gin.Recovery())

	// 給 docker / uptime 探測用，不需 token。錯誤細節（DB DSN 等）只寫 log，
	// 不回給呼叫端。
	r.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			log.Error("health check: db ping failed", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// ── 公開路由（不需 token）─────────────────────────────────
	ah := handler.NewAuthHandler(userRepo, jwtSecret, log)
	auth := v1.Group("/auth")
	{
		auth.POST("/register", ah.Register)
		auth.POST("/login", ah.Login)
	}

	// ── 保護路由（需要 Bearer token）─────────────────────────
	protected := v1.Group("")
	protected.Use(middleware.Auth(jwtSecret))
	{
		ch := handler.NewCandleHandler(candleRepo, log)
		protected.GET("/candles/:symbol", ch.GetCandles)

		ih := handler.NewIndicatorHandler(indEngine, indicatorRepo)
		protected.GET("/indicators/:symbol", ih.GetIndicators)
		protected.POST("/indicators/:symbol/compute", ih.Compute)

		sh := handler.NewSignalHandler(sigEngine, signalRepo, log)
		protected.GET("/signals", sh.GetSignals)
		protected.POST("/signals/:symbol/evaluate", sh.Evaluate)

		wh := handler.NewWatchlistHandler(watchlistRepo, log)
		protected.GET("/watchlist", wh.GetAll)
		protected.POST("/watchlist", wh.Add)
		protected.POST("/watchlist/bulk", wh.BulkAdd)
		protected.PUT("/watchlist/:symbol", wh.Update)
		protected.DELETE("/watchlist/:symbol", wh.Remove)
		protected.PATCH("/watchlist/:symbol/watch", wh.SetWatched)

		ssh := handler.NewStockSymbolHandler(stockSymbolRepo, log)
		protected.GET("/stock-symbols/search", ssh.Search)
		protected.GET("/stock-symbols/candidates", ssh.Candidates)
		protected.GET("/stock-symbols/facets", ssh.Facets)

		mh := handler.NewMarketHandler(fetcher, marketBackfillJobRepo, log)
		protected.POST("/market/backfill", mh.Backfill)
		protected.GET("/market/backfill/:job_id", mh.GetBackfillJob)

		sch := handler.NewSchedulerHandler(jobRunRepo, sched, log)
		protected.GET("/scheduler/status", sch.GetStatus)
		protected.POST("/scheduler/daily-close/run", sch.RunDailyClose)
		protected.POST("/scheduler/stock-symbol-sync/run", sch.RunStockSymbolSync)
		protected.POST("/scheduler/sr-evaluation/run", sch.RunSREvaluation)
		protected.POST("/scheduler/corporate-action-sync/run", sch.RunCorporateActionSync)

		bh := handler.NewBacktestHandler(btManager, backtestRepo, log)
		protected.POST("/backtest", bh.Submit)
		protected.GET("/backtest", bh.ListJobs)
		protected.GET("/backtest/:job_id", bh.GetJob)
		protected.GET("/backtest/:job_id/trades", bh.GetTrades)
		protected.DELETE("/backtest/:job_id", bh.Cancel)

		anh := handler.NewAnalysisHandler(analysisClient, analysis.NewVerifier(analysisRepo, candleRepo), analysisRepo, log)
		protected.POST("/analysis", anh.Create)
		protected.GET("/analysis", anh.List)
		protected.GET("/analysis/:id", anh.Get)
		protected.POST("/analysis/:id/verify", anh.Verify)
		protected.DELETE("/analysis/:id", anh.Delete)

		srAnalysisProvider := analysis.NewSRAnalysisProvider(analysisClient, srZoneRepo, positionConfig.SRReuseMaxAge)
		szh := handler.NewSRZoneHandler(
			analysisClient, srZoneRepo, watchlistRepo, srScoringTrainJobRepo, srZoneVerifier, srAnalysisProvider, log,
		)
		srRegressionResultHandler := handler.NewSRRegressionResultHandler(
			analysisClient,
			store.NewSRRegressionResultRepo(db),
			store.NewSREvaluationJobRepo(db),
			chipScoreRepo,
			store.NewSRModelGovernanceRepo(db),
			log,
		)
		protected.POST("/sr-zones", szh.Create)
		protected.GET("/sr-zones", szh.List)
		protected.POST("/sr-zones/evaluate", srRegressionResultHandler.Evaluate)
		protected.GET("/sr-zones/evaluation-jobs", srRegressionResultHandler.ListEvaluationJobs)
		protected.GET("/sr-zones/evaluation-jobs/:job_id", srRegressionResultHandler.GetEvaluationJob)
		protected.GET("/sr-zones/regression-results", srRegressionResultHandler.List)
		protected.GET("/sr-zones/:id", szh.Get)
		protected.POST("/sr-zones/:id/verify", szh.Verify)
		protected.POST("/sr-zones/train", szh.Train)
		protected.GET("/sr-zones/train-jobs", szh.ListTrainJobs)
		protected.DELETE("/sr-zones/train-jobs", szh.PruneTrainJobs)
		protected.GET("/sr-zones/train-jobs/:job_id", szh.GetTrainJob)
		protected.GET("/sr-zones/model-status", szh.ModelStatus)
		// 靜態路徑 ＋ query：同層的 /sr-zones/:id 已佔用 wildcard 位置，
		// 再放 /sr-zones/:symbol/... 會與它衝突（見 handler.EventTimeline 的說明）。
		protected.GET("/sr-zones/event-timeline", szh.EventTimeline)
		protected.DELETE("/sr-zones/:id", szh.Delete)

		uh := handler.NewUserHandler(userRepo, log)
		protected.GET("/users", uh.List)
		protected.PATCH("/users/:id/status", uh.UpdateStatus)

		portfolioRepoForScope := store.NewPortfolioRepo(db)
		pfh := handler.NewPortfolioHandler(portfolioRepoForScope, log)
		protected.GET("/portfolios", pfh.List)
		protected.POST("/portfolios", pfh.Create)

		gh := handler.NewGroupHandler(store.NewGroupRepo(db), log)
		protected.GET("/groups", gh.List)
		protected.POST("/groups", gh.Create)
		protected.POST("/groups/:id/members", gh.AddMember)

		cph := handler.NewChipHandler(
			institutionalTradeRepo, marginTradeRepo, brokerTradeRepo, chipScoreRepo,
			candleRepo, chipSyncJobRepo, chipSyncer, chipHistoryTradingDays, log,
		)
		protected.GET("/chips/:symbol/summary", cph.GetSummary)
		protected.GET("/chips/:symbol/scores", cph.GetScores)
		protected.GET("/chips/:symbol/brokers", cph.GetBrokers)
		protected.POST("/chips/sync", cph.Sync)
		protected.GET("/chips/sync/:job_id", cph.GetSyncJob)

		positionAnalyzer := portfolio.NewAnalyzer(analysisClient, positionRepo, srZoneRepo, positionConfig)
		ph := handler.NewPositionHandler(positionRepo, portfolioRepoForScope, log)
		protected.GET("/positions", ph.List)
		protected.GET("/positions/:symbol", ph.Get)
		protected.GET("/positions/:symbol/transactions", ph.ListTransactions)
		protected.POST("/positions/:symbol/transactions", ph.AddTransaction)
		protected.POST("/positions/:symbol/adjustments", ph.Adjust)

		tah := handler.NewTradeAnalysisHandler(positionRepo, portfolioRepoForScope, positionAnalyzer, log)
		protected.POST("/trade-analysis/analyze", tah.Analyze)
		protected.GET("/trade-analysis/:symbol/history", tah.ListHistory)
	}

	r.GET("/ws/market", func(c *gin.Context) {
		hub.ServeWS(c.Writer, c.Request)
	})

	// Serve embedded frontend SPA; fall back to index.html for client-side routes
	distFS := ui.FS()
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
			c.Status(http.StatusNotFound)
			return
		}
		f, err := distFS.Open(path)
		if err == nil {
			f.Close()
			c.FileFromFS(path, distFS)
			return
		}
		c.FileFromFS("/index.html", distFS)
	})

	return &Server{router: r, hub: hub, log: log}
}

func (s *Server) Hub() *ws.Hub {
	return s.hub
}

func (s *Server) Run(addr string) error {
	s.log.Info("server starting", zap.String("addr", addr))
	return s.router.Run(addr)
}
