package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// 這一組守的是 docs/api-reference.md 兩條端點的三分支錯誤契約。
//
// **兩條端點（/indicators/:symbol/compute 與 /signals/:symbol/evaluate）共用
// indicatorComputeError**，所以測分流本身就等於同時測兩條；另外各補一支
// 端到端測試確認它們真的接上了（見 handler 各自的測試檔）。

// sensitiveMarker 模擬 driver 錯誤裡會出現的連線細節。
// **它絕對不能出現在任何回應本文裡**——job_runs.error 與 API 都是使用者可見面。
const sensitiveMarker = "postgres://trading_user:s3cr3t@db.internal:5432/trading"

func runComputeError(t *testing.T, err error) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/indicators/2454/compute", nil)

	indicatorComputeError(c, zap.NewNop(), err, "test")
	return w.Code, w.Body.String()
}

func TestIndicatorComputeErrorMapsThreeBranches(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "persistence 失敗 → 503",
			err:      fmt.Errorf("%w: %s", indicator.ErrPersistence, sensitiveMarker),
			wantCode: http.StatusServiceUnavailable,
		},
		{
			name:     "資料不足 → 422",
			err:      fmt.Errorf("%w for 2454/1m: got 12, need 35", indicator.ErrInsufficientCandles),
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			// ⛔ 這一格是重點：前一版的規則是「其餘一律 422」，
			// 那會把 DB 讀取失敗謊報成「你的輸入有問題」。
			name:     "DB 讀取失敗 → 5xx，不是 422",
			err:      fmt.Errorf("load candles for 2454/1m: dial %s: connection refused", sensitiveMarker),
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "未知錯誤 → 5xx",
			err:      errors.New("something unexpected happened at " + sensitiveMarker),
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := runComputeError(t, tt.err)
			if code != tt.wantCode {
				t.Errorf("狀態碼 = %d, want %d（body=%s）", code, tt.wantCode, body)
			}
		})
	}
}

// TestIndicatorComputeErrorNeverLeaksCause 是安全邊界的斷言。
//
// 兩個 handler 原本都是 `c.JSON(422, gin.H{"error": err.Error()})`——
// 只把狀態碼改成 503、保留 err.Error() 的話，DSN 照樣從 API 洩出去。
func TestIndicatorComputeErrorNeverLeaksCause(t *testing.T) {
	causes := map[string]error{
		"persistence": fmt.Errorf("%w: pq: numeric field overflow (%s)", indicator.ErrPersistence, sensitiveMarker),
		"db_read":     fmt.Errorf("load candles: dial %s: connection refused", sensitiveMarker),
		"unknown":     errors.New("boom at " + sensitiveMarker),
	}
	for name, err := range causes {
		t.Run(name, func(t *testing.T) {
			_, body := runComputeError(t, err)
			if strings.Contains(body, sensitiveMarker) {
				t.Errorf("回應外洩了 cause：%s", body)
			}
			if strings.Contains(body, "s3cr3t") || strings.Contains(body, "db.internal") {
				t.Errorf("回應含連線細節：%s", body)
			}
		})
	}
}

// TestInsufficientCandlesMessageIsUserFacing 確認 422 那格仍給得出可行動的訊息。
//
// 前端 indicators.ts 的註解明寫「不足時後端回 422，讓呼叫端 catch 顯示錯誤」，
// 所以這一格不能像 5xx 那樣只回通用字串。
func TestInsufficientCandlesMessageIsUserFacing(t *testing.T) {
	_, body := runComputeError(t, fmt.Errorf("%w: got 12", indicator.ErrInsufficientCandles))
	if !strings.Contains(body, "資料不足") {
		t.Errorf("422 要說明是資料不足，得到 %s", body)
	}
}

// ── 端到端：兩條 endpoint 真的接上了分流 ─────────────────────────
//
// 上面的測試驗的是 indicatorComputeError 本身；這兩支確認 handler 真的呼叫它，
// 而不是還留著 `c.JSON(422, gin.H{"error": err.Error()})`。

// failingCandleRepo 讓 indicator.Compute 走到「DB 讀取失敗」那一格。
type failingCandleRepo struct {
	store.CandleRepo
	err error
}

func (f *failingCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	return nil, f.err
}

// shortCandleRepo 讓 indicator.Compute 走到「資料不足」那一格。
type shortCandleRepo struct {
	store.CandleRepo
}

func (s *shortCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	return []store.Candle{{Symbol: "2454", Close: 100}}, nil
}

func newIndicatorEngine(candles store.CandleRepo) *indicator.Engine {
	return indicator.NewEngine(candles, nil, &store.RedisClient{}, zap.NewNop())
}

func doRequest(t *testing.T, h gin.HandlerFunc, method, path string, params gin.Params) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Params = params
	h(c)
	return w.Code, w.Body.String()
}

func TestIndicatorEndpointUsesSharedErrorMapping(t *testing.T) {
	t.Run("DB 讀取失敗 → 5xx 且不外洩", func(t *testing.T) {
		repo := &failingCandleRepo{err: errors.New("dial " + sensitiveMarker + ": connection refused")}
		h := NewIndicatorHandler(newIndicatorEngine(repo), nil, zap.NewNop())
		code, body := doRequest(t, h.Compute, http.MethodPost, "/indicators/2454/compute",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusInternalServerError {
			t.Errorf("狀態碼 = %d, want 500（body=%s）", code, body)
		}
		if strings.Contains(body, sensitiveMarker) {
			t.Errorf("端點外洩了 cause：%s", body)
		}
	})

	t.Run("資料不足 → 422", func(t *testing.T) {
		h := NewIndicatorHandler(newIndicatorEngine(&shortCandleRepo{}), nil, zap.NewNop())
		code, body := doRequest(t, h.Compute, http.MethodPost, "/indicators/2454/compute",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusUnprocessableEntity {
			t.Errorf("狀態碼 = %d, want 422（body=%s）", code, body)
		}
	})
}

func TestSignalEndpointUsesSharedErrorMapping(t *testing.T) {
	// signal.Evaluate 的第一行就是 indicator.Compute，所以 indicator 的錯誤
	// 會原樣從這條 API 傳出去——兩邊的狀態碼必須一致。
	t.Run("DB 讀取失敗 → 5xx 且不外洩", func(t *testing.T) {
		repo := &failingCandleRepo{err: errors.New("dial " + sensitiveMarker + ": connection refused")}
		eng := signal.NewEngine(repo, nil, &store.RedisClient{}, newIndicatorEngine(repo), nil, zap.NewNop())
		h := NewSignalHandler(eng, nil, zap.NewNop())
		code, body := doRequest(t, h.Evaluate, http.MethodPost, "/signals/2454/evaluate",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusInternalServerError {
			t.Errorf("狀態碼 = %d, want 500（body=%s）", code, body)
		}
		if strings.Contains(body, sensitiveMarker) {
			t.Errorf("端點外洩了 cause：%s", body)
		}
	})

	t.Run("資料不足 → 422（與 indicator 端點一致）", func(t *testing.T) {
		repo := &shortCandleRepo{}
		eng := signal.NewEngine(repo, nil, &store.RedisClient{}, newIndicatorEngine(repo), nil, zap.NewNop())
		h := NewSignalHandler(eng, nil, zap.NewNop())
		code, body := doRequest(t, h.Evaluate, http.MethodPost, "/signals/2454/evaluate",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusUnprocessableEntity {
			t.Errorf("狀態碼 = %d, want 422（body=%s）", code, body)
		}
	})
}

// ── persistence → 503：兩條端點各補一格，補齊「每條 endpoint 三分支」 ──

// okCandleRepo 提供足量平盤 K 棒，讓 Compute 走到 Upsert 那一步。
type okCandleRepo struct{ store.CandleRepo }

func (okCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	base := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	out := make([]store.Candle, 0, 120)
	for i := 0; i < 120; i++ {
		out = append(out, store.Candle{
			Symbol: "2454", Timeframe: "1d",
			Open: 100, High: 100, Low: 100, Close: 100,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
		})
	}
	return out, nil
}

// failingIndicatorRepo 讓 Upsert 失敗，走 persistence 那一格。
type failingIndicatorRepo struct {
	store.IndicatorRepo
	err error
}

func (f *failingIndicatorRepo) Upsert(context.Context, *store.IndicatorSnapshot) error {
	return f.err
}

func newPersistFailingEngine(cause error) *indicator.Engine {
	return indicator.NewEngine(okCandleRepo{}, &failingIndicatorRepo{err: cause},
		&store.RedisClient{}, zap.NewNop())
}

func TestBothEndpointsMapPersistenceTo503WithoutLeaking(t *testing.T) {
	cause := errors.New("pq: numeric field overflow (" + sensitiveMarker + ")")

	t.Run("indicator endpoint", func(t *testing.T) {
		h := NewIndicatorHandler(newPersistFailingEngine(cause), nil, zap.NewNop())
		code, body := doRequest(t, h.Compute, http.MethodPost, "/indicators/2454/compute",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusServiceUnavailable {
			t.Errorf("狀態碼 = %d, want 503（body=%s）", code, body)
		}
		if strings.Contains(body, sensitiveMarker) || strings.Contains(body, "s3cr3t") {
			t.Errorf("端點外洩了 persistence cause：%s", body)
		}
	})

	t.Run("signal endpoint", func(t *testing.T) {
		ind := newPersistFailingEngine(cause)
		eng := signal.NewEngine(okCandleRepo{}, nil, &store.RedisClient{}, ind, nil, zap.NewNop())
		h := NewSignalHandler(eng, nil, zap.NewNop())
		code, body := doRequest(t, h.Evaluate, http.MethodPost, "/signals/2454/evaluate",
			gin.Params{{Key: "symbol", Value: "2454"}})

		if code != http.StatusServiceUnavailable {
			t.Errorf("狀態碼 = %d, want 503（body=%s）", code, body)
		}
		if strings.Contains(body, sensitiveMarker) || strings.Contains(body, "s3cr3t") {
			t.Errorf("端點外洩了 persistence cause：%s", body)
		}
	})
}

// TestSignalEndpointAlwaysReturnsResultFields 釘住 Signal API 的回傳契約。
//
// **沒有訊號時也要回完整狀態**——只回「沒有觸發訊號」的話，呼叫端看不到
// 判重是否已經降級成單層。
func TestSignalEndpointAlwaysReturnsResultFields(t *testing.T) {
	// 平盤資料 → 算得出指標但不觸發訊號 → SignalGenerated=false
	ind := indicator.NewEngine(okCandleRepo{}, noopIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	eng := signal.NewEngine(okCandleRepo{}, nil, &store.RedisClient{}, ind, nil, zap.NewNop())
	h := NewSignalHandler(eng, nil, zap.NewNop())

	code, body := doRequest(t, h.Evaluate, http.MethodPost, "/signals/2454/evaluate",
		gin.Params{{Key: "symbol", Value: "2454"}})

	if code != http.StatusOK {
		t.Fatalf("狀態碼 = %d, want 200（body=%s）", code, body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("回應不是 JSON：%v", err)
	}
	for _, key := range []string{
		"signal_generated", "db_persisted", "queue_enqueued", "broadcast_attempted", "degraded",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("回應缺少 %q（沒有訊號時也要回完整狀態）：%s", key, body)
		}
	}
	if got["signal_generated"] != false {
		t.Errorf("signal_generated = %v, want false", got["signal_generated"])
	}
	// degraded_stages **一律是陣列**，沒有降級時是空的——不得是 null，
	// 那會逼前端在迭代前先做 null 檢查。
	stages, ok := got["degraded_stages"].([]any)
	if !ok {
		t.Errorf("degraded_stages 應為陣列（沒有降級時是空陣列，不是 null）：%s", body)
	} else if len(stages) != 0 {
		t.Errorf("沒有降級時 degraded_stages 應為空，得到 %v", stages)
	}
}

// TestSignalEndpointReturnsDegradedStages 補上契約的另一半：
// **真的降級時 degraded_stages 要帶得出來**，否則呼叫端只知道「有問題」
// 卻不知道是哪一種。
func TestSignalEndpointReturnsDegradedStages(t *testing.T) {
	ind := indicator.NewEngine(bounceCandleRepo{}, noopIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	// signals.Insert 失敗 → degraded-success，訊號照樣送出但標 signal_persist_failed
	eng := signal.NewEngine(bounceCandleRepo{}, &failingSignalRepo{err: errors.New("pq: numeric field overflow")},
		&store.RedisClient{}, ind, noopChipRepo{}, zap.NewNop())
	h := NewSignalHandler(eng, nil, zap.NewNop())

	code, body := doRequest(t, h.Evaluate, http.MethodPost, "/signals/2330/evaluate",
		gin.Params{{Key: "symbol", Value: "2330"}})

	if code != http.StatusOK {
		t.Fatalf("degraded-success 仍應回 200，得到 %d（body=%s）", code, body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("回應不是 JSON：%v", err)
	}
	if got["signal_generated"] != true {
		t.Fatalf("應產生訊號（fixture 設計如此）：%s", body)
	}
	if got["db_persisted"] != false {
		t.Errorf("Insert 失敗時 db_persisted 應為 false：%s", body)
	}
	if got["degraded"] != true {
		t.Errorf("degraded 應為 true：%s", body)
	}
	stages, ok := got["degraded_stages"].([]any)
	if !ok || len(stages) == 0 {
		t.Fatalf("degraded_stages 應帶出降級分類：%s", body)
	}
	if stages[0] != "signal_persist_failed" {
		t.Errorf("degraded_stages = %v, want [signal_persist_failed]", stages)
	}
}

// bounceCandleRepo 提供會觸發 SUPPORT_BOUNCE 的資料，讓 handler 測試走得到
// 「有訊號」那條路。形狀與 signal 套件的 bounceCandles 一致。
type bounceCandleRepo struct{ store.CandleRepo }

func (bounceCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	base := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	out := make([]store.Candle, 0, 120)
	for i := 0; i < 119; i++ {
		low := 100.0
		if i%3 != 0 {
			low = 101.5
		}
		out = append(out, store.Candle{
			Symbol: "2330", Timeframe: "1d",
			Open: 103, High: 105, Low: low, Close: 104,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
		})
	}
	out = append(out, store.Candle{
		Symbol: "2330", Timeframe: "1d",
		Open: 101, High: 105, Low: 100, Close: 104.5,
		Volume: 3000, Timestamp: base.Add(119 * 24 * time.Hour),
	})
	return out, nil
}

type failingSignalRepo struct {
	store.SignalRepo
	err error
}

func (f *failingSignalRepo) Insert(context.Context, *store.Signal) error { return f.err }

// noopChipRepo 讓 applyChipWeighting 走「查無籌碼資料」那條——
// 它本來就設計成缺資料不阻塞訊號產生。
type noopChipRepo struct{ store.ChipScoreRepo }

func (noopChipRepo) GetLatest(context.Context, string) (*store.ChipScore, error) {
	return nil, sql.ErrNoRows
}

func (f *failingSignalRepo) GetBySymbol(context.Context, string, int) ([]store.Signal, error) {
	return nil, nil
}

type noopIndicatorRepo struct{ store.IndicatorRepo }

func (noopIndicatorRepo) Upsert(context.Context, *store.IndicatorSnapshot) error { return nil }
