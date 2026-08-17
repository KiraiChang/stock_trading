package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// universeRepoStub 只記錄收到什麼——這幾支驗的是 handler 的驗證與翻譯，
// repo 的查詢/寫入行為由 internal/store 對真實 sqlite 驗。
type universeRepoStub struct {
	gotUpsert    []store.EvaluationUniverseEntry
	gotSetActive [2]string // symbol, "true"/"false"
	setActiveOK  bool
	listActive   bool
	rows         []store.EvaluationUniverseEntry
}

func (s *universeRepoStub) ListActive(context.Context) ([]store.EvaluationUniverseEntry, error) {
	s.listActive = true
	return s.rows, nil
}
func (s *universeRepoStub) List(context.Context) ([]store.EvaluationUniverseEntry, error) {
	return s.rows, nil
}
func (s *universeRepoStub) Upsert(_ context.Context, e []store.EvaluationUniverseEntry) error {
	s.gotUpsert = e
	return nil
}
func (s *universeRepoStub) SetActive(_ context.Context, symbol string, active bool) (bool, error) {
	s.gotSetActive = [2]string{symbol, map[bool]string{true: "true", false: "false"}[active]}
	return s.setActiveOK, nil
}

func newUniverseHandlerTest(stub *universeRepoStub) (*gin.Engine, *EvaluationUniverseHandler) {
	gin.SetMode(gin.TestMode)
	h := NewEvaluationUniverseHandler(stub, zap.NewNop())
	r := gin.New()
	r.GET("/evaluation-universe", h.List)
	r.POST("/evaluation-universe", h.Upsert)
	r.PATCH("/evaluation-universe/:symbol", h.SetActive)
	return r, h
}

func validUpsertBody(overrides map[string]any) *bytes.Buffer {
	item := map[string]any{
		"symbol":           "2330",
		"bucket_hint":      "LOW_VOLATILITY",
		"bucket_edge_low":  0.046089927430152715,
		"bucket_edge_high": 0.06278197721225691,
		"universe_version": "v2",
		"source":           "T-040_STEP3",
	}
	for k, v := range overrides {
		if v == nil {
			delete(item, k)
			continue
		}
		item[k] = v
	}
	b, _ := json.Marshal(map[string]any{"items": []any{item}})
	return bytes.NewBuffer(b)
}

func doReq(r *gin.Engine, method, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

func TestUniverseUpsertServerAssignsSelectedAt(t *testing.T) {
	stub := &universeRepoStub{}
	r, _ := newUniverseHandlerTest(stub)

	if w := doReq(r, "POST", "/evaluation-universe", validUpsertBody(nil)); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(stub.gotUpsert) != 1 {
		t.Fatalf("repo 收到 %d 筆", len(stub.gotUpsert))
	}
	// selected_at 由伺服器指定：讓呼叫端送會讓「何時入池」變成可偽造的研究紀錄
	if stub.gotUpsert[0].SelectedAt.IsZero() {
		t.Fatal("selected_at 應由伺服器填上")
	}
}

func TestUniverseUpsertRejectsDuplicateSymbols(t *testing.T) {
	stub := &universeRepoStub{}
	r, _ := newUniverseHandlerTest(stub)

	item := map[string]any{
		"symbol": "2330", "bucket_hint": "LOW_VOLATILITY",
		"bucket_edge_low": 0.04, "bucket_edge_high": 0.06,
		"universe_version": "v2", "source": "T-040_STEP3",
	}
	b, _ := json.Marshal(map[string]any{"items": []any{item, item}})

	w := doReq(r, "POST", "/evaluation-universe", bytes.NewBuffer(b))
	// 同一個 transaction 內重複 symbol 會互相覆蓋，留下哪一筆取決於順序——靜默的資料遺失
	if w.Code != http.StatusBadRequest {
		t.Fatalf("重複 symbol 應回 400，得到 %d", w.Code)
	}
	if stub.gotUpsert != nil {
		t.Fatal("驗證失敗不該呼叫 repo")
	}
}

func TestUniverseUpsertRejectsInvalidEdges(t *testing.T) {
	for name, override := range map[string]map[string]any{
		"low 為 0":     {"bucket_edge_low": 0.0},
		"high <= low": {"bucket_edge_high": 0.01},
		"缺 low":       {"bucket_edge_low": nil},
	} {
		stub := &universeRepoStub{}
		r, _ := newUniverseHandlerTest(stub)
		w := doReq(r, "POST", "/evaluation-universe", validUpsertBody(override))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s：應回 400，得到 %d", name, w.Code)
		}
	}
}

func TestUniverseUpsertRejectsMissingRequiredFields(t *testing.T) {
	for _, field := range []string{"symbol", "bucket_hint", "universe_version", "source"} {
		stub := &universeRepoStub{}
		r, _ := newUniverseHandlerTest(stub)
		w := doReq(r, "POST", "/evaluation-universe", validUpsertBody(map[string]any{field: nil}))
		if w.Code != http.StatusBadRequest {
			t.Errorf("缺 %s 應回 400，得到 %d", field, w.Code)
		}
	}
}

func TestUniverseSetActiveRequiresExplicitField(t *testing.T) {
	stub := &universeRepoStub{setActiveOK: true}
	r, _ := newUniverseHandlerTest(stub)

	// 缺欄位與 false 必須分得開，否則漏帶欄位會被當成「停用」
	w := doReq(r, "PATCH", "/evaluation-universe/2330", bytes.NewBufferString(`{}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 active 應回 400，得到 %d", w.Code)
	}
	if stub.gotSetActive[0] != "" {
		t.Fatal("驗證失敗不該呼叫 repo")
	}

	w = doReq(r, "PATCH", "/evaluation-universe/2330", bytes.NewBufferString(`{"active":false}`))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if stub.gotSetActive != [2]string{"2330", "false"} {
		t.Fatalf("傳給 repo 的值不對：%v", stub.gotSetActive)
	}
}

func TestUniverseSetActiveReturns404WhenMissing(t *testing.T) {
	stub := &universeRepoStub{setActiveOK: false}
	r, _ := newUniverseHandlerTest(stub)
	w := doReq(r, "PATCH", "/evaluation-universe/9999", bytes.NewBufferString(`{"active":true}`))
	if w.Code != http.StatusNotFound {
		t.Fatalf("不在池內應回 404，得到 %d", w.Code)
	}
}

func TestUniverseListSummarisesActiveBuckets(t *testing.T) {
	stub := &universeRepoStub{rows: []store.EvaluationUniverseEntry{
		{Symbol: "0050", BucketHint: "LOW_VOLATILITY", Active: true},
		{Symbol: "2330", BucketHint: "LOW_VOLATILITY", Active: true},
		{Symbol: "2454", BucketHint: "HIGH_VOLATILITY", Active: true},
		{Symbol: "6243", BucketHint: "HIGH_VOLATILITY", Active: false},
	}}
	r, _ := newUniverseHandlerTest(stub)

	w := doReq(r, "GET", "/evaluation-universe", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got struct {
		Total         int            `json:"total"`
		ActiveCount   int            `json:"active_count"`
		ActiveBuckets map[string]int `json:"active_buckets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 4 || got.ActiveCount != 3 {
		t.Fatalf("total=%d active=%d", got.Total, got.ActiveCount)
	}
	// bucket 分佈只算 active——停用的標的不再進 evaluation，算進去會高估樣本量
	if got.ActiveBuckets["HIGH_VOLATILITY"] != 1 || got.ActiveBuckets["LOW_VOLATILITY"] != 2 {
		t.Fatalf("bucket 分佈不對：%v", got.ActiveBuckets)
	}
	if stub.listActive {
		t.Fatal("預設不該只查 active——入退池歷史是研究紀錄")
	}
}

func TestUniverseListUsesActiveQueryWhenAsked(t *testing.T) {
	stub := &universeRepoStub{}
	r, _ := newUniverseHandlerTest(stub)
	if w := doReq(r, "GET", "/evaluation-universe?active=true", nil); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !stub.listActive {
		t.Fatal("active=true 應走 ListActive（對應 idx_evaluation_universe_active）")
	}
}
