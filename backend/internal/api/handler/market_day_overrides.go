package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
	"github.com/trading/backend/pkg/timeutil"
)

// MarketDayOverrideHandler 是臨時休市（颱風假）與推翻日曆的手動設定（I-139）：
//   - GET    /api/v1/market-day-overrides?from=YYYY-MM-DD
//   - PUT    /api/v1/market-day-overrides/:date   body {"action": "closed"|"open", "reason": "…"}
//   - DELETE /api/v1/market-day-overrides/:date   body {"reason": "…"}
//
// 每次儲存與取消各寫一列（只新增、⛔ 不改寫），留下操作者、時間與原因；下一次判斷就生效。
// 只接受今天或未來的平日（過去的 job_runs 已經發生；週末一律休市，見 tradingday）。
// ⚠️ 權限沿用現有手動排程端點（任何登入的使用者）；目前只有一人使用，多人使用之前要調整。
// 現況規格見 docs/api-reference.md。
type MarketDayOverrideHandler struct {
	repo  store.MarketDayOverrideRepo
	sched *scheduler.Scheduler
	now   func() time.Time
	log   *zap.Logger
}

func NewMarketDayOverrideHandler(repo store.MarketDayOverrideRepo, sched *scheduler.Scheduler, log *zap.Logger) *MarketDayOverrideHandler {
	return &MarketDayOverrideHandler{repo: repo, sched: sched, now: time.Now, log: log}
}

// SetClockForTest 是測試接縫；production ⛔ 不呼叫。
func (h *MarketDayOverrideHandler) SetClockForTest(now func() time.Time) { h.now = now }

func (h *MarketDayOverrideHandler) judge() *tradingday.Judge {
	if h.sched != nil && h.sched.TradingDay() != nil {
		return h.sched.TradingDay()
	}
	return noJudge
}

type marketDayToday struct {
	Date string `json:"date"`
	tradingday.Result
}

type marketDayOverridesResponse struct {
	// Today 是今天的判斷與依據（畫面顯示「依日曆／手動休市／手動照常開市／日曆取不到」）。
	Today marketDayToday `json:"today"`
	// Current 是每一天目前生效中的手動設定（最新一列是 closed 或 open 的那幾天）。
	Current []store.MarketDayOverride `json:"current"`
	// History 是完整的操作紀錄（依日期、id 排序），含取消。
	History []store.MarketDayOverride `json:"history"`
}

func (h *MarketDayOverrideHandler) today() time.Time {
	l := h.now().In(timeutil.TaipeiTZ)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
}

// List 列出 from（預設今天）以後的手動設定與操作紀錄，以及今天的判斷。
func (h *MarketDayOverrideHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	today := h.today()
	from := today.Format("2006-01-02")
	if q := c.Query("from"); q != "" {
		d, err := time.ParseInLocation("2006-01-02", q, timeutil.TaipeiTZ)
		if err != nil || d.Format("2006-01-02") != q {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_from"})
			return
		}
		from = q
	}
	rows, err := h.repo.ListSince(ctx, from)
	if err != nil {
		serverError(c, h.log, err, "market day overrides: list")
		return
	}
	latest := map[string]store.MarketDayOverride{}
	order := []string{}
	for _, r := range rows {
		if _, ok := latest[r.TradeDate]; !ok {
			order = append(order, r.TradeDate)
		}
		latest[r.TradeDate] = r // rows 依 trade_date、id 排序：最後一筆就是最新
	}
	current := []store.MarketDayOverride{}
	for _, d := range order {
		if r := latest[d]; r.Effective() {
			current = append(current, r)
		}
	}
	c.JSON(http.StatusOK, marketDayOverridesResponse{
		Today:   marketDayToday{Date: today.Format("2006-01-02"), Result: h.judge().On(ctx, h.now())},
		Current: current,
		History: rows,
	})
}

type marketDayPutRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type marketDayDeleteRequest struct {
	Reason string `json:"reason"`
}

type marketDayWriteResponse struct {
	Override   store.MarketDayOverride `json:"override"`
	TradingDay tradingday.Result       `json:"trading_day"`
}

// validDate 檢查路徑上的日期：格式、今天或未來、平日。不合格時已回 422。
func (h *MarketDayOverrideHandler) validDate(c *gin.Context) (time.Time, bool) {
	raw := c.Param("date")
	d, err := time.ParseInLocation("2006-01-02", raw, timeutil.TaipeiTZ)
	if err != nil || d.Format("2006-01-02") != raw {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_date"})
		return time.Time{}, false
	}
	if d.Before(h.today()) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "past_date"})
		return time.Time{}, false
	}
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "weekend"})
		return time.Time{}, false
	}
	return d, true
}

func validReason(c *gin.Context, raw string) (string, bool) {
	r := strings.TrimSpace(raw)
	if r == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "reason_required"})
		return "", false
	}
	if utf8.RuneCountInString(r) > store.MarketDayReasonMaxRunes {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "reason_too_long"})
		return "", false
	}
	return r, true
}

// Put 設定手動休市或手動照常開市。
func (h *MarketDayOverrideHandler) Put(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	d, ok := h.validDate(c)
	if !ok {
		return
	}
	var req marketDayPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if req.Action != store.MarketDayClosed && req.Action != store.MarketDayOpen {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_action"})
		return
	}
	reason, ok := validReason(c, req.Reason)
	if !ok {
		return
	}
	h.write(c, store.MarketDayOverride{TradeDate: d.Format("2006-01-02"), Action: req.Action, Reason: reason, UserID: userID}, d)
}

// Delete 取消手動設定（寫一列 cleared），回到日曆。那一天沒有生效中的手動設定時回 409。
func (h *MarketDayOverrideHandler) Delete(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	d, ok := h.validDate(c)
	if !ok {
		return
	}
	var req marketDayDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	reason, ok := validReason(c, req.Reason)
	if !ok {
		return
	}
	date := d.Format("2006-01-02")
	cur, err := h.repo.Latest(c.Request.Context(), date)
	if err != nil {
		serverError(c, h.log, err, "market day overrides: latest")
		return
	}
	if !cur.Effective() {
		c.JSON(http.StatusConflict, gin.H{"error": "no_active_override"})
		return
	}
	h.write(c, store.MarketDayOverride{TradeDate: date, Action: store.MarketDayCleared, Reason: reason, UserID: userID}, d)
}

// write 寫一列並回傳**這次寫入的那一列**（repo 的 Insert 在同一步寫入與讀回；⛔ 不另查「那一天最新的一列」，
// 別的頁面同時寫入時才⛔ 不會回傳或記下對方的紀錄）。Insert 失敗＝沒有寫入，回 500 才是實話。
func (h *MarketDayOverrideHandler) write(c *gin.Context, o store.MarketDayOverride, d time.Time) {
	ctx := c.Request.Context()
	saved, err := h.repo.Insert(ctx, o)
	if err != nil {
		serverError(c, h.log, err, "market day overrides: insert")
		return
	}
	h.log.Info("market day override saved", zap.String("date", saved.TradeDate), zap.String("action", saved.Action),
		zap.Uint64("user_id", saved.UserID), zap.Uint64("id", saved.ID))
	c.JSON(http.StatusOK, marketDayWriteResponse{Override: saved, TradingDay: h.judge().On(ctx, d)})
}
