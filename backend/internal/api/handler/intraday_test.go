package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type intradayWatchlistStub struct {
	store.WatchlistRepo
	symbols []string
}

func (s intradayWatchlistStub) Symbols(context.Context) ([]string, error) { return s.symbols, nil }

func tpeAt(h, m int) time.Time { return time.Date(2026, 11, 2, h, m, 0, 0, timeutil.TaipeiTZ) }

func nullTime(t time.Time) store.NullTime { return store.NullTime{NullTime: sql.NullTime{Time: t, Valid: true}} }
func nullStr(s string) store.NullString {
	return store.NullString{NullString: sql.NullString{String: s, Valid: true}}
}

// T-088：狀態 API 帶出最近一輪的輪次紀錄；最近一輪被跳過時標 round_skipped（⛔ 不把上一輪的評估當成這一輪的）；
// 評估段還沒寫時分 evaluating／interrupted；手動補抓比排程新才帶；最後有效 K 棒與延遲由 DB 的 K 棒算。
func TestIntradayStatus(t *testing.T) {
	ctx := context.Background()
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/status.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(ctx, db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	evidence := store.NewIntradayEvidenceRepo(db)
	candles := store.NewCandleRepo(db)

	// 09:10 正常評估（A no_signal、B 評估段沒寫）；09:15 等鎖逾時；A 在 09:12 補抓過。
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(evidence.InsertRound(ctx, store.IntradayRound{RunID: 1, Source: store.IntradaySourceScheduled, ScheduledAt: tpeAt(9, 10),
		LockAcquiredAt: nullTime(tpeAt(9, 10)), Outcome: store.IntradayOutcomeExecuted}))
	must(evidence.InsertSymbolRuns(ctx, []store.IntradaySymbolRun{
		{RunID: 1, Symbol: "A", Source: store.IntradaySourceScheduled, DataAt: tpeAt(9, 10), DataStatus: "written",
			LastBarTS: nullTime(tpeAt(9, 10)), Fingerprint: nullStr("fa"), FetchAttempts: 1},
		{RunID: 1, Symbol: "B", Source: store.IntradaySourceScheduled, DataAt: tpeAt(9, 10), DataStatus: "written", FetchAttempts: 1},
	}))
	must(evidence.UpdateSymbolEval(ctx, 1, "A", tpeAt(9, 10), store.IntradayEvalNoSignal, ""))
	must(evidence.InsertSymbolRuns(ctx, []store.IntradaySymbolRun{
		{RunID: 2, Symbol: "A", Source: store.IntradaySourceRefill, DataAt: tpeAt(9, 12), DataStatus: "written", FetchAttempts: 1},
	}))
	must(evidence.InsertRound(ctx, store.IntradayRound{RunID: 3, Source: store.IntradaySourceScheduled, ScheduledAt: tpeAt(9, 15),
		LockWaitMs: 60000, Outcome: store.IntradayOutcomeSkippedLockTimeout}))
	must(candles.BulkInsert(ctx, []store.Candle{{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpeAt(9, 10)}}))

	h := NewIntradayHandler(evidence, intradayWatchlistStub{symbols: []string{"A", "B", "C"}}, candles, nil, zap.NewNop())
	h.SetClockForTest(func() time.Time { return tpeAt(9, 25) })
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/intraday/status", h.GetStatus)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/intraday/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d：%s", w.Code, w.Body.String())
	}

	var resp struct {
		MarketOpen        bool `json:"market_open"`
		StaleAfterMinutes int  `json:"stale_after_minutes"`
		LatestRound       *struct {
			RunID   uint64 `json:"run_id"`
			Outcome string `json:"outcome"`
			Running bool   `json:"running"`
		} `json:"latest_round"`
		Symbols []struct {
			Symbol string `json:"symbol"`
			Latest *struct {
				RunID      uint64  `json:"run_id"`
				EvalStatus *string `json:"eval_status"`
			} `json:"latest"`
			Refill *struct {
				RunID uint64 `json:"run_id"`
			} `json:"refill"`
			LastSuccessEvalAt *time.Time `json:"last_success_eval_at"`
			LastBarTS         *time.Time `json:"last_bar_ts"`
			LagMinutes        *int       `json:"lag_minutes"`
			Stale             bool       `json:"stale"`
			RoundState        string     `json:"round_state"`
			EvalPending       string     `json:"eval_pending"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.MarketOpen || resp.StaleAfterMinutes != 10 || resp.LatestRound == nil || resp.LatestRound.RunID != 3 ||
		resp.LatestRound.Outcome != store.IntradayOutcomeSkippedLockTimeout || resp.LatestRound.Running {
		t.Fatalf("最近一輪 = %s", w.Body.String())
	}
	a, b, c := resp.Symbols[0], resp.Symbols[1], resp.Symbols[2]
	// 09:12 的補抓（run 2）比 09:15 的輪次（run 3）舊：以輪次紀錄為準，⛔ 不帶補抓。
	if a.Latest == nil || a.Latest.RunID != 1 || a.Latest.EvalStatus == nil || *a.Latest.EvalStatus != store.IntradayEvalNoSignal ||
		a.RoundState != "skipped" || a.Refill != nil || a.LastSuccessEvalAt == nil || a.EvalPending != "" {
		t.Fatalf("A = %+v", a)
	}
	if a.LastBarTS == nil || !a.LastBarTS.Equal(tpeAt(9, 10)) || a.LagMinutes == nil || *a.LagMinutes != 15 || !a.Stale {
		t.Fatalf("A 的最後有效 K 棒與延遲 = %+v", a)
	}
	if b.EvalPending != "interrupted" || b.RoundState != "skipped" || b.LastBarTS != nil || b.LagMinutes != nil {
		t.Fatalf("B = %+v", b)
	}
	if c.Latest != nil || c.RoundState != "skipped" {
		t.Fatalf("C 沒有逐檔紀錄 = %+v", c)
	}
}

// review 2026-11-20：讀不到自選清單的輪次（outcome=failed）也以輪次紀錄為準，⛔ 不沿用上一輪的「未觸發」；
// 已結束、卻沒有這一檔的列 → incomplete；比輪次新的補抓照帶。
func TestIntradayStatusRoundStates(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		rounds []store.IntradayRound
		refill bool
		want   string
		wantRf bool
	}{
		{"讀不到自選清單", []store.IntradayRound{{RunID: 5, Outcome: store.IntradayOutcomeFailed}}, false, "failed", false},
		{"已結束卻沒有這一檔的列", []store.IntradayRound{{RunID: 5, Outcome: store.IntradayOutcomeExecuted,
			ExecFinishedAt: nullTime(tpeAt(9, 16))}}, false, "incomplete", false},
		{"比輪次新的補抓", []store.IntradayRound{{RunID: 5, Outcome: store.IntradayOutcomeFailed}}, true, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/rs.db"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			if err := database.RunMigrations(ctx, db, "sqlite", zap.NewNop()); err != nil {
				t.Fatal(err)
			}
			ev := store.NewIntradayEvidenceRepo(db)
			if err := ev.InsertSymbolRuns(ctx, []store.IntradaySymbolRun{{RunID: 1, Symbol: "A", Source: store.IntradaySourceScheduled,
				DataAt: tpeAt(9, 10), DataStatus: "written", FetchAttempts: 1}}); err != nil {
				t.Fatal(err)
			}
			if err := ev.UpdateSymbolEval(ctx, 1, "A", tpeAt(9, 10), store.IntradayEvalNoSignal, ""); err != nil {
				t.Fatal(err)
			}
			for _, r := range tc.rounds {
				r.Source, r.ScheduledAt = store.IntradaySourceScheduled, tpeAt(9, 15)
				if err := ev.InsertRound(ctx, r); err != nil {
					t.Fatal(err)
				}
			}
			if tc.refill {
				if err := ev.InsertSymbolRuns(ctx, []store.IntradaySymbolRun{{RunID: 6, Symbol: "A", Source: store.IntradaySourceRefill,
					DataAt: tpeAt(9, 20), DataStatus: "written", FetchAttempts: 1}}); err != nil {
					t.Fatal(err)
				}
			}
			h := NewIntradayHandler(ev, intradayWatchlistStub{symbols: []string{"A"}}, store.NewCandleRepo(db), nil, zap.NewNop())
			h.SetClockForTest(func() time.Time { return tpeAt(9, 25) })
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/s", h.GetStatus)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s", nil))
			var resp struct {
				Symbols []struct {
					RoundState string          `json:"round_state"`
					Refill     json.RawMessage `json:"refill"`
				} `json:"symbols"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			got := resp.Symbols[0]
			if got.RoundState != tc.want || (string(got.Refill) != "null") != tc.wantRf {
				t.Fatalf("round_state=%q refill=%s，期望 %q／帶補抓=%v", got.RoundState, got.Refill, tc.want, tc.wantRf)
			}
		})
	}
}

// 補抓的錯誤對照：不在自選清單 400、沒有批次盤中源 409。
func TestIntradayRefillErrors(t *testing.T) {
	watch := intradayWatchlistStub{symbols: []string{"A"}}
	fetcher := market.NewFetcher(nil, nil, zap.NewNop())
	s := scheduler.New(fetcher, nil, watch, &jobRunRepoStub{}, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop())
	h := NewIntradayHandler(nil, watch, nil, s, zap.NewNop())
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/run", h.RunRefill)

	for _, tc := range []struct {
		body string
		code int
		want string
	}{
		{"", http.StatusConflict, "intraday_source_unavailable"},
		{`{"symbols":`, http.StatusBadRequest, "invalid_body"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(tc.body)))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("body %q → %d %s，期望 %d %s", tc.body, w.Code, w.Body.String(), tc.code, tc.want)
		}
	}
}
