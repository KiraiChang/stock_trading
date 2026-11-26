package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
	"github.com/trading/backend/pkg/timeutil"
)

// ── T-089 D2 的測試替身 ─────────────────────────────────────────

type planPositionRepo struct {
	tradePositionRepoStub
	pos    *store.Position
	getErr error
}

func (r *planPositionRepo) Get(context.Context, uint64, string) (*store.Position, error) {
	return r.pos, r.getErr
}

type planCandleStub struct {
	ts     time.Time
	err    error
	noData bool
}

func (c planCandleStub) GetLatest(ctx context.Context, symbol, timeframe string) (*store.Candle, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.noData {
		return nil, sql.ErrNoRows
	}
	return &store.Candle{Symbol: symbol, Timeframe: timeframe, Timestamp: c.ts}, nil
}

type planSRRefStub struct {
	refs  []store.SRZoneAnalysisSourceRef
	err   error
	calls int
	ids   []uint64
}

func (s *planSRRefStub) GetSourceRefs(ctx context.Context, ids []uint64) ([]store.SRZoneAnalysisSourceRef, error) {
	s.calls++
	s.ids = append(s.ids, ids...)
	return s.refs, s.err
}

// planJudgeStub：週末休市；closed 的日期休市（calendar）；fallback 的日期回 fallback；unavailable 的日期回 calendar＋OverrideUnavailable。
type planJudgeStub struct {
	closed, fallback, unavailable map[string]bool
}

func (j planJudgeStub) On(_ context.Context, day time.Time) tradingday.Result {
	l := day.In(timeutil.TaipeiTZ)
	d := l.Format("2006-01-02")
	if wd := l.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return tradingday.Result{IsTrading: false, Source: tradingday.SourceWeekend}
	}
	if j.fallback[d] {
		return tradingday.Result{IsTrading: true, Source: tradingday.SourceFallback}
	}
	return tradingday.Result{IsTrading: !j.closed[d], Source: tradingday.SourceCalendar, OverrideUnavailable: j.unavailable[d]}
}

func planAt(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, timeutil.TaipeiTZ)
}

type planEnv struct {
	repo    *planPositionRepo
	candles planCandleStub
	srRefs  *planSRRefStub
	judge   planJudgeStub
	now     time.Time
	noJudge bool
}

func (e *planEnv) handler() *TradeAnalysisHandler {
	if e.repo == nil {
		e.repo = &planPositionRepo{}
	}
	if e.srRefs == nil {
		e.srRefs = &planSRRefStub{}
	}
	h := &TradeAnalysisHandler{
		repo: e.repo, analyzer: &tradeAnalyzerStub{}, log: zap.NewNop(),
		candles: e.candles, srRefs: e.srRefs, now: func() time.Time { return e.now },
	}
	if !e.noJudge {
		j := e.judge
		h.judge = func() tradeDayJudge { return j }
	}
	return h
}

type planHistoryResp struct {
	Analyses   []map[string]any `json:"analyses"`
	DataStatus map[string]any   `json:"data_status"`
}

func (e *planEnv) history(t *testing.T) planHistoryResp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/trade-analysis/:symbol/history", e.handler().ListHistory)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trade-analysis/2330/history?portfolio_id=2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out planHistoryResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return out
}

func dsString(t *testing.T, ds map[string]any, key string) any {
	t.Helper()
	v, ok := ds[key]
	if !ok {
		t.Fatalf("data_status 少了 %s：%v", key, ds)
	}
	return v
}

// 新紀錄（evidence 帶 timeframe）
func newPlan(id uint64, tf string, at time.Time) store.PositionAnalysis {
	return store.PositionAnalysis{
		ID: id, Symbol: "2330", RuleVersion: "position_sr_zone_v1", AnalyzedAt: at, CreatedAt: at.Add(16 * time.Hour),
		SRZoneAnalysisID: store.NewNullInt64(int64(100 + id)),
		Evidence:         store.RawJSON(`{"decision_context":{"mode":"FLAT_ENTRY","timeframe":"` + tf + `","market_data_at":"` + at.UTC().Format(time.RFC3339) + `"}}`),
	}
}

// 本案之前的現行紀錄（evidence 沒有 timeframe 鍵，只能回查快照）
func oldPlan(id uint64, srID int64, at time.Time) store.PositionAnalysis {
	return store.PositionAnalysis{
		ID: id, Symbol: "2330", RuleVersion: "position_sr_zone_v1", AnalyzedAt: at, CreatedAt: at.Add(16 * time.Hour),
		SRZoneAnalysisID: store.NewNullInt64(srID),
		Evidence:         store.RawJSON(`{"decision_context":{"mode":"FLAT_ENTRY","sr_zone_analysis_id":` + jsonInt(srID) + `}}`),
	}
}

func jsonInt(v int64) string { b, _ := json.Marshal(v); return string(b) }

// ── source_confirmed_1d 與 market_date ─────────────────────────

func TestTradePlanSourceConfirmed(t *testing.T) {
	day := planAt(2026, 11, 24, 0, 0)
	other := planAt(2026, 11, 21, 0, 0)
	legacy := store.PositionAnalysis{
		ID: 1, Symbol: "2330", RuleVersion: "holding_sr_zone_v1_legacy", AnalyzedAt: day, CreatedAt: day,
		SRZoneAnalysisID: store.NewNullInt64(501), // 038 原樣搬移的舊 SR id，而且那份快照「看起來」對得上
	}
	rows := []store.PositionAnalysis{
		legacy,
		newPlan(2, "1d", day),
		newPlan(3, "1m", day),
		oldPlan(4, 504, day),   // 快照是 1d、時間相同 → true
		oldPlan(5, 505, day),   // 快照已刪 → false
		oldPlan(6, 506, day),   // 快照時間對不上 → false
		oldPlan(7, 507, day),   // 快照是 1h → false
		oldPlan(8, 504, other), // 同一份快照、計畫時間不同 → false
	}
	refs := &planSRRefStub{refs: []store.SRZoneAnalysisSourceRef{
		{ID: 501, Timeframe: "1d", AnalyzedAt: day},
		{ID: 504, Timeframe: "1d", AnalyzedAt: day},
		{ID: 506, Timeframe: "1d", AnalyzedAt: other},
		{ID: 507, Timeframe: "1h", AnalyzedAt: day},
	}}
	env := &planEnv{
		repo:    &planPositionRepo{tradePositionRepoStub: tradePositionRepoStub{analyses: rows}},
		candles: planCandleStub{ts: day}, srRefs: refs, now: planAt(2026, 11, 25, 9, 0),
	}
	out := env.history(t)
	want := map[float64]bool{1: false, 2: true, 3: false, 4: true, 5: false, 6: false, 7: false, 8: false}
	for _, a := range out.Analyses {
		id := a["id"].(float64)
		if got := a["source_confirmed_1d"].(bool); got != want[id] {
			t.Errorf("id %v：source_confirmed_1d 要是 %v，得到 %v", id, want[id], got)
		}
		md, has := a["market_date"]
		if !has {
			t.Errorf("id %v：要有 market_date 鍵（null 也要出現）", id)
		}
		if want[id] && md != "2026-11-24" {
			t.Errorf("id %v：確認的計畫 market_date 要是台北日期 2026-11-24，得到 %v", id, md)
		}
		if !want[id] && md != nil {
			t.Errorf("id %v：來源無法確認時 market_date 要是 null，得到 %v", id, md)
		}
		if _, ok := a["action_label"]; !ok {
			t.Errorf("id %v：原本的欄位要照舊攤平在同一層", id)
		}
	}
	if len(out.Analyses) != len(rows) {
		t.Fatalf("⛔ 不收窄：要回全部 %d 筆，得到 %d", len(rows), len(out.Analyses))
	}
	if refs.calls != 1 {
		t.Fatalf("回查要一句查詢一次取完，呼叫了 %d 次", refs.calls)
	}
	gotIDs := map[uint64]bool{}
	for _, id := range refs.ids {
		gotIDs[id] = true
	}
	if gotIDs[501] || gotIDs[102] || gotIDs[103] || !gotIDs[504] || !gotIDs[505] || !gotIDs[506] || !gotIDs[507] || len(refs.ids) != 4 {
		t.Fatalf("只回查需要的 id（舊紀錄與帶 timeframe 的新紀錄⛔ 不查、同一個 id 只查一次），得到 %v", refs.ids)
	}
	if out.DataStatus["source_lookup"] != "ok" {
		t.Fatalf("source_lookup 要是 ok，得到 %v", out.DataStatus["source_lookup"])
	}
}

func TestTradePlanSourceLookupFailure(t *testing.T) {
	day := planAt(2026, 11, 24, 0, 0)
	rows := []store.PositionAnalysis{newPlan(2, "1d", day), oldPlan(4, 504, day)}
	env := &planEnv{
		repo:    &planPositionRepo{tradePositionRepoStub: tradePositionRepoStub{analyses: rows}},
		candles: planCandleStub{ts: day}, srRefs: &planSRRefStub{err: errors.New("db down")}, now: planAt(2026, 11, 25, 9, 0),
	}
	out := env.history(t)
	if out.DataStatus["source_lookup"] != "error" {
		t.Fatalf("回查失敗：source_lookup 要是 error，得到 %v", out.DataStatus["source_lookup"])
	}
	for _, a := range out.Analyses {
		id := a["id"].(float64)
		got := a["source_confirmed_1d"].(bool)
		if id == 4 && (got || a["market_date"] != nil) {
			t.Errorf("要回查的那筆在回查失敗時當 false、market_date null，得到 %v %v", got, a["market_date"])
		}
		if id == 2 && !got {
			t.Errorf("evidence 帶 timeframe 的新紀錄⛔ 不依賴回查，仍是 true")
		}
	}
}

// ── data_status ────────────────────────────────────────────────

func TestTradePlanLatestCandleStatus(t *testing.T) {
	now := planAt(2026, 11, 25, 9, 0)
	cases := []struct {
		name     string
		candles  planCandleStub
		want     string
		wantDate any
	}{
		{"有日 K", planCandleStub{ts: planAt(2026, 11, 24, 0, 0)}, "ok", "2026-11-24"},
		{"查無日 K", planCandleStub{noData: true}, "no_data", nil},
		{"查詢失敗（⛔ 不是 no_data）", planCandleStub{err: errors.New("db down")}, "error", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := &planEnv{candles: c.candles, now: now}
			ds := env.history(t).DataStatus
			if dsString(t, ds, "latest_candle") != c.want || dsString(t, ds, "latest_candle_date") != c.wantDate {
				t.Fatalf("要 %s／%v，得到 %v／%v", c.want, c.wantDate, ds["latest_candle"], ds["latest_candle_date"])
			}
		})
	}
	t.Run("沒有注入 candles → error", func(t *testing.T) {
		h := (&planEnv{now: now}).handler()
		h.candles = nil
		st := h.buildDataStatus(context.Background(), 2, "2330", "ok")
		if st.LatestCandle != "error" {
			t.Fatalf("要 error，得到 %s", st.LatestCandle)
		}
	})
}

func TestTradePlanExpectedDate(t *testing.T) {
	holidays := planJudgeStub{closed: map[string]bool{
		"2026-12-24": true, "2026-12-25": true, // 週四、週五連假
		"2027-01-01": true,
	}}
	cases := []struct {
		name              string
		now               time.Time
		judge             planJudgeStub
		expected, settled string
		calendar          string
	}{
		{"交易日 09:00 → 前一交易日", planAt(2026, 11, 25, 9, 0), planJudgeStub{}, "2026-11-24", "2026-11-24", "confirmed"},
		{"交易日 13:29 → 前一交易日", planAt(2026, 11, 25, 13, 29), planJudgeStub{}, "2026-11-24", "2026-11-24", "confirmed"},
		{"交易日 13:30 → 今天", planAt(2026, 11, 25, 13, 30), planJudgeStub{}, "2026-11-25", "2026-11-24", "confirmed"},
		{"週一盤前 → 上週五", planAt(2026, 11, 23, 8, 0), planJudgeStub{}, "2026-11-20", "2026-11-20", "confirmed"},
		{"週六 → 週五", planAt(2026, 11, 28, 15, 0), planJudgeStub{}, "2026-11-27", "2026-11-27", "confirmed"},
		{"連假中 → 連假前最後一個交易日", planAt(2026, 12, 25, 16, 0), holidays, "2026-12-23", "2026-12-23", "confirmed"},
		{"跨年：1 月初往回推到前一年", planAt(2027, 1, 4, 9, 0), holidays, "2026-12-31", "2026-12-31", "confirmed"},
		{"往回推碰到 fallback → unconfirmed", planAt(2026, 11, 25, 9, 0),
			planJudgeStub{fallback: map[string]bool{"2026-11-24": true}}, "2026-11-24", "2026-11-24", "unconfirmed"},
		{"今天是 fallback → unconfirmed", planAt(2026, 11, 25, 14, 0),
			planJudgeStub{fallback: map[string]bool{"2026-11-25": true}}, "2026-11-25", "2026-11-24", "unconfirmed"},
		{"往回推的某一天讀不到手動設定 → unconfirmed", planAt(2026, 11, 23, 9, 0),
			planJudgeStub{unavailable: map[string]bool{"2026-11-20": true}}, "2026-11-20", "2026-11-20", "unconfirmed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := &planEnv{candles: planCandleStub{ts: planAt(2026, 11, 20, 0, 0)}, judge: c.judge, now: c.now}
			ds := env.history(t).DataStatus
			if ds["expected_date"] != c.expected || ds["settled_date"] != c.settled || ds["calendar"] != c.calendar {
				t.Fatalf("要 expected %s、settled %s、calendar %s；得到 %v、%v、%v",
					c.expected, c.settled, c.calendar, ds["expected_date"], ds["settled_date"], ds["calendar"])
			}
		})
	}
	t.Run("往回 15 天找不到交易日 → unconfirmed、settled null", func(t *testing.T) {
		closed := map[string]bool{}
		for d := planAt(2026, 11, 1, 0, 0); d.Before(planAt(2026, 11, 26, 0, 0)); d = d.AddDate(0, 0, 1) {
			closed[d.Format("2006-01-02")] = true
		}
		env := &planEnv{candles: planCandleStub{noData: true}, judge: planJudgeStub{closed: closed}, now: planAt(2026, 11, 25, 9, 0)}
		ds := env.history(t).DataStatus
		if ds["calendar"] != "unconfirmed" || ds["settled_date"] != nil || ds["expected_date"] != nil {
			t.Fatalf("要 unconfirmed、null、null；得到 %v %v %v", ds["calendar"], ds["settled_date"], ds["expected_date"])
		}
	})
	t.Run("沒有共用判斷（noJudge：平日 fallback）→ unconfirmed", func(t *testing.T) {
		env := &planEnv{candles: planCandleStub{ts: planAt(2026, 11, 24, 0, 0)}, now: planAt(2026, 11, 25, 9, 0), noJudge: true}
		if ds := env.history(t).DataStatus; ds["calendar"] != "unconfirmed" {
			t.Fatalf("要 unconfirmed，得到 %v", ds["calendar"])
		}
	})
}

func TestTradePlanAwaitingToday(t *testing.T) {
	closed := planJudgeStub{closed: map[string]bool{"2026-11-25": true}}
	cases := []struct {
		name   string
		now    time.Time
		latest time.Time
		judge  planJudgeStub
		want   bool
	}{
		{"交易日 13:30、最新是昨天 → true", planAt(2026, 11, 25, 13, 30), planAt(2026, 11, 24, 0, 0), planJudgeStub{}, true},
		{"交易日 13:30、最新是今天 → false", planAt(2026, 11, 25, 13, 30), planAt(2026, 11, 25, 0, 0), planJudgeStub{}, false},
		{"交易日 13:29 → false", planAt(2026, 11, 25, 13, 29), planAt(2026, 11, 24, 0, 0), planJudgeStub{}, false},
		{"休市日 → false", planAt(2026, 11, 25, 15, 0), planAt(2026, 11, 24, 0, 0), closed, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := &planEnv{candles: planCandleStub{ts: c.latest}, judge: c.judge, now: c.now}
			if got := env.history(t).DataStatus["awaiting_today"]; got != c.want {
				t.Fatalf("awaiting_today 要 %v，得到 %v", c.want, got)
			}
		})
	}
}

func TestTradePlanPositionVersion(t *testing.T) {
	now := planAt(2026, 11, 25, 9, 0)
	cases := []struct {
		name string
		repo *planPositionRepo
		want any
	}{
		{"有持倉列", &planPositionRepo{pos: &store.Position{Version: 5}}, float64(5)},
		{"沒有持倉列（sql.ErrNoRows）→ 0", &planPositionRepo{getErr: sql.ErrNoRows}, float64(0)},
		{"讀取失敗 → null", &planPositionRepo{getErr: errors.New("db down")}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := &planEnv{repo: c.repo, candles: planCandleStub{ts: planAt(2026, 11, 24, 0, 0)}, now: now}
			ds := env.history(t).DataStatus
			if v, ok := ds["position_version"]; !ok || v != c.want {
				t.Fatalf("position_version 要 %v，得到 %v（有鍵 %v）", c.want, v, ok)
			}
		})
	}
}

// Analyze 的回應：analysis 多兩個衍生欄位、頂層多 data_status；既有的 context／sr_zone_analysis／zones 照舊。
func TestTradeAnalysisAnalyzeIncludesPlanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	day := planAt(2026, 11, 24, 0, 0)
	plan := newPlan(9, "1d", day)
	env := &planEnv{
		repo:    &planPositionRepo{pos: &store.Position{Version: 3}},
		candles: planCandleStub{ts: day}, now: planAt(2026, 11, 25, 9, 0),
	}
	h := env.handler()
	h.analyzer = &tradeAnalyzerStub{result: &portfolio.AnalyzeResult{
		Analysis: &plan, SR: &store.SRZoneAnalysis{ID: 109, Symbol: "2330"}, Zones: []store.SRZone{{ID: 1, AnalysisID: 109}},
	}}
	router := gin.New()
	router.POST("/trade-analysis/analyze", h.Analyze)
	req := httptest.NewRequest(http.MethodPost, "/trade-analysis/analyze", bytes.NewBufferString(`{"symbol":"2330","portfolio_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	a := out["analysis"].(map[string]any)
	if a["source_confirmed_1d"] != true || a["market_date"] != "2026-11-24" || a["id"] != float64(9) {
		t.Fatalf("analysis 要帶衍生欄位與原本的欄位，得到 %v", a)
	}
	ds := out["data_status"].(map[string]any)
	if ds["latest_candle"] != "ok" || ds["position_version"] != float64(3) || ds["expected_date"] != "2026-11-24" || ds["source_lookup"] != "ok" {
		t.Fatalf("data_status 不對：%v", ds)
	}
	for _, k := range []string{"context", "sr_zone_analysis", "zones"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("既有的 %s 要照舊在回應裡", k)
		}
	}
	if _, ok := ds["as_of"]; !ok {
		t.Fatal("data_status 要有 as_of")
	}
}
