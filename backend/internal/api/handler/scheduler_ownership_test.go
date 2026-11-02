package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
)

// ── 手動觸發主檔同步／公司行動同步：空閒回 202、已有一輪在跑回 409 ─────────────
// （I-123～I-126 計畫書 D5：cron、手動與自動重跑共用一個執行所有權；contract 見 docs/api-reference.md）

// blockingFirstCall 讓第一次呼叫卡住（先關閉 entered、再等 block），之後的呼叫直接返回。
type blockingFirstCall struct {
	once    sync.Once
	entered chan struct{}
	block   chan struct{}
}

func newBlockingFirstCall() *blockingFirstCall {
	return &blockingFirstCall{entered: make(chan struct{}), block: make(chan struct{})}
}

func (b *blockingFirstCall) wait() {
	b.once.Do(func() {
		close(b.entered)
		<-b.block
	})
}

type blockingSymbolSource struct{ *blockingFirstCall }

// 以失敗收尾：⛔ 不碰測試裡是 nil 的主檔 repo。
func (s blockingSymbolSource) FetchStockSymbols(context.Context) ([]store.StockSymbol, error) {
	s.wait()
	return nil, errors.New("test source: no data")
}

type blockingSplitSource struct{ *blockingFirstCall }

func (s blockingSplitSource) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	s.wait()
	return nil, errors.New("test split source: no data")
}

func newOwnershipScheduler(syncer *market.StockSymbolSyncer) *scheduler.Scheduler {
	return scheduler.New(
		nil, nil, nil, &jobRunRepoStub{}, nil, nil, nil, "", syncer, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
}

func assertConflictWhileRunning(
	t *testing.T, path string, handle gin.HandlerFunc, gate *blockingFirstCall, wantErr string,
) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST(path, handle)
	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		return w
	}

	if w := post(); w.Code != http.StatusAccepted {
		t.Fatalf("空閒時應回 202，got %d body=%s", w.Code, w.Body.String())
	}
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("202 之後背景那一輪沒有真的開始")
	}

	w := post()
	if w.Code != http.StatusConflict {
		t.Fatalf("已有一輪在跑時應回 409，got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error != wantErr {
		t.Fatalf("409 body = %s（err=%v），期望 error=%q", w.Body.String(), err, wantErr)
	}

	// 釋放之後要能再次接受請求。
	close(gate.block)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w := post(); w.Code == http.StatusAccepted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("釋放後仍然拿不到 202")
}

func TestRunStockSymbolSyncConflictsWhileRunning(t *testing.T) {
	gate := newBlockingFirstCall()
	s := newOwnershipScheduler(market.NewStockSymbolSyncer(blockingSymbolSource{gate}, nil, zap.NewNop()))
	h := NewSchedulerHandler(&jobRunRepoStub{}, s, zap.NewNop())

	assertConflictWhileRunning(t, "/scheduler/stock-symbol-sync/run", h.RunStockSymbolSync, gate,
		"stock symbol sync already running")
}

func TestRunCorporateActionSyncConflictsWhileRunning(t *testing.T) {
	gate := newBlockingFirstCall()
	s := newOwnershipScheduler(nil)
	s.SetAdjuster(market.NewAdjuster(blockingSplitSource{gate}, nil, nil, zap.NewNop()), config.CorporateActionConfig{})
	h := NewSchedulerHandler(&jobRunRepoStub{}, s, zap.NewNop())

	assertConflictWhileRunning(t, "/scheduler/corporate-action-sync/run", h.RunCorporateActionSync, gate,
		"corporate action sync already running")
}
