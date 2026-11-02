package market

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/store"
)

// timeoutErr 模擬 http.Client 逾時時 url.Error 包住的錯誤（實作 Timeout()）。
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "Client.Timeout exceeded while awaiting headers" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// dialErr 組出 net/http 實際回傳的形狀：url.Error → OpError(dial) → 內層錯誤。
func dialErr(inner error) error {
	return &url.Error{Op: "Get", URL: "https://example.invalid/x", Err: &net.OpError{Op: "dial", Net: "tcp", Err: inner}}
}

func TestIsTransientClassifiesByType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		// I-122 的 DNS 逾時：`dial tcp: lookup … on 127.0.0.11:53: … i/o timeout`
		{"DNS 逾時", fmt.Errorf("twse isin fetch x: %w", dialErr(&net.DNSError{Err: "i/o timeout", Name: "isin.twse.com.tw", IsTimeout: true})), true},
		{"DNS 暫時性失敗", dialErr(&net.DNSError{Err: "server misbehaving", IsTemporary: true}), true},
		{"DNS NXDOMAIN ⛔ 不算", dialErr(&net.DNSError{Err: "no such host", IsNotFound: true}), false},
		// I-122 的 IPv6 不可達：`dial tcp [2001:…]:443: connect: network is unreachable`
		{"網路不可達", dialErr(os.NewSyscallError("connect", syscall.ENETUNREACH)), true},
		{"連線被拒", dialErr(os.NewSyscallError("connect", syscall.ECONNREFUSED)), true},
		{"連線被重設", &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ECONNRESET)}, true},
		{"HTTP client 逾時", &url.Error{Op: "Get", URL: "https://example.invalid/x", Err: timeoutErr{}}, true},
		{"連線中途斷掉", fmt.Errorf("recompute: %w", io.ErrUnexpectedEOF), true},
		{"HTTP 503", newHTTPStatusError(503, "x: HTTP %d", 503), true},
		{"HTTP 500", newHTTPStatusError(500, "x: HTTP %d", 500), true},
		{"HTTP 404 ⛔ 不算", newHTTPStatusError(404, "x: HTTP %d", 404), false},
		{"HTTP 429 ⛔ 不算", newHTTPStatusError(429, "x: HTTP %d", 429), false},
		{"包在外層的 5xx", fmt.Errorf("finmind fetch failed after %d retries: %w", 3, newHTTPStatusError(503, "y")), true},
		// I-124 的 recovery mode：`FATAL: the database system is in recovery mode (SQLSTATE 57P03)`
		{"SQLSTATE 57P03", fmt.Errorf("failed to connect: %w", &pgconn.PgError{Code: "57P03"}), true},
		{"SQLSTATE 08006", &pgconn.PgError{Code: "08006"}, true},
		{"SQLSTATE 53300", &pgconn.PgError{Code: "53300"}, true},
		{"SQLSTATE 23505 ⛔ 不算", &pgconn.PgError{Code: "23505"}, false},
		// job 自己的預算用完：呼叫端要先看自己的 ctx，這裡⛔ 不認
		{"context 逾時 ⛔ 不算", context.DeadlineExceeded, false},
		{"context 取消 ⛔ 不算", context.Canceled, false},
		{"解析錯誤 ⛔ 不算", errors.New("twse isin: parse error"), false},
		{"主檔快照是空的 ⛔ 不算", store.ErrEmptyStockSymbolSnapshot, false},
		{"前置條件不成立 ⛔ 不算", ErrStockSymbolSyncNotReady, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransient(tt.err); got != tt.want {
				t.Fatalf("IsTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// statusServer 回指定狀態碼與本文。
func statusServer(t *testing.T, code int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestHTTPStatusErrorKeepsOriginalText：五個 client 改用 HTTPStatusError 之後，
// **錯誤文字與改動前逐字相同**（log 與 joberr.Classify 都依賴它），而且 5xx 判得出暫時性、4xx 判不出。
func TestHTTPStatusErrorKeepsOriginalText(t *testing.T) {
	ctx := context.Background()
	for _, code := range []int{503, 404} {
		code := code
		t.Run(fmt.Sprintf("twse isin %d", code), func(t *testing.T) {
			srv := statusServer(t, code, "maintenance")
			c := NewTWSEISINClient([]string{srv.URL}, TWSEISINClientOptions{}, zap.NewNop())
			_, err := c.FetchStockSymbols(ctx)
			want := fmt.Sprintf("twse isin fetch %s: twse isin: unexpected status %d", srv.URL, code)
			assertStatusErr(t, err, want, code)
		})
		t.Run(fmt.Sprintf("twse suspend listing %d", code), func(t *testing.T) {
			srv := statusServer(t, code, "maintenance")
			c := NewTWSESuspendListingClient(srv.URL, 5*time.Second, zap.NewNop())
			_, err := c.FetchDelistings(ctx)
			assertStatusErr(t, err, fmt.Sprintf("twse suspend listing: unexpected status %d", code), code)
		})
		t.Run(fmt.Sprintf("yahoo quote %d", code), func(t *testing.T) {
			srv := statusServer(t, code, "maintenance")
			c := NewYahooQuoteClient(config.YahooConfig{
				BaseURL: srv.URL + "/FinanceChartService.ApacLibraCharts", RateLimit: 10000, BatchSize: 40,
			})
			_, err := c.FetchIntradayCandlesBatch(ctx, []string{"2330"})
			assertStatusErr(t, err, fmt.Sprintf("yahoo http error: status=%d msg=maintenance", code), code)
		})
		t.Run(fmt.Sprintf("yahoo dividends %d", code), func(t *testing.T) {
			srv := statusServer(t, code, "maintenance")
			c := NewYahooDividendClient(600, zap.NewNop())
			c.baseURLForTest = srv.URL
			_, err := c.FetchDividends(ctx, "2330")
			assertStatusErr(t, err, fmt.Sprintf("yahoo dividends 2330: HTTP %d", code), code)
		})
	}
}

// TestFinMind5xxStaysTransientAfterRetries：FinMind 的請求層重試⛔ 不變（退避 1／2／4 秒，
// 這個測試因此要等約 7 秒），用完之後外層錯誤仍判得出 5xx；429 照舊重試、但⛔ 不算暫時性。
func TestFinMind5xxStaysTransientAfterRetries(t *testing.T) {
	for _, tc := range []struct {
		code int
		want bool
	}{{503, true}, {429, false}} {
		tc := tc
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			t.Parallel()
			srv := statusServer(t, tc.code, "busy")
			c := NewFinMindClient(config.FinMindConfig{APIKey: "test", BaseURL: srv.URL, RateLimit: 6000})
			_, err := c.FetchCapitalReductions(context.Background(), "2330")
			want := fmt.Sprintf("finmind fetch failed after %d retries: finmind http error: status=%d body=busy", fetchMaxRetries, tc.code)
			if err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
			if got := IsTransient(err); got != tc.want {
				t.Fatalf("IsTransient = %v, want %v", got, tc.want)
			}
		})
	}
}

func assertStatusErr(t *testing.T, err error, want string, code int) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.Code != code {
		t.Fatalf("err 應帶 HTTPStatusError{Code: %d}，得到 %#v", code, err)
	}
	if got, want := IsTransient(err), code >= 500; got != want {
		t.Fatalf("IsTransient = %v, want %v", got, want)
	}
}
