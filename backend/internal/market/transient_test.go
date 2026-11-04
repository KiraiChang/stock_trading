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

// transientCauses 是 TransientCause 的完整值域（true＝暫時性）。新增原因碼要同時改這裡與 transient.go 的註解。
var transientCauses = map[string]bool{
	"dns_timeout": true, "dns_temporary": true, "http_5xx": true, "db_connection": true,
	"net_unreachable": true, "host_unreachable": true, "conn_refused": true, "conn_reset": true,
	"net_timeout": true, "net_other": true, "http_timeout": true, "unexpected_eof": true,
	"dns_permanent": false, "http_non_5xx": false, "db_other": false, "not_transient": false, "none": false,
}

func TestIsTransientClassifiesByType(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
		cause string // TransientCause 的原因碼（觀察期的追加修正）
	}{
		{"nil", nil, false, "none"},
		// I-122 的 DNS 逾時：`dial tcp: lookup … on 127.0.0.11:53: … i/o timeout`
		{"DNS 逾時", fmt.Errorf("twse isin fetch x: %w", dialErr(&net.DNSError{Err: "i/o timeout", Name: "isin.twse.com.tw", IsTimeout: true})), true, "dns_timeout"},
		{"DNS 暫時性失敗", dialErr(&net.DNSError{Err: "server misbehaving", IsTemporary: true}), true, "dns_temporary"},
		{"DNS NXDOMAIN ⛔ 不算", dialErr(&net.DNSError{Err: "no such host", IsNotFound: true}), false, "dns_permanent"},
		// I-122 的 IPv6 不可達：`dial tcp [2001:…]:443: connect: network is unreachable`
		{"網路不可達", dialErr(os.NewSyscallError("connect", syscall.ENETUNREACH)), true, "net_unreachable"},
		{"連線被拒", dialErr(os.NewSyscallError("connect", syscall.ECONNREFUSED)), true, "conn_refused"},
		{"連線被重設", &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ECONNRESET)}, true, "conn_reset"},
		{"HTTP client 逾時", &url.Error{Op: "Get", URL: "https://example.invalid/x", Err: timeoutErr{}}, true, "http_timeout"},
		{"連線中途斷掉", fmt.Errorf("recompute: %w", io.ErrUnexpectedEOF), true, "unexpected_eof"},
		{"HTTP 503", newHTTPStatusError(503, "x: HTTP %d", 503), true, "http_5xx"},
		{"HTTP 500", newHTTPStatusError(500, "x: HTTP %d", 500), true, "http_5xx"},
		{"HTTP 404 ⛔ 不算", newHTTPStatusError(404, "x: HTTP %d", 404), false, "http_non_5xx"},
		{"HTTP 429 ⛔ 不算", newHTTPStatusError(429, "x: HTTP %d", 429), false, "http_non_5xx"},
		{"包在外層的 5xx", fmt.Errorf("finmind fetch failed after %d retries: %w", 3, newHTTPStatusError(503, "y")), true, "http_5xx"},
		// I-124 的 recovery mode：`FATAL: the database system is in recovery mode (SQLSTATE 57P03)`
		{"SQLSTATE 57P03", fmt.Errorf("failed to connect: %w", &pgconn.PgError{Code: "57P03"}), true, "db_connection"},
		{"SQLSTATE 08006", &pgconn.PgError{Code: "08006"}, true, "db_connection"},
		{"SQLSTATE 53300", &pgconn.PgError{Code: "53300"}, true, "db_connection"},
		{"SQLSTATE 23505 ⛔ 不算", &pgconn.PgError{Code: "23505"}, false, "db_other"},
		// job 自己的預算用完：呼叫端要先看自己的 ctx，這裡⛔ 不認
		{"context 逾時 ⛔ 不算", context.DeadlineExceeded, false, "not_transient"},
		{"context 取消 ⛔ 不算", context.Canceled, false, "not_transient"},
		{"解析錯誤 ⛔ 不算", errors.New("twse isin: parse error"), false, "not_transient"},
		{"主檔快照是空的 ⛔ 不算", store.ErrEmptyStockSymbolSnapshot, false, "not_transient"},
		{"前置條件不成立 ⛔ 不算", ErrStockSymbolSyncNotReady, false, "not_transient"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransient(tt.err); got != tt.want {
				t.Fatalf("IsTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
			if got := TransientCause(tt.err); got != tt.cause {
				t.Fatalf("TransientCause(%v) = %q, want %q", tt.err, got, tt.cause)
			}
			// 原因碼與重試決策出自同一個判定：暫時性的原因碼⛔ 不得出現在非暫時性的錯誤上，反之亦然。
			if transientCauses[tt.cause] != tt.want {
				t.Fatalf("原因碼 %q 的暫時性 = %v，與 IsTransient = %v 不一致", tt.cause, transientCauses[tt.cause], tt.want)
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

// deadlineDividendSource 等到 ctx 結束，再回 http client 被取消時的錯誤（url.Error 包住 ctx 的錯誤，帶 Timeout()）。
type deadlineDividendSource struct{}

func (deadlineDividendSource) FetchDividends(ctx context.Context, _ string) ([]store.CorporateAction, error) {
	<-ctx.Done()
	return nil, &url.Error{Op: "Get", URL: "https://tw.stock.yahoo.com/x", Err: ctx.Err()}
}

type transientDividendSource struct{}

func (transientDividendSource) FetchDividends(context.Context, string) ([]store.CorporateAction, error) {
	return nil, dialErr(&net.DNSError{Err: "i/o timeout", Name: "tw.stock.yahoo.com", IsTimeout: true})
}

// SyncFailure.Transient：失敗的當下整輪預算已用完，就⛔ 不算暫時性——http client 被取消時回的 url.Error
// 也帶 Timeout()，只看錯誤會讓公司行動的第二輪去重試「其實是預算用完」的標的。對照組：預算內的 DNS 逾時算暫時性。
func TestSyncFailureTransientExcludesOwnDeadline(t *testing.T) {
	a := NewAdjuster(nil, nil, nil, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	a.SetDividendSource(deadlineDividendSource{})
	_, _, failures, _ := a.SyncPerSymbolEvents(ctx, []string{"2330"})
	if len(failures) == 0 {
		t.Fatal("應記下 2330 的失敗")
	}
	for _, f := range failures {
		if f.Transient {
			t.Fatalf("預算用完之後的失敗⛔ 不算暫時性，得到 %+v", f)
		}
	}

	a.SetDividendSource(transientDividendSource{})
	_, _, failures, _ = a.SyncPerSymbolEvents(context.Background(), []string{"2330"})
	if len(failures) != 1 || !failures[0].Transient {
		t.Fatalf("預算內的 DNS 逾時應算暫時性，得到 %+v", failures)
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

// TransientCause 細分連線層錯誤；任何 OpError 仍然都算暫時性（細分只影響原因碼）。
func TestTransientCauseNetOpDetail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		cause string
	}{
		{"主機不可達", dialErr(os.NewSyscallError("connect", syscall.EHOSTUNREACH)), "host_unreachable"},
		{"讀取逾時", &net.OpError{Op: "read", Net: "tcp", Err: timeoutErr{}}, "net_timeout"},
		{"其他連線層錯誤", &net.OpError{Op: "write", Net: "tcp", Err: errors.New("x")}, "net_other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := TransientCause(tc.err); got != tc.cause {
				t.Fatalf("TransientCause = %q, want %q", got, tc.cause)
			}
			if !IsTransient(tc.err) || !transientCauses[tc.cause] {
				t.Fatalf("連線層錯誤必須仍是暫時性")
			}
		})
	}
}

// 原因碼⛔ 不得帶出錯誤原文（只由型別決定，值域封閉）。
func TestTransientCauseNeverLeaksErrorText(t *testing.T) {
	secret := "postgres://user:s3cret@db.internal:5432/x?token=abc"
	for _, err := range []error{
		errors.New(secret),
		fmt.Errorf("wrap: %w", dialErr(os.NewSyscallError("connect", syscall.ENETUNREACH))),
		&url.Error{Op: "Get", URL: secret, Err: timeoutErr{}},
		newHTTPStatusError(502, "upstream %s: HTTP %d", secret, 502),
	} {
		cause := TransientCause(err)
		if _, ok := transientCauses[cause]; !ok {
			t.Fatalf("原因碼 %q 不在封閉值域內", cause)
		}
	}
}
