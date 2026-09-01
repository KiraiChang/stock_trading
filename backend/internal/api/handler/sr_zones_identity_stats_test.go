package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// 身分關聯決策的統計要能落地，而且**降級的那幾次也要留一列**。

// agg 與 rows **刻意分開設定**：本筆要驗的正是「rows 被 limit 截斷、而 summary 沒有」，
// stub 若讓兩者共用同一份 rows 推導，那個情境根本造不出來。
type identityStatsRepoStub struct {
	rows      []store.SRIdentityStats
	agg       store.SRIdentityStatsAggregate
	insertErr error
	// 查詢端的兩條路徑各有自己的錯誤，才驗得出「List 成功但 Summarize 失敗」。
	listErr      error
	summarizeErr error
}

func (s *identityStatsRepoStub) Insert(ctx context.Context, row *store.SRIdentityStats) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.rows = append(s.rows, *row)
	return nil
}

func (s *identityStatsRepoStub) List(ctx context.Context, q store.SRIdentityStatsQuery) ([]store.SRIdentityStats, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	rows := s.rows
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	return rows, nil
}

// Summarize 回傳與 rows 無關的聚合——真實 repo 也是獨立一次查詢，**不套 limit**。
func (s *identityStatsRepoStub) Summarize(ctx context.Context, q store.SRIdentityStatsQuery) (store.SRIdentityStatsAggregate, error) {
	if s.summarizeErr != nil {
		return store.SRIdentityStatsAggregate{}, s.summarizeErr
	}
	return s.agg, nil
}

func newStatsHandler(repo store.SRIdentityStatsRepo) *SRZoneHandler {
	h := &SRZoneHandler{log: zap.NewNop()}
	h.SetIdentityStats(repo)
	return h
}

func TestPersistIdentityStatsMapsEventCounts(t *testing.T) {
	repo := &identityStatsRepoStub{}
	h := newStatsHandler(repo)

	h.persistIdentityStats(context.Background(), 42, "2330", "1d",
		&zoneIdentityMatch{live: make([]store.LiveZone, 7)},
		&zoneIdentityOutcome{},
		&eventIdentityStats{
			MatchedByChain:   5,
			MatchedByCurrent: 3,
			MatchedByAlias:   1,
			UnmatchedKeys:    []string{"k1"},
			ChainConflicts:   []string{"c1", "c2"},
			CarriedParseFail: 4,
			Invariant:        []string{"bad"},
		})

	if len(repo.rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(repo.rows))
	}
	got := repo.rows[0]
	if got.AnalysisID != 42 || got.Symbol != "2330" || got.Timeframe != "1d" {
		t.Fatalf("識別欄位不對：%+v", got)
	}
	if got.MatchedByChain != 5 || got.MatchedByCurrent != 3 || got.MatchedByAlias != 1 {
		t.Errorf("三段命中數不對：%+v", got)
	}
	// **切片欄位存的是長度不是內容**：表只要計數，明細留在 log。
	if got.UnmatchedKeys != 1 || got.ChainConflicts != 2 || got.InvariantViolations != 1 {
		t.Errorf("切片欄位應存長度：%+v", got)
	}
	if got.CarriedParseFail != 4 {
		t.Errorf("carried_parse_fail = %d, want 4", got.CarriedParseFail)
	}
	if got.ZoneLiveCandidates != 7 {
		t.Errorf("zone_live_candidates = %d, want 7（比率的分母參考）", got.ZoneLiveCandidates)
	}
	if got.ZoneIdentityDegraded || got.EventIdentityDegraded {
		t.Error("正常跑完不該標成 degraded")
	}
}

// **降級也要留一列。** 若這時候乾脆不寫，趨勢圖上會看到「這天很乾淨」，
// 而真相是「這天什麼都沒算」——那正是本筆要消滅的那種靜默。
func TestPersistIdentityStatsRecordsDegradedRun(t *testing.T) {
	repo := &identityStatsRepoStub{}
	h := newStatsHandler(repo)

	h.persistIdentityStats(context.Background(), 43, "2330", "1d", nil, nil, nil)

	if len(repo.rows) != 1 {
		t.Fatalf("降級時仍要寫一列，got %d", len(repo.rows))
	}
	got := repo.rows[0]
	if !got.ZoneIdentityDegraded || !got.EventIdentityDegraded {
		t.Errorf("兩個 degraded 旗標都該是 true：%+v", got)
	}
	if got.MatchedByChain != 0 || got.AliasAmbiguous != 0 {
		t.Errorf("降級時計數應全為 0（那正是要靠旗標區分的原因）：%+v", got)
	}
}

// 統計寫入失敗**不能影響分析**——這條與身分層其餘寫入的 fail-open 語意一致。
func TestPersistIdentityStatsFailOpen(t *testing.T) {
	h := newStatsHandler(&identityStatsRepoStub{insertErr: errors.New("db down")})

	// 沒有 panic、沒有回傳值要處理：呼叫端不會因為統計失敗而中斷。
	h.persistIdentityStats(context.Background(), 44, "2330", "1d", nil, nil, nil)
}

// 未注入 repo 時整段不執行，行為與導入前完全相同。
func TestPersistIdentityStatsSkippedWhenRepoMissing(t *testing.T) {
	h := &SRZoneHandler{log: zap.NewNop()}
	h.persistIdentityStats(context.Background(), 45, "2330", "1d", nil, nil, nil)
}

// summary 的比率在查詢時才算——分母隨區間而變，存進表等於把決定寫死。
//
// **吃的是 store 的 aggregate 而不是 rows**：加總是 SQL 的事，
// 這一層只負責 matched_total 與 alias_hit_rate 這兩個 derived 欄位。
func TestSummarizeIdentityStatsComputesAliasHitRate(t *testing.T) {
	got := summarizeIdentityStats(store.SRIdentityStatsAggregate{
		Analyses: 3, Degraded: 1,
		MatchedByChain: 14, MatchedByCurrent: 4, MatchedByAlias: 2,
		AliasAmbiguous: 1,
	})

	if got.Analyses != 3 || got.Degraded != 1 {
		t.Fatalf("analyses/degraded = %d/%d, want 3/1", got.Analyses, got.Degraded)
	}
	if got.MatchedTotal != 20 {
		t.Fatalf("matched_total = %d, want 20", got.MatchedTotal)
	}
	if got.AliasHitRate != 0.1 {
		t.Errorf("alias_hit_rate = %v, want 0.1", got.AliasHitRate)
	}
	if got.AliasAmbiguous != 1 {
		t.Errorf("alias_ambiguous 應原樣帶出：%d", got.AliasAmbiguous)
	}
}

// 沒有任何命中時不能除以零。
func TestSummarizeIdentityStatsHandlesEmpty(t *testing.T) {
	got := summarizeIdentityStats(store.SRIdentityStatsAggregate{})
	if got.Analyses != 0 || got.MatchedTotal != 0 || got.AliasHitRate != 0 {
		t.Errorf("空輸入應全為 0：%+v", got)
	}
}

func getIdentityStats(t *testing.T, h *SRZoneHandler, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sr-zones/identity-stats"+query, nil)
	h.IdentityStats(c)
	return w
}

// **回歸守門**（2026-08-31 修正的 bug）：
// limit 只截斷明細，summary 仍是完整 days 區間的聚合。
//
// 修法前 summary 是對「被 limit 截斷的那批」加總，所以預設的「30 天」實際只涵蓋
// 約 9 個交易日，而 alias_hit_rate 的分母由 limit 決定——比率被無聲截斷。
func TestIdentityStatsSummaryIsNotTruncatedByLimit(t *testing.T) {
	repo := &identityStatsRepoStub{
		rows: make([]store.SRIdentityStats, 30),
		// 區間內其實有 500 次分析，遠多於 rows 能帶回的 2 列。
		agg: store.SRIdentityStatsAggregate{
			Analyses: 500, MatchedByChain: 700, MatchedByCurrent: 200, MatchedByAlias: 100,
		},
	}
	w := getIdentityStats(t, newStatsHandler(repo), "?limit=2")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		Rows    []store.SRIdentityStats `json:"rows"`
		Summary identityStatsSummary    `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Rows) != 2 {
		t.Fatalf("rows 應被 limit 截成 2 列，得到 %d", len(body.Rows))
	}
	if body.Summary.Analyses != 500 {
		t.Errorf("summary 應涵蓋整個區間（500），得到 %d——limit 不該決定分母", body.Summary.Analyses)
	}
	// 分母是 1000（區間全部命中）而不是 rows 那 2 列。
	if body.Summary.MatchedTotal != 1000 || body.Summary.AliasHitRate != 0.1 {
		t.Errorf("matched_total/alias_hit_rate = %d/%v, want 1000/0.1",
			body.Summary.MatchedTotal, body.Summary.AliasHitRate)
	}
}

// Summarize 是這次新增的**第二個 DB 失敗點**，失敗一律 500。
//
// ⛔ 不准降級成「List 成功就先回 rows、summary 給零值」：零值 summary 與
// 「區間內真的沒資料」在 response 上長得一模一樣，呼叫端無從分辨——那正是這次要修掉的
// 那類「不會報錯的失真」。
func TestIdentityStatsSummarizeFailureReturns500(t *testing.T) {
	repo := &identityStatsRepoStub{
		rows:         make([]store.SRIdentityStats, 3),
		summarizeErr: errors.New("aggregate boom"),
	}
	w := getIdentityStats(t, newStatsHandler(repo), "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "rows") {
		t.Errorf("聚合失敗時不可回傳部分結果：%s", w.Body.String())
	}
}

// List 失敗的既有行為不變（回歸）。
func TestIdentityStatsListFailureReturns500(t *testing.T) {
	repo := &identityStatsRepoStub{listErr: errors.New("list boom")}
	if w := getIdentityStats(t, newStatsHandler(repo), ""); w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
