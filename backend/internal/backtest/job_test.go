package backtest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/store"
)

// 觸發 python-server 的 log 依結果分開（I-134）；不論哪一種都⛔ 不改任務狀態。
// repo 是 nil 介面：triggerHTTP 只要碰到 repo 就會 panic，等於驗證「⛔ 不改任務狀態」。

type noRepo struct{ store.BacktestRepo }

func newTestManager(url string, timeout time.Duration) (*Manager, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.InfoLevel)
	m := NewManager(noRepo{}, url, zap.New(core))
	m.httpTimeout = timeout
	return m, logs
}

func TestTriggerHTTPLogsByOutcome(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel zapcore.Level
		wantMsg   string
	}{
		{"accepted", http.StatusAccepted, zapcore.InfoLevel, "python http trigger ok"},
		{"already claimed", http.StatusConflict, zapcore.InfoLevel, "python http trigger: job already claimed or no longer pending"},
		{"server error", http.StatusInternalServerError, zapcore.WarnLevel, "python http trigger rejected"},
		{"validation error", http.StatusUnprocessableEntity, zapcore.WarnLevel, "python http trigger rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/backtest" || r.Method != http.MethodPost {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			m, logs := newTestManager(srv.URL, 5*time.Second)

			m.triggerHTTP(&store.BacktestJob{JobID: "j1"})

			assertSingleLog(t, logs, tt.wantLevel, tt.wantMsg)
		})
	}
}

func TestTriggerHTTPConnectionFailureIsResultUnknown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // 連不上
	m, logs := newTestManager(url, 5*time.Second)

	m.triggerHTTP(&store.BacktestJob{JobID: "j1"})

	assertResultUnknown(t, logs)
}

func TestTriggerHTTPTimeoutIsResultUnknown(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // 認領之後回應遲遲沒送出
	}))
	defer srv.Close()
	defer close(release)
	m, logs := newTestManager(srv.URL, 50*time.Millisecond)

	m.triggerHTTP(&store.BacktestJob{JobID: "j1"})

	assertResultUnknown(t, logs)
}

func assertResultUnknown(t *testing.T, logs *observer.ObservedLogs) {
	t.Helper()
	assertSingleLog(t, logs, zapcore.WarnLevel,
		"python trigger result unknown: python-server may have claimed the job; worker only picks it up if it is still pending")
}

func assertSingleLog(t *testing.T, logs *observer.ObservedLogs, level zapcore.Level, msg string) {
	t.Helper()
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 log, got %d: %+v", len(entries), entries)
	}
	if entries[0].Level != level || entries[0].Message != msg {
		t.Fatalf("log = %s %q, want %s %q", entries[0].Level, entries[0].Message, level, msg)
	}
	if got := entries[0].ContextMap()["job_id"]; got != "j1" {
		t.Fatalf("job_id field = %v", got)
	}
}
