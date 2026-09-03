package joberr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/trading/backend/internal/indicator"
)

// sensitive 模擬 driver 錯誤裡會出現的連線細節。**它絕不能出現在任何輸出裡。**
const sensitive = "postgres://trading_user:s3cr3t@db.internal:5432/trading"

func TestClassifyKnownCauses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Reason
	}{
		{"資料不足走 typed error", fmt.Errorf("%w: got 12", indicator.ErrInsufficientCandles), InsufficientData},
		{"逾時 sentinel", context.DeadlineExceeded, Timeout},
		{"取消 sentinel", context.Canceled, Timeout},
		{"型別溢位", errors.New("pq: numeric field overflow"), NumericOverflow},
		{"欄位過長", errors.New("pq: value too long for type character varying(10)"), NumericOverflow},
		{"唯一鍵", errors.New(`pq: duplicate key value violates unique constraint "x"`), ConstraintViolation},
		{"連不上", errors.New("dial tcp: connection refused"), ConnRefused},
		{"READONLY", errors.New("READONLY You can't write against a read only replica"), Readonly},
		{"查無資料", errors.New("sql: no rows in result set"), NotFound},
		{"上游狀態碼", errors.New("twse request failed: unexpected status 503"), Upstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Errorf("Classify = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyFallsBackToInternal 是安全邊界最關鍵的一條。
func TestClassifyFallsBackToInternal(t *testing.T) {
	err := errors.New("totally unrecognised failure at " + sensitive)
	if got := Classify(err); got != Internal {
		t.Fatalf("未知錯誤要回 internal_error，得到 %q", got)
	}
	if got := Classify(nil); got != Internal {
		t.Errorf("nil 也要回 internal_error，得到 %q", got)
	}
}

// TestNoOutputEverLeaksCause 對三個對外函式一次驗完。
func TestNoOutputEverLeaksCause(t *testing.T) {
	causes := []error{
		errors.New("dial " + sensitive + ": connection refused"),
		fmt.Errorf("wrapped: %w", errors.New(sensitive)),
		errors.New("boom at " + sensitive),
	}
	for i, err := range causes {
		t.Run(fmt.Sprintf("cause%d", i), func(t *testing.T) {
			for name, got := range map[string]string{
				"Classify":   string(Classify(err)),
				"Summary":    Summary("stage", err),
				"SummaryFor": SummaryFor("stage", "2330", err),
				"Describe":   Describe(err),
			} {
				if strings.Contains(got, sensitive) || strings.Contains(got, "s3cr3t") ||
					strings.Contains(got, "db.internal") {
					t.Errorf("%s 外洩了 cause：%q", name, got)
				}
			}
		})
	}
}

func TestSummaryFormats(t *testing.T) {
	err := errors.New("connection refused")
	if got := Summary("watchlist_fetch", err); got != "watchlist_fetch:conn_refused" {
		t.Errorf("Summary = %q", got)
	}
	if got := SummaryFor("chip_sync", "2330", err); got != "chip_sync:2330:conn_refused" {
		t.Errorf("SummaryFor = %q", got)
	}
}

// safeErr 模擬「自己組的、訊息已確認安全」的錯誤。
type safeErr struct{ msg string }

func (e safeErr) Error() string          { return e.msg }
func (e safeErr) SafeJobMessage() string { return e.msg }

// TestDescribeLetsSafeMessagesThrough 守住 SafeMessenger 的用途：
// 分類器是為了擋外來錯誤，**不該把我們自己的結構化訊號也吃掉**。
func TestDescribeLetsSafeMessagesThrough(t *testing.T) {
	msg := "市場層級對照源陳舊: source_as_of=2026-08-20 落後 3 個交易日（門檻 2）"
	if got := Describe(safeErr{msg}); got != msg {
		t.Errorf("SafeMessenger 的訊息應原樣通過，得到 %q", got)
	}
	// 包一層仍要認得（errors.As）。
	if got := Describe(fmt.Errorf("外層: %w", safeErr{msg})); got != msg {
		t.Errorf("包一層後仍應認得，得到 %q", got)
	}
	// 一般錯誤仍走分類器。
	if got := Describe(errors.New("dial " + sensitive)); strings.Contains(got, sensitive) {
		t.Errorf("非 SafeMessenger 不得原樣通過：%q", got)
	}
}
