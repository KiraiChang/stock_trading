package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/api/handler"
	"github.com/trading/backend/internal/api/middleware"
	"github.com/trading/backend/internal/api/ws"
	"github.com/trading/backend/internal/backtest"
	"github.com/trading/backend/internal/store"
)

type Server struct {
	router *gin.Engine
	hub    *ws.Hub
	log    *zap.Logger
}

func NewServer(
	candleRepo    store.CandleRepo,
	indicatorRepo store.IndicatorRepo,
	signalRepo    store.SignalRepo,
	watchlistRepo store.WatchlistRepo,
	backtestRepo  store.BacktestRepo,
	btManager     *backtest.Manager,
	log           *zap.Logger,
) *Server {
	hub := ws.NewHub(log)
	go hub.Run()

	r := gin.New()
	r.Use(middleware.Logger(log), middleware.CORS(), gin.Recovery())

	v1 := r.Group("/api/v1")
	{
		ch := handler.NewCandleHandler(candleRepo)
		v1.GET("/candles/:symbol", ch.GetCandles)

		ih := handler.NewIndicatorHandler(indicatorRepo)
		v1.GET("/indicators/:symbol", ih.GetIndicators)

		sh := handler.NewSignalHandler(signalRepo)
		v1.GET("/signals", sh.GetSignals)

		wh := handler.NewWatchlistHandler(watchlistRepo)
		v1.GET("/watchlist", wh.GetAll)
		v1.POST("/watchlist", wh.Add)
		v1.DELETE("/watchlist/:symbol", wh.Remove)

		bh := handler.NewBacktestHandler(btManager, backtestRepo)
		v1.POST("/backtest", bh.Submit)
		v1.GET("/backtest", bh.ListJobs)
		v1.GET("/backtest/:job_id", bh.GetJob)
		v1.GET("/backtest/:job_id/trades", bh.GetTrades)
		v1.DELETE("/backtest/:job_id", bh.Cancel)
	}

	r.GET("/ws/market", func(c *gin.Context) {
		hub.ServeWS(c.Writer, c.Request)
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
