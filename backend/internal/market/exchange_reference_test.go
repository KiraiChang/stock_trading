package market

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestReference(t *testing.T, handler http.HandlerFunc) (ExchangeReference, *SourceBreaker, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	breaker := NewSourceBreaker(5, time.Hour)
	// RequestInterval 留 0：測試不需要節流，留著只會讓每支測試多等半秒。
	ref := NewExchangeReference(breaker, ExchangeReferenceOptions{
		CalendarURL: srv.URL + "/calendar",
		MarketURL:   srv.URL + "/market",
		TWSEURL:     srv.URL + "/twse",
		TPExURL:     srv.URL + "/tpex",
	}, zap.NewNop())
	return ref, breaker, srv
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// ── 年度日曆 ────────────────────────────────────────────────────────────────

// 矩陣 #9、#10：四種列型要被分開處理。
//
// **兩個方向都會錯**：全部扣除會讓 1/2、2/11、2/23 這些真正的交易日被排除；
// 只扣「放假」會漏掉 2/12、2/13——它們是平日、不是放假日，但市場無交易。
func TestTradingCalendarClassifiesEachRowKind(t *testing.T) {
	body := `{"stat":"OK","data":[
		["2026-01-01","開國紀念日","放假一日"],
		["2026-01-02","國曆新年開始交易日","1月2日<br>開始交易"],
		["2026-02-11","春節前最後交易日","2月11日最後交易"],
		["2026-02-12","市場無交易，僅辦理結算交割作業",""],
		["2026-02-13","市場無交易，僅辦理結算交割作業",""],
		["2026-02-23","春節後開始交易日","2月23日開始交易"],
		["2026-02-28","和平紀念日","放假一日"],
		["2026-09-25","中秋節","放假一日"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	cal, err := ref.TradingCalendar(context.Background(), 2026)
	if err != nil {
		t.Fatalf("TradingCalendar failed: %v", err)
	}

	day := func(s string) time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	// 矩陣 #9：這三天是**正常交易日的標記**，不得被當成休市扣除。
	for _, s := range []string{"2026-01-02", "2026-02-11", "2026-02-23"} {
		if !cal.IsTradingDay(day(s)) {
			t.Errorf("%s 是交易日（開始／最後交易），不得被扣除", s)
		}
	}
	// 矩陣 #10：平日但「市場無交易」，必須扣除，否則會誤報那兩天缺 K。
	for _, s := range []string{"2026-02-12", "2026-02-13"} {
		if cal.IsTradingDay(day(s)) {
			t.Errorf("%s 是平日但市場無交易，必須扣除", s)
		}
	}
	// 放假列。
	if cal.IsTradingDay(day("2026-01-01")) || cal.IsTradingDay(day("2026-09-25")) {
		t.Error("放假日不得被當成交易日")
	}
	// 沒出現在日曆裡的平日預設是交易日。
	if !cal.IsTradingDay(day("2026-08-27")) {
		t.Error("日曆沒提到的平日應是交易日")
	}
	// 週末一律不是。
	if cal.IsTradingDay(day("2026-08-29")) {
		t.Error("週六不是交易日")
	}
}

// 矩陣 #11：未知列型別**不得猜測**，要轉成 verification_unavailable。
//
// 這是中文字串比對，TWSE 改字就會落到這裡——降級方向刻意選「壞在明處」。
func TestTradingCalendarRejectsUnknownRowKind(t *testing.T) {
	body := `{"stat":"OK","data":[["2026-03-05","某種新的安排",""]]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.TradingCalendar(context.Background(), 2026)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("未知列型別要判 unavailable，得到 %v", err)
	}
}

// 矩陣 #17：回傳的年份與請求的年份不符時，**不得**拿它去判斷。
//
// `queryYear` 的實測行為證明 TWSE 會對無效參數回 200 ＋ 格式正常但年份錯誤的資料，
// 用錯參數名的實作會拿當年日曆去判去年而完全不會報錯。
func TestTradingCalendarRejectsWrongYear(t *testing.T) {
	body := `{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"]]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.TradingCalendar(context.Background(), 2025)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("年份不符要判 unavailable，得到 %v", err)
	}
}

// 空結果同樣是 unavailable——不得當成「這一年沒有任何假日」。
func TestTradingCalendarRejectsEmptyResult(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(`{"stat":"OK","data":[]}`))

	_, err := ref.TradingCalendar(context.Background(), 2026)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("0 筆要判 unavailable，得到 %v", err)
	}
}

// 矩陣 #34、#35：lag 是**左開右閉的交易日數**。
//
// 跨週末時日曆日差是 3 但實際只落後一個交易時段；少算右端點會讓門檻設 1 時
// 永遠判不出過期。
func TestTradingCalendarLagCountsTradingDaysHalfOpen(t *testing.T) {
	// 2/12、2/13 市場無交易，用來當連假情境。
	body := `{"stat":"OK","data":[
		["2026-02-12","市場無交易，僅辦理結算交割作業",""],
		["2026-02-13","市場無交易，僅辦理結算交割作業",""]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))
	cal, err := ref.TradingCalendar(context.Background(), 2026)
	if err != nil {
		t.Fatalf("TradingCalendar failed: %v", err)
	}
	day := func(s string) time.Time {
		d, _ := time.Parse("2006-01-02", s)
		return d
	}

	// 跨週末：對照源停在週五 8/28，週一 8/31 檢查 → (週五, 週一] = {週一} → lag = 1。
	if got := cal.TradingDaysBetween(day("2026-08-28"), day("2026-08-31")); got != 1 {
		t.Errorf("跨週末 lag = %d, 期望 1（日曆日差 3 是誤導）", got)
	}
	// 同一天 → lag = 0（左開，右端點就是自己）。
	if got := cal.TradingDaysBetween(day("2026-08-31"), day("2026-08-31")); got != 0 {
		t.Errorf("同日 lag = %d, 期望 0", got)
	}
	// 跨連假：2/11(三) → 2/16(一)，中間 2/12、2/13 市場無交易、2/14-15 週末，
	// 所以只有 2/16 一個交易日。
	if got := cal.TradingDaysBetween(day("2026-02-11"), day("2026-02-16")); got != 1 {
		t.Errorf("跨連假 lag = %d, 期望 1", got)
	}
}

// ── 市場層級 ────────────────────────────────────────────────────────────────

func TestMarketLastTradingDateReadsLastRow(t *testing.T) {
	body := `{"stat":"OK","data":[
		["115/08/24","1","2","3","4","5"],
		["115/08/25","1","2","3","4","5"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	got, err := ref.MarketLastTradingDate(context.Background())
	if err != nil {
		t.Fatalf("MarketLastTradingDate failed: %v", err)
	}
	if got != "2026-08-25" {
		t.Errorf("got %q, 期望 2026-08-25（民國 115 → 西元 2026）", got)
	}
}

// ── 個股逐月核對 ────────────────────────────────────────────────────────────

// 矩陣 #5c：零量列、空值／`--` 佔位列、缺列，**三者都是「無成交（正常）」**。
//
// 唯一會進入結果集合的是「有列且量價皆 > 0」——那才是缺口的必要條件
// （缺口＝交易所有、我們沒有）。方向弄反的話，2867 停止買賣的每一天都會被報成缺口。
func TestStockTradedDatesOnlyCountsRowsWithVolumeAndPrice(t *testing.T) {
	// 欄位順序：0 日期、1 成交股數、2 成交金額、3 開盤、4 最高、5 最低、**6 收盤價**。
	// `date` 是必要的歸屬證據，真實 TWSE 一定會回。
	body := `{"stat":"OK","date":"20260801","data":[
		["115/08/03","76,212,030","1","2","3","4","9.70","5","6"],
		["115/08/04","0","1","2","3","4","9.70","5","6"],
		["115/08/05","1,000","1","2","3","4","0.00","5","6"],
		["115/08/06","--","1","2","3","4","--","5","6"],
		["115/08/07","","1","2","3","4","","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	got, err := ref.StockTradedDates(context.Background(), "2867", "上市", 2026, time.August)
	if err != nil {
		t.Fatalf("StockTradedDates failed: %v", err)
	}
	if len(got) != 1 || !got["2026-08-03"] {
		t.Errorf("只有量價皆 > 0 的那天算有成交，得到 %v", got)
	}
	// 08/10 根本沒有列——**缺列代表交易所沒有那天的成交證據，是最不該告警的情況**。
	if got["2026-08-10"] {
		t.Error("缺列不得被當成有成交")
	}
}

// 矩陣 #5b：上櫃走 tradingStock，能力與 TWSE 對等，**不得**判成不可驗。
func TestStockTradedDatesRoutesOTCToTPEx(t *testing.T) {
	var gotPath string
	ref, _, _ := newTestReference(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stat":"ok","tables":[{
			"subtitle":"1785 光洋科 115年08月 成交資訊",
			"data":[["115/08/26","1,234","5,678","10","11","9","10.5","0.1","20"]]
		}]}`))
	})

	got, err := ref.StockTradedDates(context.Background(), "1785", "上櫃", 2026, time.August)
	if err != nil {
		t.Fatalf("上櫃歷史必須查得到：%v", err)
	}
	if !got["2026-08-26"] {
		t.Errorf("got %v", got)
	}
	// 參數形狀與 TWSE 不同：code= 與 date=YYYY/MM/DD。
	if !containsAll(gotPath, "/tpex", "code=1785", "date=2026%2F08%2F01") {
		t.Errorf("TPEx 參數形狀不對：%s", gotPath)
	}
}

// 矩陣 #5d：**參數錯誤但回 200 空結果**必須判 unavailable。
//
// 實測 `stkno=1785&d=115/08` 回 HTTP 200 ＋ data:[] ＋ subtitle「請輸入股票代碼及資料年月」。
// 天真的實作會把它讀成「那個月完全沒有成交」，於是把整個月都報成缺口。
func TestStockTradedDatesRejectsTPExMismatchedSubtitle(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(
		`{"stat":"ok","tables":[{"subtitle":"請輸入股票代碼及資料年月","data":[]}]}`))

	_, err := ref.StockTradedDates(context.Background(), "1785", "上櫃", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("歸屬不符要判 unavailable，得到 %v", err)
	}
}

// ⛔ **歸屬通過之後，空資料＝該月無成交**，不是來源不可用。
//
// 標的整月合法停止交易時，參數正確、歸屬正確、data 就是空陣列。把它判成 unavailable
// 會讓 2867 這類標的整月都變成驗不了。
func TestStockTradedDatesAcceptsEmptyDataWhenSubtitleMatches(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(
		`{"stat":"ok","tables":[{"subtitle":"2867 三商壽 115年08月 成交資訊","data":[]}]}`))

	got, err := ref.StockTradedDates(context.Background(), "2867", "上櫃", 2026, time.August)
	if err != nil {
		t.Fatalf("歸屬通過的空資料是「該月無成交」，不是錯誤：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("空資料應回空集合，得到 %v", got)
	}
}

// 矩陣 #20：主檔查無該標的（Market 也不存在）→ unavailable，**不得預設成上市**。
//
// 猜錯端點會得到一個看似正常的空結果，然後把整個月報成缺口。
func TestStockTradedDatesRejectsUnknownMarket(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(`{"stat":"OK","data":[]}`))

	_, err := ref.StockTradedDates(context.Background(), "9999", "", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("未知市場別要判 unavailable，得到 %v", err)
	}
}

// 矩陣 #7：端點失敗（非 200）要判 unavailable，**不得誤判成「無成交」**，
// 而且要累加來源 breaker。
func TestStockTradedDatesFailureCountsTowardBreaker(t *testing.T) {
	ref, breaker, _ := newTestReference(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("端點失敗要判 unavailable，得到 %v", err)
	}
	if breaker.Failures(SourceTWSEStockDay) != 1 {
		t.Errorf("實際送出且失敗的請求要累加 breaker，得到 %d", breaker.Failures(SourceTWSEStockDay))
	}
}

// 矩陣 #23：breaker 已開時直接回 unavailable，**不再送請求**。
func TestExchangeReferenceSkipsRequestWhenBreakerOpen(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stat":"OK","data":[]}`))
	}))
	defer srv.Close()

	breaker := NewSourceBreaker(1, time.Hour)
	breaker.Fail(SourceTWSEStockDay) // 門檻 1 → 立刻開啟
	ref := NewExchangeReference(breaker, ExchangeReferenceOptions{TWSEURL: srv.URL}, zap.NewNop())

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("breaker 開啟時要判 unavailable，得到 %v", err)
	}
	if calls != 0 {
		t.Errorf("breaker 開啟時不該再送請求，實際送了 %d 次", calls)
	}
}

// 格式變動（回 HTML 錯誤頁）算來源故障：判 unavailable ＋ 累加 breaker。
func TestExchangeReferenceTreatsNonJSONAsSourceFailure(t *testing.T) {
	ref, breaker, _ := newTestReference(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>maintenance</html>`))
	})

	_, err := ref.TradingCalendar(context.Background(), 2026)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("非 JSON 要判 unavailable，得到 %v", err)
	}
	if breaker.Failures(SourceTWSECalendar) != 1 {
		t.Errorf("格式壞掉要累加 breaker，得到 %d", breaker.Failures(SourceTWSECalendar))
	}
}

// 年份不符**不算來源故障**：來源還活著，只是這次答案不能用。
//
// 把它算進 breaker 會讓一個格式變動直接癱瘓整個來源，
// 連原本驗得到的資料也一起被跳過。
func TestExchangeReferenceWrongYearDoesNotTripBreaker(t *testing.T) {
	ref, breaker, _ := newTestReference(t, jsonHandler(
		`{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"]]}`))

	_, _ = ref.TradingCalendar(context.Background(), 2025)
	if breaker.Failures(SourceTWSECalendar) != 0 {
		t.Errorf("內容不符預期不是來源故障，不該累加 breaker，得到 %d",
			breaker.Failures(SourceTWSECalendar))
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ── Review 回歸：格式異常不得被讀成「沒有成交」 ──────────────────────────────

// **已知佔位符與無法辨識的格式必須分開。**
//
// 原本兩者都回 false → 呼叫端跳過該日期 → 整月可能記成 verified，
// 一次 schema 變動就會把真正的缺口全部洗成「驗過了沒問題」。
func TestStockTradedDatesRejectsUnrecognisedNumberFormat(t *testing.T) {
	// 成交量是個看不懂的字串（不是 `--` 也不是空值）。
	body := `{"stat":"OK","date":"20260801","data":[
		["115/08/03","N/A","1","2","3","4","9.70","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("讀不懂的格式要判 unavailable，得到 %v", err)
	}
	if !errors.Is(err, ErrUnrecognisedExchangeValue) {
		t.Errorf("錯誤要指出是無法辨識的值，得到 %v", err)
	}
}

// 對照組：已知佔位符仍然是「沒有成交」，**不得**因為上一條而被誤判成 unavailable。
func TestStockTradedDatesStillTreatsPlaceholdersAsNoTrade(t *testing.T) {
	body := `{"stat":"OK","date":"20260801","data":[
		["115/08/03","--","1","2","3","4","--","5","6"],
		["115/08/04","","1","2","3","4","","5","6"],
		["115/08/05","0","1","2","3","4","9.70","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	got, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if err != nil {
		t.Fatalf("佔位符與零量是正常的「沒有成交」，不是錯誤：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("三列都沒有成交，應回空集合，得到 %v", got)
	}
}

// ── Review 回歸：年度日曆快取（calendar_ttl_hours 原本是無效設定） ──────────

func TestTradingCalendarCachesWithinTTL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"]]}`))
	}))
	defer srv.Close()

	ref := NewExchangeReference(NewSourceBreaker(5, time.Hour), ExchangeReferenceOptions{
		CalendarURL: srv.URL, CalendarTTL: time.Hour,
	}, zap.NewNop())

	for i := 0; i < 3; i++ {
		if _, err := ref.TradingCalendar(context.Background(), 2026); err != nil {
			t.Fatalf("TradingCalendar failed: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("TTL 內應只打一次交易所，實際 %d 次", calls)
	}
	// 不同年度是不同的 key，不該共用快取。
	if _, err := ref.TradingCalendar(context.Background(), 2025); err == nil {
		t.Error("2025 的請求會拿到 2026 的資料，應被年份驗證擋下")
	}
	if calls != 2 {
		t.Errorf("不同年度要各自請求，實際 %d 次", calls)
	}
}

// **失敗不得寫進快取**：否則一次限流會讓整個 TTL 期間都拿不到日曆。
func TestTradingCalendarDoesNotCacheFailures(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"]]}`))
	}))
	defer srv.Close()

	ref := NewExchangeReference(NewSourceBreaker(5, time.Hour), ExchangeReferenceOptions{
		CalendarURL: srv.URL, CalendarTTL: time.Hour,
	}, zap.NewNop())

	if _, err := ref.TradingCalendar(context.Background(), 2026); err == nil {
		t.Fatal("第一次應失敗")
	}
	if _, err := ref.TradingCalendar(context.Background(), 2026); err != nil {
		t.Fatalf("失敗不該被快取住，第二次應成功：%v", err)
	}
}

// ── Review 回歸：TWSE 也要驗回應歸屬（#1） ──────────────────────────────────

// TWSE 有與 `queryYear` 同類的前科：參數被忽略卻回 200 ＋ 格式正常。
//
// 沒有歸屬檢查時，拿到別月的資料會讓候選日期比對不到，
// 於是該月被記成 **verified**——「拿錯資料卻顯示驗證成功」。
func TestStockTradedDatesRejectsTWSEMismatchedMonth(t *testing.T) {
	// 請求 2026-08，回的是 2026-07。
	body := `{"stat":"OK","date":"20260701","data":[
		["115/07/03","1,000","1","2","3","4","9.70","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("回應年月不符要判 unavailable，得到 %v", err)
	}
}

// 整份 date 對了，**但個別列屬於別的月份**——逐列也要驗。
func TestStockTradedDatesRejectsTWSERowOutsideRequestedMonth(t *testing.T) {
	body := `{"stat":"OK","date":"20260801","data":[
		["115/08/03","1,000","1","2","3","4","9.70","5","6"],
		["115/09/01","1,000","1","2","3","4","9.70","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("列的年月不符要判 unavailable，得到 %v", err)
	}
}

// 對照組：歸屬正確時照常回傳，**空 data 是「該月無成交」不是錯誤**
// （與 TPEx 的規則一致——標的整月合法停止交易時就是這個形狀）。
func TestStockTradedDatesAcceptsTWSEMatchingMonthWithEmptyData(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(`{"stat":"OK","date":"20260801","data":[]}`))

	got, err := ref.StockTradedDates(context.Background(), "2867", "上市", 2026, time.August)
	if err != nil {
		t.Fatalf("歸屬正確的空資料不是錯誤：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("應回空集合，得到 %v", got)
	}
}

// ── Review 回歸：不存在的日期不得被靜默正規化（#2） ─────────────────────────

// **time.Date 會把 115/02/31 正規化成 3/3，而且不回任何錯誤。**
//
// 少了反向檢查，來源的內容錯誤會被靜默轉成另一個合法日期——
// 那比 verification_unavailable 更糟，因為它看起來完全正常。
func TestStockTradedDatesRejectsNonExistentDate(t *testing.T) {
	body := `{"stat":"OK","date":"20260201","data":[
		["115/02/31","1,000","1","2","3","4","9.70","5","6"]
	]}`
	ref, _, _ := newTestReference(t, jsonHandler(body))

	_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.February)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("不存在的日期要判 unavailable，得到 %v", err)
	}
}

// 年度日曆的民國分支同樣要走嚴格檢查。
func TestTradingCalendarRejectsNonExistentROCDate(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(
		`{"stat":"OK","data":[["1150231","某個節日","放假一日"]]}`))

	_, err := ref.TradingCalendar(context.Background(), 2026)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("不存在的日期要判 unavailable，得到 %v", err)
	}
}

// 對照組：閏年的 2/29 是**存在的**，不得被誤擋。
//
// 2024 是閏年（民國 113）。少了這條，一個「乾脆把 2 月上限設成 28」的實作也會通過上一條。
func TestParseROCDateAcceptsLeapDay(t *testing.T) {
	got, err := parseROCDate("113/02/29")
	if err != nil {
		t.Fatalf("2024-02-29 是存在的日期：%v", err)
	}
	if got.Format("2006-01-02") != "2024-02-29" {
		t.Errorf("got %s", got.Format("2006-01-02"))
	}
	// 平年的 2/29 不存在。
	if _, err := parseROCDate("114/02/29"); err == nil {
		t.Error("2025-02-29 不存在，應回錯誤")
	}
}

// ── Review 回歸：空資料時 `date` 是唯一的歸屬證據（第三輪 #1） ───────────────

// **逐列檢查在 `data: []` 上是 no-op**，所以空回應完全沒有列可以交叉佐證歸屬。
// 那正是最需要 `date` 的情況——缺了它或格式怪異時，別月的空回應會被當成
// 「該月無成交」而記成 verified。
func TestStockTradedDatesRequiresTWSEDateForEmptyData(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"缺少 date", `{"stat":"OK","data":[]}`},
		// `202608-invalid` 會被 HasPrefix("202608") 放行——那是前一版的洞。
		{"date 格式錯誤", `{"stat":"OK","date":"202608-invalid","data":[]}`},
		{"date 長度不足", `{"stat":"OK","date":"202608","data":[]}`},
		{"date 不是數字", `{"stat":"OK","date":"2026AA01","data":[]}`},
		// 不存在的日期同樣不能通過——它代表我們讀不懂這份回應。
		{"date 是不存在的日期", `{"stat":"OK","date":"20260231","data":[]}`},
		{"date 屬於別的月份", `{"stat":"OK","date":"20260701","data":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, _, _ := newTestReference(t, jsonHandler(tc.body))

			_, err := ref.StockTradedDates(context.Background(), "2330", "上市", 2026, time.August)
			if !errors.Is(err, ErrVerificationUnavailable) {
				t.Fatalf("歸屬無法確認時要判 unavailable，得到 %v", err)
			}
		})
	}
}

// 對照組：`date` 正確的空月份是**合法的「該月無成交」**，不得被上面那組誤擋。
//
// 少了這條，一個「乾脆把所有空 data 都判成 unavailable」的實作也會通過——
// 而那會讓 2867 這類整月停止交易的標的永遠驗不了。
func TestStockTradedDatesAcceptsValidEmptyMonth(t *testing.T) {
	ref, _, _ := newTestReference(t, jsonHandler(`{"stat":"OK","date":"20260801","data":[]}`))

	got, err := ref.StockTradedDates(context.Background(), "2867", "上市", 2026, time.August)
	if err != nil {
		t.Fatalf("date 正確的空月份是合法的「無成交」：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("應回空集合，得到 %v", got)
	}
}
