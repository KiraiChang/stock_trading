package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// stockSymbolRepoStub 只記錄收到的 options——這幾支測試驗的是 handler 怎麼把 query
// 翻譯成 repo 條件，repo 本身的查詢邏輯由 internal/store 的測試對真實 sqlite 驗。
type stockSymbolRepoStub struct {
	gotOpts store.StockSymbolCandidateOptions
	result  store.StockSymbolCandidateResult
}

func (s *stockSymbolRepoStub) UpsertSnapshot(context.Context, []store.StockSymbol, time.Time) (store.StockSymbolSyncResult, error) {
	return store.StockSymbolSyncResult{}, nil
}
func (s *stockSymbolRepoStub) Get(context.Context, string) (*store.StockSymbol, error) {
	return nil, nil
}
func (s *stockSymbolRepoStub) List(context.Context, bool) ([]store.StockSymbol, error) {
	return nil, nil
}
func (s *stockSymbolRepoStub) Search(context.Context, store.StockSymbolSearchOptions) ([]store.StockSymbol, error) {
	return nil, nil
}
func (s *stockSymbolRepoStub) ListCandidates(_ context.Context, opts store.StockSymbolCandidateOptions) (store.StockSymbolCandidateResult, error) {
	s.gotOpts = opts
	return s.result, nil
}

func candidatesRequest(t *testing.T, repo store.StockSymbolRepo, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewStockSymbolHandler(repo, zap.NewNop())
	router := gin.New()
	router.GET("/stock-symbols/candidates", h.Candidates)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stock-symbols/candidates"+query, nil)
	router.ServeHTTP(rec, req)
	return rec
}

// TestCandidatesDefaultSecurityTypes 把 defaultCandidateSecurityTypes 的字面值釘住。
//
// **為什麼需要專門一支測試**：這兩個字串是中文（`股票` / `ETF`），而 stock_symbols 存的是
// TWSE ISIN 的原始分類。寫錯的話後果不是報錯，是**無參數請求靜靜回 0 筆**——
// 而清單為空與「條件真的沒有匹配」在 HTTP 200 底下長得一模一樣。
//
// 另一面向：這個預設值同時是安全閥。實測 43,061 筆上市資料裡 94% 是認購（售）權證，
// 且代號排序在股票之前；預設值失效等於讓一個不帶參數的請求回傳一整批權證，
// 而這份清單被設計成可直接餵給 5 req/min 的 backfill。
func TestCandidatesDefaultSecurityTypes(t *testing.T) {
	repo := &stockSymbolRepoStub{}
	if rec := candidatesRequest(t, repo, ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	got := repo.gotOpts.SecurityTypes
	for _, want := range []string{"股票", "ETF"} {
		if !slices.Contains(got, want) {
			t.Errorf("預設 security_type 缺少 %q，實得 %v——DB 存的是中文分類，"+
				"值不對會讓無參數請求靜靜回 0 筆", want, got)
		}
	}
	if slices.Contains(got, "上市認購(售)權證") {
		t.Error("預設值把權證也帶進來了——那是 94% 的母體且沒有 K 線資料")
	}
}

// TestCandidatesExplicitSecurityTypeOverridesDefault：明確指定時不該再套預設值，
// 否則想單獨查權證或特別股的呼叫端永遠會多拿到股票與 ETF。
func TestCandidatesExplicitSecurityTypeOverridesDefault(t *testing.T) {
	repo := &stockSymbolRepoStub{}
	candidatesRequest(t, repo, "?security_type=ETF")

	if got := repo.gotOpts.SecurityTypes; len(got) != 1 || got[0] != "ETF" {
		t.Errorf("security_type = %v, want [ETF]——明確指定時不該再併入預設值", got)
	}
}

// TestCandidatesZeroMeansUnlimited 鎖住兩個「0 不是錯誤，是不限制」的參數。
// 前端規格寫的是「留空 = 不限」，但數字輸入框被清空或歸零時送出的是 0，
// 若回 400 會讓整個請求失敗；若當成有效條件則會靜靜縮小母體。
func TestCandidatesZeroMeansUnlimited(t *testing.T) {
	repo := &stockSymbolRepoStub{}
	rec := candidatesRequest(t, repo, "?listed_years=0&per_industry=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200——0 應視為不限制而不是參數錯誤", rec.Code)
	}
	if !repo.gotOpts.ListedBefore.IsZero() {
		t.Errorf("listed_years=0 卻設了 ListedBefore=%v——那會連帶啟用 listed_date IS NOT NULL，"+
			"靜靜濾掉上市日解析失敗的標的", repo.gotOpts.ListedBefore)
	}
	if repo.gotOpts.PerIndustryLimit != 0 {
		t.Errorf("per_industry=0 卻設了上限 %d", repo.gotOpts.PerIndustryLimit)
	}
}

func TestCandidatesRejectsNegativeParams(t *testing.T) {
	for _, q := range []string{"?listed_years=-1", "?per_industry=-1", "?limit=-1", "?limit=0"} {
		repo := &stockSymbolRepoStub{}
		if rec := candidatesRequest(t, repo, q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s 的狀態碼 = %d, want 400", q, rec.Code)
		}
	}
}

// TestCandidatesEmptyResultShapes：0 筆時 symbols 與 rows 都要是 []，不能是 null。
// 前端會對兩者做 .map()，形狀不一致的那一個會直接爆掉。
func TestCandidatesEmptyResultShapes(t *testing.T) {
	repo := &stockSymbolRepoStub{result: store.StockSymbolCandidateResult{Symbols: []store.StockSymbol{}}}
	rec := candidatesRequest(t, repo, "?industry=不存在的產業")

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON: %v", err)
	}
	for _, key := range []string{"symbols", "rows"} {
		if string(body[key]) == "null" {
			t.Errorf("%q 是 null，應該是 []——前端對它做 .map() 會爆掉", key)
		}
	}
}

// TestCandidatesPassesTruncated：截斷旗標要原樣傳出去，呼叫端才知道清單不完整。
func TestCandidatesPassesTruncated(t *testing.T) {
	repo := &stockSymbolRepoStub{result: store.StockSymbolCandidateResult{
		Symbols:   []store.StockSymbol{{Symbol: "2330", Industry: "半導體業"}},
		Truncated: true,
	}}
	rec := candidatesRequest(t, repo, "?limit=1")

	var body struct {
		Count      int            `json:"count"`
		Symbols    []string       `json:"symbols"`
		ByIndustry map[string]int `json:"by_industry"`
		Truncated  bool           `json:"truncated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON: %v", err)
	}
	if !body.Truncated {
		t.Error("truncated 沒有傳出去——呼叫端會以為拿到完整清單")
	}
	if body.Count != 1 || len(body.Symbols) != 1 || body.Symbols[0] != "2330" {
		t.Errorf("count/symbols 不符：%+v", body)
	}
	if body.ByIndustry["半導體業"] != 1 {
		t.Errorf("by_industry 不符：%+v", body.ByIndustry)
	}
}
