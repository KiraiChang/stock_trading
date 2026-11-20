package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// IntradayHandler 是盤中逐檔行情與評估狀態（T-088＋I-103）：
//   - GET  /api/v1/intraday/status
//   - POST /api/v1/scheduler/intraday-refill/run
//
// 現況規格見 docs/api-reference.md 與 docs/yahoo-intraday-integration.md。
type IntradayHandler struct {
	evidence  store.IntradayEvidenceRepo
	watchlist store.WatchlistRepo
	candles   store.CandleRepo
	sched     *scheduler.Scheduler
	now       func() time.Time
	log       *zap.Logger
}

func NewIntradayHandler(
	evidence store.IntradayEvidenceRepo, watchlist store.WatchlistRepo, candles store.CandleRepo,
	sched *scheduler.Scheduler, log *zap.Logger,
) *IntradayHandler {
	return &IntradayHandler{evidence: evidence, watchlist: watchlist, candles: candles, sched: sched, now: time.Now, log: log}
}

// SetClockForTest 是測試接縫；production ⛔ 不呼叫。
func (h *IntradayHandler) SetClockForTest(now func() time.Time) { h.now = now }

type intradayRoundView struct {
	store.IntradayRound
	// Running：這一輪現在還在執行（行程內的狀態，⛔ 不從 DB 推）。
	Running bool `json:"running"`
}

type intradaySymbolStatus struct {
	Symbol string `json:"symbol"`
	// Latest 是這一檔最近一輪排程的逐檔列；沒有紀錄時是 null。
	Latest *store.IntradaySymbolRun `json:"latest"`
	// Refill 是最近一次手動補抓的列，**只在比 Latest 新時**才帶（畫面顯示「已補抓，本次未評估」）。
	Refill *store.IntradaySymbolRun `json:"refill"`
	// LastSuccessEvalAt／LastSuccessRunID 是上一次成功評估。
	LastSuccessEvalAt *time.Time `json:"last_success_eval_at"`
	LastSuccessRunID  *uint64    `json:"last_success_run_id"`
	// LastBarTS 是 DB 裡這一檔最後一根 1 分 K 的時間（系統自己的判斷依據，⛔ 不看第三方有沒有回應）。
	LastBarTS *time.Time `json:"last_bar_ts"`
	// LagMinutes 依當下時間算；最後一根不是今天（台北）時是 null。
	LagMinutes *int `json:"lag_minutes"`
	// Stale：盤中、而且落後超過門檻（scheduler.IntradayStaleAfter）。
	Stale bool `json:"stale"`
	// RoundState：最近一輪排程比這一檔最近的逐檔列新時，那一輪的狀態——畫面以輪次紀錄為準，⛔ 不把上一輪的評估結果當成這一輪的。
	//   "skipped"：等鎖逾時、沒有執行；"failed"：在逐檔結果之前就失敗（例如讀不到自選清單）；
	//   "running"：那一輪還在跑、這一檔的列還沒寫；"incomplete"：那一輪已經結束，卻沒有這一檔的列（存證缺失或中斷）。
	// 空字串＝最近的逐檔列就是最近一輪的（或比它新的補抓）。
	RoundState string `json:"round_state,omitempty"`
	// EvalPending：最近的逐檔列還沒有評估段時，"evaluating"（那一輪還在跑）或 "interrupted"（已經結束或中斷）。
	EvalPending string `json:"eval_pending,omitempty"`
}

type intradayStatusResponse struct {
	Now               time.Time              `json:"now"`
	MarketOpen        bool                   `json:"market_open"`
	StaleAfterMinutes int                    `json:"stale_after_minutes"`
	LatestRound       *intradayRoundView     `json:"latest_round"`
	Symbols           []intradaySymbolStatus `json:"symbols"`
}

// GetStatus 回傳自選清單每一檔的盤中行情與評估狀態。
func (h *IntradayHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	now := h.now()
	symbols, err := h.watchlist.Symbols(ctx)
	if err != nil {
		serverError(c, h.log, err, "intraday status: watchlist")
		return
	}
	resp := intradayStatusResponse{
		Now: now, MarketOpen: timeutil.IsMarketOpen(now),
		StaleAfterMinutes: int(scheduler.IntradayStaleAfter / time.Minute),
		Symbols:           make([]intradaySymbolStatus, 0, len(symbols)),
	}

	var running uint64
	if h.sched != nil {
		running = h.sched.IntradayRunningRunID()
	}
	round, err := h.evidence.LatestRound(ctx, store.IntradaySourceScheduled)
	if err != nil {
		serverError(c, h.log, err, "intraday status: latest round")
		return
	}
	if round != nil {
		resp.LatestRound = &intradayRoundView{IntradayRound: *round, Running: running != 0 && running == round.RunID}
	}
	latest, err := h.evidence.LatestSymbolRuns(ctx, store.IntradaySourceScheduled, symbols)
	if err != nil {
		serverError(c, h.log, err, "intraday status: latest rows")
		return
	}
	refills, err := h.evidence.LatestSymbolRuns(ctx, store.IntradaySourceRefill, symbols)
	if err != nil {
		serverError(c, h.log, err, "intraday status: refill rows")
		return
	}
	successes, err := h.evidence.LastSuccessfulEvals(ctx, symbols)
	if err != nil {
		serverError(c, h.log, err, "intraday status: last successful evaluations")
		return
	}

	nowLocal := now.In(timeutil.TaipeiTZ)
	for _, sym := range symbols {
		st := intradaySymbolStatus{Symbol: sym}
		if row, ok := latest[sym]; ok {
			r := row
			st.Latest = &r
			if !row.EvalStatus.Valid {
				if running != 0 && running == row.RunID {
					st.EvalPending = "evaluating"
				} else {
					st.EvalPending = "interrupted"
				}
			}
		}
		if row, ok := refills[sym]; ok && (st.Latest == nil || row.RunID > st.Latest.RunID) {
			r := row
			st.Refill = &r
		}
		if row, ok := successes[sym]; ok {
			id := row.RunID
			st.LastSuccessRunID = &id
			if row.EvalAt.Valid {
				t := row.EvalAt.Time
				st.LastSuccessEvalAt = &t
			}
		}
		if round != nil && (st.Latest == nil || round.RunID > st.Latest.RunID) {
			switch {
			case round.Outcome == store.IntradayOutcomeSkippedLockTimeout:
				st.RoundState = "skipped"
			case round.Outcome == store.IntradayOutcomeFailed:
				st.RoundState = "failed"
			case resp.LatestRound.Running:
				st.RoundState = "running"
			case st.Latest != nil:
				// 新加入自選清單、還沒跑過的標的（Latest 是 nil）⛔ 不算不完整。
				st.RoundState = "incomplete"
			}
		}
		// 補抓與輪次紀錄都比逐檔列新時，以 run_id 較新的為準。
		if st.Refill != nil && st.RoundState != "" {
			if round.RunID > st.Refill.RunID {
				st.Refill = nil
			} else {
				st.RoundState = ""
			}
		}
		if h.candles != nil {
			last, err := h.candles.GetLatest(ctx, sym, "1m")
			switch {
			case errors.Is(err, sql.ErrNoRows):
				// 這一檔沒有任何 1 分 K。
			case err != nil:
				h.log.Warn("intraday status: latest candle", zap.String("symbol", sym), zap.Error(err))
			case last != nil:
				ts := last.Timestamp
				st.LastBarTS = &ts
				l := ts.In(timeutil.TaipeiTZ)
				if l.Year() == nowLocal.Year() && l.YearDay() == nowLocal.YearDay() {
					lag := int(now.Sub(ts) / time.Minute)
					st.LagMinutes = &lag
					st.Stale = resp.MarketOpen && now.Sub(ts) > scheduler.IntradayStaleAfter
				}
			}
		}
		resp.Symbols = append(resp.Symbols, st)
	}
	c.JSON(http.StatusOK, resp)
}

type intradayRefillRequest struct {
	Symbols []string `json:"symbols"`
}

// RunRefill 手動補抓當日分 K（只寫 K 棒、⛔ 不評估訊號）。**同步執行**：等鎖最多 30 秒、整次最多 45 秒。
//
// ⚠️ 權限沿用現有手動排程端點（任何登入的使用者都能觸發）；目前只有一人使用，多人使用之前要調整。
func (h *IntradayHandler) RunRefill(c *gin.Context) {
	if h.sched == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "scheduler_unavailable"})
		return
	}
	var req intradayRefillRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
			return
		}
	}
	report, err := h.sched.RunIntradayRefill(c.Request.Context(), req.Symbols)
	switch {
	case errors.Is(err, scheduler.ErrIntradayRefillUnknownSymbol):
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol_not_in_watchlist"})
		return
	case errors.Is(err, scheduler.ErrIntradayRefillBusy):
		c.JSON(http.StatusConflict, gin.H{"error": "intraday_busy"})
		return
	case errors.Is(err, scheduler.ErrIntradayRefillUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "intraday_source_unavailable"})
		return
	case err != nil:
		serverError(c, h.log, err, "intraday refill")
		return
	}
	c.JSON(http.StatusOK, report)
}
