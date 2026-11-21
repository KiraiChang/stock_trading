package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
	"github.com/trading/backend/pkg/timeutil"
)

// I-139：狀態 API 的 trading_day／market_open／session_data，以及手動設定的三支端點。

type calStub struct{ closed map[string]bool }

func (c calStub) TradingCalendar(_ context.Context, year int) (*market.TradingCalendar, error) {
	return market.NewTradingCalendarForTest(year, c.closed), nil
}

func newMarketDayDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/marketday.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return db
}

// schedWithJudge 建一個只帶交易日判斷的 Scheduler（handler 經由 sched.TradingDay() 取得共用實例）。
func schedWithJudge(j *tradingday.Judge) *scheduler.Scheduler {
	s := scheduler.New(nil, nil, nil, nil, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop())
	s.SetTradingDay(j)
	return s
}

type statusView struct {
	TradingDay struct {
		IsTrading bool   `json:"is_trading"`
		Source    string `json:"source"`
	} `json:"trading_day"`
	MarketOpen  bool   `json:"market_open"`
	SessionData string `json:"session_data"`
}

func getStatus(t *testing.T, h *IntradayHandler) statusView {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/intraday/status", h.GetStatus)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/intraday/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d：%s", w.Code, w.Body.String())
	}
	var v statusView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestIntradayStatusTradingDay(t *testing.T) {
	db := newMarketDayDB(t)
	overrides := store.NewMarketDayOverrideRepo(db)
	judge := tradingday.New(calStub{closed: map[string]bool{"2026-12-25": true}}, overrides, zap.NewNop())
	h := NewIntradayHandler(store.NewIntradayEvidenceRepo(db), intradayWatchlistStub{symbols: []string{"A"}},
		store.NewCandleRepo(db), schedWithJudge(judge), zap.NewNop())

	// 交易日盤中：calendar、market_open
	h.SetClockForTest(func() time.Time { return tpeAt(10, 0) })
	if v := getStatus(t, h); !v.TradingDay.IsTrading || v.TradingDay.Source != tradingday.SourceCalendar || !v.MarketOpen {
		t.Fatalf("交易日盤中：%+v", v)
	}
	// 手動休市：下一次判斷就生效
	if _, err := overrides.Insert(context.Background(), store.MarketDayOverride{TradeDate: "2026-11-02", Action: store.MarketDayClosed, Reason: "颱風", UserID: 1}); err != nil {
		t.Fatal(err)
	}
	if v := getStatus(t, h); v.TradingDay.IsTrading || v.TradingDay.Source != tradingday.SourceManual || v.MarketOpen || v.SessionData != sessionDataClosed {
		t.Fatalf("手動休市：is_trading=false、manual、market_open=false、closed，得到 %+v", v)
	}
	// 日曆的休市日
	h.SetClockForTest(func() time.Time { return time.Date(2026, 12, 25, 10, 0, 0, 0, timeutil.TaipeiTZ) })
	if v := getStatus(t, h); v.TradingDay.IsTrading || v.TradingDay.Source != tradingday.SourceCalendar || v.MarketOpen {
		t.Fatalf("日曆休市日：%+v", v)
	}
	// 週末
	h.SetClockForTest(func() time.Time { return time.Date(2026, 11, 28, 10, 0, 0, 0, timeutil.TaipeiTZ) })
	if v := getStatus(t, h); v.TradingDay.Source != tradingday.SourceWeekend || v.MarketOpen {
		t.Fatalf("週末：%+v", v)
	}
	// 沒有注入判斷（sched=nil）：平日照舊當交易日（fallback）
	h2 := NewIntradayHandler(store.NewIntradayEvidenceRepo(db), intradayWatchlistStub{symbols: []string{"A"}}, store.NewCandleRepo(db), nil, zap.NewNop())
	h2.SetClockForTest(func() time.Time { return tpeAt(10, 0) })
	if v := getStatus(t, h2); !v.TradingDay.IsTrading || v.TradingDay.Source != tradingday.SourceFallback || !v.MarketOpen {
		t.Fatalf("沒有注入判斷時照舊：%+v", v)
	}
}

func TestDeriveSessionData(t *testing.T) {
	const at1000, at0920 = 10 * 60, 9*60 + 20
	cases := []struct {
		name                     string
		trading                  bool
		symbols                  int
		candleErr, anyToday      bool
		minute                   int
		fetchFailing, storeFails bool
		want                     string
	}{
		{"休市", false, 3, true, true, at1000, true, true, sessionDataClosed},
		{"自選清單是空的", true, 0, false, false, at1000, false, false, sessionDataEmptyWatchlist},
		{"查 DB 失敗", true, 3, true, false, at1000, false, false, sessionDataUnavailable},
		{"已有今天的 1 分 K", true, 3, false, true, at1000, true, false, sessionDataOK},
		{"09:30 之前", true, 3, false, false, at0920, true, false, sessionDataPending},
		{"已知抓取失敗", true, 3, false, false, at1000, true, true, sessionDataFetchFailing},
		{"已有回應但寫入失敗", true, 3, false, false, at1000, false, true, sessionDataStoreFailing},
		{"尚未收到", true, 3, false, false, at1000, false, false, sessionDataNoneYet},
		{"09:30 整點開始提示", true, 3, false, false, 9*60 + 30, false, false, sessionDataNoneYet},
	}
	for _, c := range cases {
		if got := deriveSessionData(c.trading, c.symbols, c.candleErr, c.anyToday, c.minute, c.fetchFailing, c.storeFails); got != c.want {
			t.Errorf("%s：得到 %s，期望 %s", c.name, got, c.want)
		}
	}
}

type failingLatestCandles struct{ store.CandleRepo }

func (failingLatestCandles) GetLatest(context.Context, string, string) (*store.Candle, error) {
	return nil, errors.New("db down")
}

// 實作驗收補的案例（v1.1 review）：已有回應但 K 棒全部寫入失敗 → store_failing（沿用 T-088 的寫入失敗提示）。
func TestIntradayStatusSessionData(t *testing.T) {
	ctx := context.Background()
	newH := func(t *testing.T, statuses map[string]string, outcome string, bars bool) *IntradayHandler {
		t.Helper()
		db := newMarketDayDB(t)
		ev := store.NewIntradayEvidenceRepo(db)
		candles := store.NewCandleRepo(db)
		if outcome != "" {
			if err := ev.InsertRound(ctx, store.IntradayRound{RunID: 9, Source: store.IntradaySourceScheduled, ScheduledAt: tpeAt(9, 55),
				LockAcquiredAt: nullTime(tpeAt(9, 55)), Outcome: outcome}); err != nil {
				t.Fatal(err)
			}
		}
		rows := []store.IntradaySymbolRun{}
		for sym, st := range statuses {
			rows = append(rows, store.IntradaySymbolRun{RunID: 9, Symbol: sym, Source: store.IntradaySourceScheduled, DataAt: tpeAt(9, 55), DataStatus: st, FetchAttempts: 1})
		}
		if err := ev.InsertSymbolRuns(ctx, rows); err != nil {
			t.Fatal(err)
		}
		if bars {
			if err := candles.BulkInsert(ctx, []store.Candle{{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpeAt(9, 54)}}); err != nil {
				t.Fatal(err)
			}
		}
		judge := tradingday.New(calStub{}, nil, zap.NewNop())
		h := NewIntradayHandler(ev, intradayWatchlistStub{symbols: []string{"A", "B"}}, candles, schedWithJudge(judge), zap.NewNop())
		h.SetClockForTest(func() time.Time { return tpeAt(10, 0) })
		return h
	}
	exec := store.IntradayOutcomeExecuted
	for _, c := range []struct {
		name     string
		statuses map[string]string
		outcome  string
		bars     bool
		want     string
	}{
		{"K 棒全部寫入失敗", map[string]string{"A": "store_failed", "B": "store_failed"}, exec, false, sessionDataStoreFailing},
		{"抓取失敗", map[string]string{"A": "fetch_failed", "B": "store_failed"}, exec, false, sessionDataFetchFailing},
		{"輪次 failed（讀不到自選清單）", map[string]string{}, store.IntradayOutcomeFailed, false, sessionDataFetchFailing},
		{"沒有已知失敗", map[string]string{"A": "no_data", "B": "no_data"}, exec, false, sessionDataNoneYet},
		{"今天已有 K 棒", map[string]string{"A": "written"}, exec, true, sessionDataOK},
	} {
		t.Run(c.name, func(t *testing.T) {
			if v := getStatus(t, newH(t, c.statuses, c.outcome, c.bars)); v.SessionData != c.want {
				t.Fatalf("session_data = %s，期望 %s", v.SessionData, c.want)
			}
		})
	}
	t.Run("查 DB 失敗", func(t *testing.T) {
		db := newMarketDayDB(t)
		h := NewIntradayHandler(store.NewIntradayEvidenceRepo(db), intradayWatchlistStub{symbols: []string{"A"}},
			failingLatestCandles{}, schedWithJudge(tradingday.New(calStub{}, nil, zap.NewNop())), zap.NewNop())
		h.SetClockForTest(func() time.Time { return tpeAt(10, 0) })
		if v := getStatus(t, h); v.SessionData != sessionDataUnavailable {
			t.Fatalf("session_data = %s", v.SessionData)
		}
	})
	t.Run("自選清單是空的", func(t *testing.T) {
		db := newMarketDayDB(t)
		h := NewIntradayHandler(store.NewIntradayEvidenceRepo(db), intradayWatchlistStub{}, store.NewCandleRepo(db),
			schedWithJudge(tradingday.New(calStub{}, nil, zap.NewNop())), zap.NewNop())
		h.SetClockForTest(func() time.Time { return tpeAt(10, 0) })
		if v := getStatus(t, h); v.SessionData != sessionDataEmptyWatchlist {
			t.Fatalf("session_data = %s", v.SessionData)
		}
	})
}

func overrideRouter(h *MarketDayOverrideHandler, userID uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", userID) })
	r.GET("/market-day-overrides", h.List)
	r.PUT("/market-day-overrides/:date", h.Put)
	r.DELETE("/market-day-overrides/:date", h.Delete)
	return r
}

func call(t *testing.T, r *gin.Engine, method, path, body string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestMarketDayOverrides(t *testing.T) {
	db := newMarketDayDB(t)
	repo := store.NewMarketDayOverrideRepo(db)
	judge := tradingday.New(calStub{closed: map[string]bool{"2026-11-04": true}}, repo, zap.NewNop())
	h := NewMarketDayOverrideHandler(repo, schedWithJudge(judge), zap.NewNop())
	h.SetClockForTest(func() time.Time { return tpeAt(8, 0) }) // 2026-11-02（週一）
	r := overrideRouter(h, 7)

	for _, c := range []struct {
		name, path, body, err string
		code                  int
	}{
		{"過去的日期", "/market-day-overrides/2026-11-01", `{"action":"closed","reason":"x"}`, "past_date", 422},
		{"週末", "/market-day-overrides/2026-11-07", `{"action":"open","reason":"x"}`, "weekend", 422},
		{"日期格式", "/market-day-overrides/2026-11-2", `{"action":"closed","reason":"x"}`, "invalid_date", 422},
		{"不存在的日期", "/market-day-overrides/2026-11-31", `{"action":"closed","reason":"x"}`, "invalid_date", 422},
		{"action 值域", "/market-day-overrides/2026-11-03", `{"action":"cleared","reason":"x"}`, "invalid_action", 422},
		{"原因空白", "/market-day-overrides/2026-11-03", `{"action":"closed","reason":"  "}`, "reason_required", 422},
		{"原因太長", "/market-day-overrides/2026-11-03", `{"action":"closed","reason":"` + strings.Repeat("颱", 201) + `"}`, "reason_too_long", 422},
		{"body 壞掉", "/market-day-overrides/2026-11-03", `{`, "invalid_body", 400},
	} {
		if code, out := call(t, r, http.MethodPut, c.path, c.body); code != c.code || out["error"] != c.err {
			t.Errorf("%s：得到 %d %v，期望 %d %s", c.name, code, out, c.code, c.err)
		}
	}

	// 今天手動休市 → 回傳的 trading_day 立刻是 manual 休市
	code, out := call(t, r, http.MethodPut, "/market-day-overrides/2026-11-02", `{"action":"closed","reason":" 證交所公告颱風全日休市 "}`)
	if code != 200 {
		t.Fatalf("PUT：%d %v", code, out)
	}
	ov := out["override"].(map[string]any)
	td := out["trading_day"].(map[string]any)
	if ov["action"] != "closed" || ov["reason"] != "證交所公告颱風全日休市" || ov["user_id"].(float64) != 7 || td["is_trading"] != false || td["source"] != "manual" {
		t.Fatalf("PUT 回傳：%v", out)
	}
	// 日曆休市日（11-04）手動照常開市
	if code, out := call(t, r, http.MethodPut, "/market-day-overrides/2026-11-04", `{"action":"open","reason":"公告照常交易"}`); code != 200 ||
		out["trading_day"].(map[string]any)["is_trading"] != true {
		t.Fatalf("手動開市：%d %v", code, out)
	}
	// 取消：沒有生效中的設定 → 409；有 → 寫 cleared、回到日曆
	if code, out := call(t, r, http.MethodDelete, "/market-day-overrides/2026-11-03", `{"reason":"x"}`); code != 409 || out["error"] != "no_active_override" {
		t.Fatalf("沒有設定時取消：%d %v", code, out)
	}
	if code, out := call(t, r, http.MethodDelete, "/market-day-overrides/2026-11-02", `{"reason":""}`); code != 422 || out["error"] != "reason_required" {
		t.Fatalf("取消也要填原因：%d %v", code, out)
	}
	code, out = call(t, r, http.MethodDelete, "/market-day-overrides/2026-11-02", `{"reason":"誤設"}`)
	if code != 200 || out["override"].(map[string]any)["action"] != "cleared" || out["trading_day"].(map[string]any)["source"] != "calendar" {
		t.Fatalf("取消：%d %v", code, out)
	}
	if code, _ := call(t, r, http.MethodDelete, "/market-day-overrides/2026-11-02", `{"reason":"再取消一次"}`); code != 409 {
		t.Fatalf("已取消的再取消要 409，得到 %d", code)
	}

	// GET：今天的判斷、目前生效中的設定、完整操作紀錄
	code, out = call(t, r, http.MethodGet, "/market-day-overrides", "")
	if code != 200 {
		t.Fatalf("GET：%d %v", code, out)
	}
	today := out["today"].(map[string]any)
	if today["date"] != "2026-11-02" || today["is_trading"] != true || today["source"] != "calendar" {
		t.Fatalf("today：%v", today)
	}
	cur := out["current"].([]any)
	if len(cur) != 1 || cur[0].(map[string]any)["trade_date"] != "2026-11-04" {
		t.Fatalf("current 只剩 11-04 的手動開市：%v", cur)
	}
	if hist := out["history"].([]any); len(hist) != 3 {
		t.Fatalf("history 要有 3 列（休市、取消、開市），得到 %v", hist)
	}
	if code, out := call(t, r, http.MethodGet, "/market-day-overrides?from=2026-11-3", ""); code != 400 || out["error"] != "invalid_from" {
		t.Fatalf("from 格式：%d %v", code, out)
	}
}

// interleavingRepo 模擬「別的頁面剛好在這次寫入之後也寫了一筆」，而且讀回（Latest）壞掉。
type interleavingRepo struct {
	store.MarketDayOverrideRepo
	other     bool
	failWrite bool
}

func (r interleavingRepo) Insert(ctx context.Context, o store.MarketDayOverride) (store.MarketDayOverride, error) {
	if r.failWrite {
		return store.MarketDayOverride{}, errors.New("insert failed")
	}
	saved, err := r.MarketDayOverrideRepo.Insert(ctx, o)
	if err == nil && r.other {
		_, _ = r.MarketDayOverrideRepo.Insert(ctx, store.MarketDayOverride{TradeDate: o.TradeDate, Action: store.MarketDayOpen, Reason: "另一個頁面", UserID: 99})
	}
	return saved, err
}

func (interleavingRepo) Latest(context.Context, string) (*store.MarketDayOverride, error) {
	return nil, errors.New("read back failed")
}

// v1.2 實作 review [P2]：寫入之後回傳的要是這一次寫入的那一列——別的頁面同時寫入、或讀回壞掉，都⛔ 不能回錯的紀錄或 500。
func TestMarketDayOverrideWriteReturnsOwnRow(t *testing.T) {
	db := newMarketDayDB(t)
	real := store.NewMarketDayOverrideRepo(db)
	judge := tradingday.New(calStub{}, real, zap.NewNop())
	h := NewMarketDayOverrideHandler(interleavingRepo{MarketDayOverrideRepo: real, other: true}, schedWithJudge(judge), zap.NewNop())
	h.SetClockForTest(func() time.Time { return tpeAt(8, 0) })
	code, out := call(t, overrideRouter(h, 7), http.MethodPut, "/market-day-overrides/2026-11-03", `{"action":"closed","reason":"我的設定"}`)
	if code != 200 {
		t.Fatalf("寫入成功就要回 200（⛔ 不依賴讀回），得到 %d %v", code, out)
	}
	ov := out["override"].(map[string]any)
	if ov["reason"] != "我的設定" || ov["user_id"].(float64) != 7 || ov["action"] != "closed" {
		t.Fatalf("要回傳這次寫入的那一列，⛔ 不是別的頁面的，得到 %v", ov)
	}

	// 寫入失敗：回 500，而且⛔ 沒有寫進去
	hf := NewMarketDayOverrideHandler(interleavingRepo{MarketDayOverrideRepo: real, failWrite: true}, schedWithJudge(judge), zap.NewNop())
	hf.SetClockForTest(func() time.Time { return tpeAt(8, 0) })
	if code, _ := call(t, overrideRouter(hf, 7), http.MethodPut, "/market-day-overrides/2026-11-04", `{"action":"closed","reason":"x"}`); code != 500 {
		t.Fatalf("寫入失敗要回 500，得到 %d", code)
	}
	if o, err := real.Latest(context.Background(), "2026-11-04"); err != nil || o != nil {
		t.Fatalf("寫入失敗時⛔ 不該留下紀錄，得到 %+v %v", o, err)
	}
}

func TestMarketDayOverrideConcurrentWrites(t *testing.T) {
	db := newMarketDayDB(t)
	repo := store.NewMarketDayOverrideRepo(db)
	h := NewMarketDayOverrideHandler(repo, schedWithJudge(tradingday.New(calStub{}, repo, zap.NewNop())), zap.NewNop())
	h.SetClockForTest(func() time.Time { return tpeAt(8, 0) })
	r := overrideRouter(h, 7)

	const n = 8
	type res struct {
		code   int
		reason string
		id     float64
		want   string
	}
	results := make(chan res, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			action := []string{"closed", "open"}[i%2]
			want := "頁面 " + string(rune('A'+i))
			code, out := call(t, r, http.MethodPut, "/market-day-overrides/2026-11-03", `{"action":"`+action+`","reason":"`+want+`"}`)
			got := res{code: code, want: want}
			if ov, ok := out["override"].(map[string]any); ok {
				got.reason, _ = ov["reason"].(string)
				got.id, _ = ov["id"].(float64)
			}
			results <- got
		}(i)
	}
	ids := map[float64]bool{}
	for i := 0; i < n; i++ {
		g := <-results
		if g.code != 200 || g.reason != g.want {
			t.Fatalf("並行寫入：每個請求都要拿回自己的那一列，得到 %+v", g)
		}
		ids[g.id] = true
	}
	if len(ids) != n {
		t.Fatalf("每一筆都要是不同的列，得到 %d 個 id", len(ids))
	}
	if hist, _ := repo.ListSince(context.Background(), "2026-11-03"); len(hist) != n {
		t.Fatalf("操作紀錄要有 %d 列，得到 %d", n, len(hist))
	}
}
