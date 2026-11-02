package market

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
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
	if err == nil {
		return false
	}
	// DNS 要排在 OpError 之前：查詢失敗會包在 `dial` 的 OpError 裡，NXDOMAIN 也一樣，
	// 先看 DNSError 才能把它排除。
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTimeout || dnsErr.IsTemporary
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.Code >= 500
	}
	// pgconn 的 PgError 有 SQLState()；用介面比對，⛔ 不必讓 market 套件 import pgx。
	var sqlErr interface{ SQLState() string }
	if errors.As(err, &sqlErr) {
		return transientSQLState(sqlErr.SQLState())
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return true
	}
	return errors.Is(err, io.ErrUnexpectedEOF)
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
