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
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
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
	db            *sqlx.DB,
	candleRepo    store.CandleRepo,
	indicatorRepo store.IndicatorRepo,
	indEngine     *indicator.Engine,
	sigEngine     *signal.Engine,
	signalRepo    store.SignalRepo,
	watchlistRepo store.WatchlistRepo,
	backtestRepo  store.BacktestRepo,
	jobRunRepo    store.JobRunRepo,
	analysisRepo  store.AnalysisRepo,
	srZoneRepo    store.SRZoneRepo,
	btManager     *backtest.Manager,
	analysisClient *analysis.Client,
	fetcher       *market.Fetcher,
	userRepo      store.UserRepo,
	jwtSecret     string,
	log           *zap.Logger,
) *Server {
	hub := ws.NewHub(log)
	go hub.Run()

	r := gin.New()
	r.Use(middleware.Logger(log), middleware.CORS(), gin.Recovery())

	// 給 docker / uptime 探測用，不需 token
	r.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// ── 公開路由（不需 token）─────────────────────────────────
	ah := handler.NewAuthHandler(userRepo, jwtSecret)
	auth := v1.Group("/auth")
	{
		auth.POST("/register", ah.Register)
		auth.POST("/login", ah.Login)
	}

	// ── 保護路由（需要 Bearer token）─────────────────────────
	protected := v1.Group("")
	protected.Use(middleware.Auth(jwtSecret))
	{
		ch := handler.NewCandleHandler(candleRepo)
		protected.GET("/candles/:symbol", ch.GetCandles)

		ih := handler.NewIndicatorHandler(indEngine, indicatorRepo)
		protected.GET("/indicators/:symbol", ih.GetIndicators)
		protected.POST("/indicators/:symbol/compute", ih.Compute)

		sh := handler.NewSignalHandler(sigEngine, signalRepo)
		protected.GET("/signals", sh.GetSignals)
		protected.POST("/signals/:symbol/evaluate", sh.Evaluate)

		wh := handler.NewWatchlistHandler(watchlistRepo)
		protected.GET("/watchlist", wh.GetAll)
		protected.POST("/watchlist", wh.Add)
		protected.POST("/watchlist/bulk", wh.BulkAdd)
		protected.PUT("/watchlist/:symbol", wh.Update)
		protected.DELETE("/watchlist/:symbol", wh.Remove)
		protected.PATCH("/watchlist/:symbol/watch", wh.SetWatched)

		mh := handler.NewMarketHandler(fetcher, watchlistRepo, log)
		protected.POST("/market/backfill", mh.Backfill)

		sch := handler.NewSchedulerHandler(jobRunRepo)
		protected.GET("/scheduler/status", sch.GetStatus)

		bh := handler.NewBacktestHandler(btManager, backtestRepo)
		protected.POST("/backtest", bh.Submit)
		protected.GET("/backtest", bh.ListJobs)
		protected.GET("/backtest/:job_id", bh.GetJob)
		protected.GET("/backtest/:job_id/trades", bh.GetTrades)
		protected.DELETE("/backtest/:job_id", bh.Cancel)

		anh := handler.NewAnalysisHandler(analysisClient, analysis.NewVerifier(analysisRepo, candleRepo), analysisRepo)
		protected.POST("/analysis", anh.Create)
		protected.GET("/analysis", anh.List)
		protected.GET("/analysis/:id", anh.Get)
		protected.POST("/analysis/:id/verify", anh.Verify)
		protected.DELETE("/analysis/:id", anh.Delete)

		szh := handler.NewSRZoneHandler(analysisClient, srZoneRepo)
		protected.POST("/sr-zones", szh.Create)
		protected.GET("/sr-zones", szh.List)
		protected.GET("/sr-zones/:id", szh.Get)
		protected.DELETE("/sr-zones/:id", szh.Delete)

		uh := handler.NewUserHandler(userRepo)
		protected.GET("/users", uh.List)
		protected.PATCH("/users/:id/status", uh.UpdateStatus)
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
