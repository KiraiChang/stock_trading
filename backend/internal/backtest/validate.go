package backtest

import (
	"fmt"
	"regexp"
	"time"
)

// 建回測任務前的檢查與對外的錯誤代碼（I-137）。python 的 backtest/job_errors.py 實作同一份規則：
// python 在兩條執行路徑的共用入口再檢查一次（繞過 Go 或修正之前已經排隊的任務會 failed）。
// 兩邊一致性由 scripts/check-backtest-contract.py 比對（MaxDate、backtrader 名單、標的數預設值、錯誤案例檔）。

// MaxDate：取數的上界是「結束日的隔天 00:00（台北）」，9999-12-31 加一天會溢位。python 端用同一個值。
const MaxDate = "2099-12-31"

// MinDate：年份 0000 Go 的 time.Parse 收、python 的 date 不收（PostgreSQL 也會報錯 → 500），所以 Go 這邊擋掉。
const MinDate = "0001-01-01"

// BacktraderStrategies 是 python STRATEGY_MAP（backtrader 引擎）的策略。它們只看第一檔資料，所以一次只接受 1 檔
// （真正的多檔另見 docs/issue.md I-138）。前端的單選名單與 python 的 STRATEGY_MAP 必須相同。
var BacktraderStrategies = map[string]bool{
	"breakout_v1": true,
}

var dateRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func validDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return false
	}
	return s >= MinDate && s <= MaxDate // 同為 YYYY-MM-DD，字串比較就是日期比較
}

// ValidateRequest 回傳不合法的原因代碼（跟 python 寫進 backtest_jobs.error 的代碼相同）；合法時回空字串。
// 必填欄位由 handler 先檢查。策略名稱是否認得由 python 判斷（Go 沒有 modular 的名單）。
func ValidateRequest(req CreateRequest, maxSymbols int) string {
	if !validDate(req.StartDate) {
		return "invalid_date:start_date"
	}
	if !validDate(req.EndDate) {
		return "invalid_date:end_date"
	}
	if req.StartDate > req.EndDate {
		return "start_after_end"
	}
	n := len(req.Symbols)
	if n > maxSymbols {
		return fmt.Sprintf("too_many_symbols:%d:%d", n, maxSymbols)
	}
	if BacktraderStrategies[req.Strategy] && n > 1 {
		return fmt.Sprintf("backtrader_single_symbol:%d", n)
	}
	return ""
}

// 允許回傳給 API 的 error 形式（I-137）。`cancelled by user` 是取消任務寫的既有文字，
// `interrupted_on_restart` 是 I-134 的既有代碼。
var publicJobErrorRes = []*regexp.Regexp{
	regexp.MustCompile(`^invalid_date:(start_date|end_date)$`),
	regexp.MustCompile(`^start_after_end$`),
	regexp.MustCompile(`^too_many_symbols:[0-9]{1,9}:[0-9]{1,9}$`),
	regexp.MustCompile(`^backtrader_single_symbol:[0-9]{1,9}$`),
	regexp.MustCompile(`^task_too_large:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}$`),
	regexp.MustCompile(`^symbol_too_large:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}$`),
	regexp.MustCompile(`^insufficient_data:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}$`),
	regexp.MustCompile(`^no_data_in_range$`),
	regexp.MustCompile(`^unknown_strategy$`),
	regexp.MustCompile(`^interrupted_on_restart$`),
	regexp.MustCompile(`^internal_error$`),
	regexp.MustCompile(`^cancelled by user$`),
}

// PublicJobError 是 GET /backtest 與 /backtest/:job_id 回傳 error 之前的轉換：空字串照舊、符合允許規則的照舊，
// 其他（I-137 之前留下的原始例外文字，可能帶 SQL 或參數）一律 internal_error。DB 的舊資料⛔ 不改。
func PublicJobError(s string) string {
	if s == "" {
		return s
	}
	for _, re := range publicJobErrorRes {
		if re.MatchString(s) {
			return s
		}
	}
	return "internal_error"
}
