package market

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// 暫時性錯誤的判定（I-123～I-126 計畫書 docs/plans/i123-i126-job-retry.md「3.1」）。
//
// **只認型別、⛔ 不比對訊息文字**：訊息文字是給人與 log 看的，改一個字就會讓判定靜默失效；
// 型別判定則由編譯器與測試守住。joberr.Classify 走的是另一條路（為了安全的使用者可見字串，
// 退路才比對關鍵字），兩者的用途不同、⛔ 不要合併。

// HTTPStatusError 是上游回非 2xx 時的錯誤。`Error()` 的文字由呼叫端完整給定，
// **必須與改用這個型別之前的字串逐字相同**——log 與 joberr.Classify 的結果都依賴它。
type HTTPStatusError struct {
	Code int
	msg  string
}

func (e *HTTPStatusError) Error() string { return e.msg }

// newHTTPStatusError 以 fmt 格式組出原本的錯誤文字，並附上狀態碼供 IsTransient 判斷。
func newHTTPStatusError(code int, format string, args ...any) error {
	return &HTTPStatusError{Code: code, msg: fmt.Sprintf(format, args...)}
}

// IsTransient 回答「這個錯誤值不值得過一陣子再試」。
//
// 判定為暫時性：DNS 查詢逾時或暫時性失敗、連線層錯誤（拒絕、重設、網路不可達、讀寫逾時）、
// HTTP client 逾時、連線中途斷掉、上游 5xx、DB 連線類錯誤（SQLSTATE class 08、57P01～57P03、53300）。
//
// ⛔ **不算**：4xx（含 404、429——429 是限流，縮短間隔重試只會更糟）、DNS 的 NXDOMAIN、解析錯誤、
// 以及任何沒列在上面的錯誤。
//
// ⚠️ **呼叫端要先檢查自己的 ctx**：job 自己的預算用完時，HTTP 請求回的也是逾時，
// 這裡分不出「上游慢」與「我們的預算到了」，⛔ 不能把後者當成暫時性。
func IsTransient(err error) bool {
	_, transient := classifyTransient(err)
	return transient
}

// TransientCause 回傳錯誤的**安全原因碼**，供 log 事後追溯「重試救回的是哪一類錯誤」
// （I-123～I-126 計畫書「十二」的「觀察期的追加修正」）。
//
// 值域是封閉的（只由型別決定、⛔ 不含任何錯誤原文，可以放心寫進 log）：
//
//   - 暫時性：`dns_timeout`、`dns_temporary`、`http_5xx`、`db_connection`、`net_unreachable`、
//     `host_unreachable`、`conn_refused`、`conn_reset`、`net_timeout`、`net_other`、
//     `http_timeout`、`unexpected_eof`
//   - 非暫時性：`dns_permanent`、`http_non_5xx`、`db_other`、`not_transient`
//   - `err == nil`：`none`
//
// ⚠️ 和 IsTransient 出自同一個判定（classifyTransient），⛔ 不另寫一份——兩邊各自判斷時，
// 改了一邊而另一邊沒跟上，log 會寫出和實際重試決策不一致的原因碼。
func TransientCause(err error) string {
	cause, _ := classifyTransient(err)
	return cause
}

// classifyTransient 是 IsTransient 與 TransientCause 共用的唯一判定。
func classifyTransient(err error) (cause string, transient bool) {
	if err == nil {
		return "none", false
	}
	// DNS 要排在 OpError 之前：查詢失敗會包在 `dial` 的 OpError 裡，NXDOMAIN 也一樣，
	// 先看 DNSError 才能把它排除。
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		switch {
		case dnsErr.IsTimeout:
			return "dns_timeout", true
		case dnsErr.IsTemporary:
			return "dns_temporary", true
		}
		return "dns_permanent", false
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		if statusErr.Code >= 500 {
			return "http_5xx", true
		}
		return "http_non_5xx", false
	}
	// pgconn 的 PgError 有 SQLState()；用介面比對，⛔ 不必讓 market 套件 import pgx。
	var sqlErr interface{ SQLState() string }
	if errors.As(err, &sqlErr) {
		if transientSQLState(sqlErr.SQLState()) {
			return "db_connection", true
		}
		return "db_other", false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return netOpCause(opErr), true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return "http_timeout", true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return "unexpected_eof", true
	}
	return "not_transient", false
}

// netOpCause 細分連線層錯誤。⚠️ 只影響原因碼：任何 OpError 都算暫時性（與細分之前相同）。
func netOpCause(opErr *net.OpError) string {
	switch {
	case errors.Is(opErr, syscall.ENETUNREACH):
		return "net_unreachable" // I-122 的 IPv6 不可達
	case errors.Is(opErr, syscall.EHOSTUNREACH):
		return "host_unreachable"
	case errors.Is(opErr, syscall.ECONNREFUSED):
		return "conn_refused"
	case errors.Is(opErr, syscall.ECONNRESET):
		return "conn_reset"
	case opErr.Timeout():
		return "net_timeout"
	}
	return "net_other"
}

// transientSQLState：connection exception（class 08）、admin／crash shutdown、
// cannot connect now（recovery mode，I-124 的 57P03）、too many connections。
func transientSQLState(code string) bool {
	switch code {
	case "57P01", "57P02", "57P03", "53300":
		return true
	}
	return strings.HasPrefix(code, "08")
}
