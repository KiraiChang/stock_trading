package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ErrVerificationUnavailable 代表「**驗不了**」，而不是「驗過了，沒有缺口」。
//
// ⚠️ **這個區別是整個缺漏偵測的核心。** 把驗不了記成驗過了，機制就會在最需要它的時候
// 靜默失效——那正是本筆（`issue.md` I-091）要消滅的形狀。所有解析不出來、歸屬對不上、
// 年份不符的情況都必須走這裡，**不得猜測、不得當成「無成交」**。
var ErrVerificationUnavailable = errors.New("verification unavailable")

const (
	defaultExchangeTimeout = 20 * time.Second

	twseCalendarURL = "https://www.twse.com.tw/rwd/zh/holidaySchedule/holidaySchedule"
	twseMarketURL   = "https://www.twse.com.tw/rwd/zh/afterTrading/FMTQIK"
	twseStockDayURL = "https://www.twse.com.tw/rwd/zh/afterTrading/STOCK_DAY"
	tpexStockDayURL = "https://www.tpex.org.tw/www/zh-tw/afterTrading/tradingStock"
)

// 市場別。來自 `stock_symbols.market`（由 I-094 的 StatesBySymbols 一併帶回）。
//
// ⚠️ **主檔查無該標的時 Market 也不存在**，那類候選決定不了要打哪個端點，
// 必須落 ErrVerificationUnavailable，**不得預設成上市**。
const (
	marketListedKeyword = "上市"
	marketOTCKeyword    = "上櫃"
)

// ExchangeReference 是缺漏偵測的四個對照查詢。
//
// **四種 schema、回傳位置與量的單位各自不同，不得假設同構**——TPEx 的資料在
// `tables[0].data`、成交量單位是「張」而不是「股」。
type ExchangeReference interface {
	// TradingCalendar 回傳該西元年度的交易日曆。
	//
	// **不是「休市日清單」**：TWSE 的 holidaySchedule 至少有四種列型，其中
	// 「開始交易」「最後交易」那幾列**是交易日**，全部扣除會讓那幾天的缺 K 永遠測不到。
	TradingCalendar(ctx context.Context, year int) (*TradingCalendar, error)

	// MarketLastTradingDate 回傳市場層級端點目前涵蓋到的最後一個交易日（`source_as_of`）。
	//
	// **它只是觀測值，不能單獨當稽核右界**：端點若回應成功、格式正常、內容卻停滯數日，
	// 視窗會跟著倒退，比它新的缺口全都不會被檢查而且不會被歸類為驗證不可用。
	// 右界要用年度日曆推導，這個值只用來算 lag。
	MarketLastTradingDate(ctx context.Context) (string, error)

	// StockTradedDates 回傳該標的在指定年月**確實有成交**的日期集合（`YYYY-MM-DD`）。
	//
	// **判定是「有列 且 成交量 > 0 且 收盤價 > 0」**。零量列、空值／`--` 佔位列、
	// 以及根本沒有那一列，三者都代表**那天沒有成交**，一律不在回傳集合裡——
	// 缺口的定義是「交易所有、我們沒有」，缺列正是最不該告警的情況。
	StockTradedDates(ctx context.Context, symbol, market string, year int, month time.Month) (map[string]bool, error)

	// IsSourceOpen 回報該來源目前是否斷路中。
	//
	// **呼叫端需要在送請求之前就知道**：breaker 開啟時被跳過的候選
	// **不計入單輪上限、也不更新 last_attempted_at**（它是刻意不被嘗試，不是嘗試失敗）。
	// 只靠「呼叫後拿到 ErrVerificationUnavailable」分不出這兩者。
	IsSourceOpen(source string) bool
}

// TradingCalendar 是某一年度的交易日曆判定結果。
//
// **只存「非交易日」的集合，不存交易日清單**：日曆端點給的是例外（假日與特殊安排），
// 平日預設就是交易日。存例外比存全集更貼近來源，也不會因為端點少給幾筆就整年變空。
type TradingCalendar struct {
	Year int
	// nonTrading 是該年度**明確判定為非交易日**的日期（YYYY-MM-DD），含
	// 「放假／補假」與「市場無交易，僅辦理結算交割作業」兩類。
	nonTrading map[string]bool
}

// IsTradingDay 判斷某一天是不是交易日。
//
// 週末一律不是；其餘看日曆的非交易日集合。**「開始交易」「最後交易」那幾列不在集合裡**，
// 所以它們照常是交易日——那正是不能用集合相減的原因。
func (c *TradingCalendar) IsTradingDay(day time.Time) bool {
	switch day.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return !c.nonTrading[day.Format("2006-01-02")]
}

// TradingDaysBetween 回傳左開右閉區間 `(after, through]` 內的交易日數。
//
// ⚠️ **左開右閉是刻意的，而且是修過的 off-by-one**（`issue.md` I-091）：
// 週一檢查、對照源停在週五時，區間是 `(週五, 週一]` = {週一} → **lag = 1 而不是 0**。
// 少算右端點的話，`market_stale_days=1` 會永遠判不出過期。
//
// 也因此**單位是交易日不是日曆日**：跨週末的日曆日差是 3，但實際只落後一個交易時段，
// 用日曆日會讓每個週一都誤報一次。
func (c *TradingCalendar) TradingDaysBetween(after, through time.Time) int {
	n := 0
	for d := after.AddDate(0, 0, 1); !d.After(through); d = d.AddDate(0, 0, 1) {
		if c.IsTradingDay(d) {
			n++
		}
	}
	return n
}

// NewMergedTradingCalendar 把多個年度的日曆合併成一份。
//
// **跨年視窗一定要用它**：回看視窗在年初會跨到前一年，而只載入當年日曆的話，
// 去年 12 月的假日會被當成交易日 → 那幾天全部誤報成缺 K。
//
// Year 取最後一個（最新的）年度，只作為識別用；判定只看合併後的非交易日集合。
func NewMergedTradingCalendar(cals ...*TradingCalendar) *TradingCalendar {
	out := &TradingCalendar{nonTrading: map[string]bool{}}
	for _, c := range cals {
		if c == nil {
			continue
		}
		if c.Year > out.Year {
			out.Year = c.Year
		}
		for k := range c.nonTrading {
			out.nonTrading[k] = true
		}
	}
	return out
}

// NewTradingCalendarForTest 讓其他套件的測試造一份日曆，不必架 httptest 假伺服器。
//
// **只給測試用**：正式路徑一律走 TradingCalendar，那裡有年份驗證與逐列分類，
// 那些守門才是這個型別的價值所在。
func NewTradingCalendarForTest(year int, nonTrading map[string]bool) *TradingCalendar {
	cal := &TradingCalendar{Year: year, nonTrading: make(map[string]bool, len(nonTrading))}
	for k, v := range nonTrading {
		if v {
			cal.nonTrading[k] = true
		}
	}
	return cal
}

// exchangeReference 是 ExchangeReference 的 HTTP 實作。
type exchangeReference struct {
	http     *http.Client
	breaker  *SourceBreaker
	interval time.Duration
	log      *zap.Logger

	// 端點可覆寫，供 httptest 假伺服器使用。
	calendarURL string
	marketURL   string
	twseURL     string
	tpexURL     string

	lastRequest time.Time

	// 年度日曆快取。**歷史年度不會變**，當年度靠 TTL 容忍年中補班補假修訂。
	// 沒有它的話每輪都會逐年重打 TWSE——對一份整年預先公布、幾乎不變的資料而言
	// 是白費的請求量。
	calMu    sync.Mutex
	calCache map[int]cachedCalendar
	calTTL   time.Duration
}

type cachedCalendar struct {
	cal       *TradingCalendar
	fetchedAt time.Time
}

// ExchangeReferenceOptions 的零值欄位沿用預設。
type ExchangeReferenceOptions struct {
	Timeout time.Duration
	// RequestInterval 是對交易所端點的節流間隔。呼叫端應傳入**已正規化**的值
	// （下限 100ms，見 scheduler 的正規化函式）。
	RequestInterval time.Duration
	// CalendarTTL 是年度日曆的快取存活時間（calendar_ttl_hours）。
	// 零值代表不快取——**正式路徑不該用零值**，那會讓每輪都逐年重打交易所。
	CalendarTTL time.Duration
	// BaseURLs 只在測試中覆寫；正式環境留空。
	CalendarURL string
	MarketURL   string
	TWSEURL     string
	TPExURL     string
}

func NewExchangeReference(
	breaker *SourceBreaker, opts ExchangeReferenceOptions, log *zap.Logger,
) ExchangeReference {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultExchangeTimeout
	}
	pick := func(v, def string) string {
		if v != "" {
			return v
		}
		return def
	}
	return &exchangeReference{
		http:        &http.Client{Timeout: timeout},
		breaker:     breaker,
		interval:    opts.RequestInterval,
		log:         log,
		calendarURL: pick(opts.CalendarURL, twseCalendarURL),
		marketURL:   pick(opts.MarketURL, twseMarketURL),
		twseURL:     pick(opts.TWSEURL, twseStockDayURL),
		tpexURL:     pick(opts.TPExURL, tpexStockDayURL),
		calCache:    make(map[int]cachedCalendar, 2),
		calTTL:      opts.CalendarTTL,
	}
}

func (c *exchangeReference) IsSourceOpen(source string) bool {
	return c.breaker.IsOpen(source)
}

// getJSON 送一次請求並解析 JSON，同時維護節流與來源 breaker。
//
// **breaker 只在請求真的失敗時累加**：解析成功但內容不符預期（年份錯、歸屬對不上）
// 屬於「來源還活著、只是這次答案不能用」，那是 ErrVerificationUnavailable 但**不是**
// 來源故障。把它算進 breaker 會讓一個格式變動直接癱瘓整個來源。
func (c *exchangeReference) getJSON(ctx context.Context, source, rawURL string, out any) error {
	if c.breaker.IsOpen(source) {
		return fmt.Errorf("%w: %s breaker open", ErrVerificationUnavailable, source)
	}
	c.throttle(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrVerificationUnavailable, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.breaker.Fail(source)
		return fmt.Errorf("%w: %s request failed: %v", ErrVerificationUnavailable, source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.breaker.Fail(source)
		return fmt.Errorf("%w: %s status %d", ErrVerificationUnavailable, source, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.breaker.Fail(source)
		return fmt.Errorf("%w: %s read body: %v", ErrVerificationUnavailable, source, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		// **JSON 壞掉算來源故障**：這通常是回了 HTML 錯誤頁或限流頁。
		c.breaker.Fail(source)
		return fmt.Errorf("%w: %s decode: %v", ErrVerificationUnavailable, source, err)
	}
	c.breaker.Succeed(source)
	return nil
}

// throttle 保證兩次請求之間至少間隔 RequestInterval。
//
// **對交易所保守節流**，與 FinMind 的 5 req/min 無關——這是為了不對公開端點造成壓力。
func (c *exchangeReference) throttle(ctx context.Context) {
	if c.interval <= 0 {
		return
	}
	wait := c.interval - time.Since(c.lastRequest)
	if wait > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
	c.lastRequest = time.Now()
}

// ── 年度交易日曆 ────────────────────────────────────────────────────────────

type twseCalendarResponse struct {
	Stat  string     `json:"stat"`
	Data  [][]string `json:"data"`
	Title string     `json:"title"`
}

// TradingCalendar 見介面上的說明。
//
// ⚠️ **參數名一定是 `date`，不是 `queryYear`**（2026-08-26 實測）。`queryYear` 會被
// **完全忽略**：端點照樣回 200、格式正常，但回的是**當年**的資料。用錯參數名的實作
// 會拿當年日曆去判斷去年的交易日而**完全不會報錯**——所以下面一定要驗年份。
func (c *exchangeReference) TradingCalendar(ctx context.Context, year int) (*TradingCalendar, error) {
	if cal, ok := c.cachedCalendar(year); ok {
		return cal, nil
	}
	cal, err := c.fetchCalendar(ctx, year)
	if err != nil {
		// **失敗不寫快取**：否則一次限流會讓整個 TTL 期間都拿不到日曆。
		return nil, err
	}
	c.storeCalendar(year, cal)
	return cal, nil
}

func (c *exchangeReference) cachedCalendar(year int) (*TradingCalendar, bool) {
	if c.calTTL <= 0 {
		return nil, false
	}
	c.calMu.Lock()
	defer c.calMu.Unlock()
	entry, ok := c.calCache[year]
	if !ok || time.Since(entry.fetchedAt) >= c.calTTL {
		return nil, false
	}
	return entry.cal, true
}

func (c *exchangeReference) storeCalendar(year int, cal *TradingCalendar) {
	if c.calTTL <= 0 {
		return
	}
	c.calMu.Lock()
	defer c.calMu.Unlock()
	c.calCache[year] = cachedCalendar{cal: cal, fetchedAt: time.Now()}
}

func (c *exchangeReference) fetchCalendar(ctx context.Context, year int) (*TradingCalendar, error) {
	q := url.Values{}
	q.Set("date", fmt.Sprintf("%04d0101", year))
	q.Set("response", "json")

	var resp twseCalendarResponse
	if err := c.getJSON(ctx, SourceTWSECalendar, c.calendarURL+"?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("%w: calendar %d 回傳 0 筆", ErrVerificationUnavailable, year)
	}

	cal := &TradingCalendar{Year: year, nonTrading: make(map[string]bool, 32)}
	for _, row := range resp.Data {
		if len(row) < 2 {
			return nil, fmt.Errorf("%w: calendar %d 欄位數異常 %v", ErrVerificationUnavailable, year, row)
		}
		day, err := parseCalendarDate(row[0])
		if err != nil {
			return nil, fmt.Errorf("%w: calendar %d 日期無法解析 %q", ErrVerificationUnavailable, year, row[0])
		}
		// **每一列都要驗年份**：`queryYear` 的行為證明 TWSE 會對無效參數回傳看似正常的
		// 資料。不驗的話會拿別年的日曆去判斷，而且不會有任何東西報錯。
		if day.Year() != year {
			return nil, fmt.Errorf("%w: calendar 請求 %d 卻回傳 %d", ErrVerificationUnavailable, year, day.Year())
		}

		name := normalizeCalendarText(row[1])
		desc := ""
		if len(row) >= 3 {
			desc = normalizeCalendarText(row[2])
		}
		kind, err := classifyCalendarRow(day, name, desc)
		if err != nil {
			return nil, fmt.Errorf("%w: calendar %d %s: %v", ErrVerificationUnavailable, year, row[0], err)
		}
		if kind == calendarNonTrading {
			cal.nonTrading[day.Format("2006-01-02")] = true
		}
	}
	// **不對筆數多寡做完整性判斷**：2026 是 27 筆、2025 是 24 筆，筆數本來就逐年不同，
	// 拿它當門檻是憑空的。可驗證的只有「年份相符且非空」，上面兩項已經做了。
	return cal, nil
}

type calendarRowKind int

const (
	calendarTrading calendarRowKind = iota
	calendarNonTrading
	calendarWeekend
)

// classifyCalendarRow 逐列分類，**不是集合相減**。
//
// 四種列型的實例（115 年度實測）：
//
//	放假休市              1150101 開國紀念日、1150925 中秋節          → 非交易日
//	正常交易日的標記        1150102 開始交易日、1150211 最後交易日      → **是交易日**
//	市場無交易僅辦理結算     1150212（四）、1150213（五）              → 非交易日
//	週末列                1150228（六）、1151025（日）               → 忽略
//
// **兩個方向都會錯**：全部扣除會讓 1/2、2/11、2/23 這些真正的交易日被排除；
// 只扣「放假」會漏掉 2/12、2/13——它們是平日、不是放假日，但市場無交易。
//
// ⚠️ **順序不能對調**：規則 1 必須在「放假」之前。某列同時被兩者命中時，交易日語意優先。
//
// ⚠️ 這是**中文名稱的字串比對，本質脆弱**——TWSE 改字就會落到最後一條而變成
// verification_unavailable。**那個降級方向是刻意的**：讓改字壞在明處，而不是被猜過去。
func classifyCalendarRow(day time.Time, name, desc string) (calendarRowKind, error) {
	joined := name + " " + desc
	if strings.Contains(joined, "開始交易") || strings.Contains(joined, "最後交易") {
		return calendarTrading, nil
	}
	// 實測這類列的「說明」是空字串，所以條件要看**名稱**。
	if strings.Contains(name, "市場無交易") {
		return calendarNonTrading, nil
	}
	if strings.Contains(desc, "放假") || strings.Contains(desc, "補假") {
		return calendarNonTrading, nil
	}
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return calendarWeekend, nil
	}
	return calendarTrading, fmt.Errorf("未知的日曆列型別 name=%q desc=%q", name, desc)
}

// normalizeCalendarText 去 HTML 標籤與前後空白。
// 實測 1150211 的說明帶 `<br>`，不處理會讓字串比對落空。
func normalizeCalendarText(s string) string {
	for {
		open := strings.Index(s, "<")
		if open < 0 {
			break
		}
		closeAt := strings.Index(s[open:], ">")
		if closeAt < 0 {
			break
		}
		s = s[:open] + s[open+closeAt+1:]
	}
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", ""))
}

// parseCalendarDate 接受 RWD 的 ISO 日期（`2026-01-01`）與民國格式（`1150101`）。
func parseCalendarDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if day, err := time.Parse("2006-01-02", s); err == nil {
		return day, nil
	}
	if len(s) == 7 {
		y, err1 := strconv.Atoi(s[:3])
		m, err2 := strconv.Atoi(s[3:5])
		d, err3 := strconv.Atoi(s[5:7])
		if err1 == nil && err2 == nil && err3 == nil {
			// 同樣要走嚴格檢查——time.Date 會把 1150231 正規化成 3/3。
			return newStrictDate(y+1911, m, d, s)
		}
	}
	return time.Time{}, fmt.Errorf("無法解析日期 %q", s)
}

// ── 市場層級（source_as_of） ────────────────────────────────────────────────

type twseMarketResponse struct {
	Stat string     `json:"stat"`
	Data [][]string `json:"data"`
}

// MarketLastTradingDate 見介面上的說明。
func (c *exchangeReference) MarketLastTradingDate(ctx context.Context) (string, error) {
	q := url.Values{}
	q.Set("response", "json")

	var resp twseMarketResponse
	if err := c.getJSON(ctx, SourceTWSEMarket, c.marketURL+"?"+q.Encode(), &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("%w: market 端點回傳 0 筆", ErrVerificationUnavailable)
	}
	last := resp.Data[len(resp.Data)-1]
	if len(last) == 0 {
		return "", fmt.Errorf("%w: market 端點末列為空", ErrVerificationUnavailable)
	}
	day, err := parseROCDate(last[0])
	if err != nil {
		return "", fmt.Errorf("%w: market 端點日期無法解析 %q", ErrVerificationUnavailable, last[0])
	}
	return day.Format("2006-01-02"), nil
}

// ── 個股逐月核對 ────────────────────────────────────────────────────────────

// StockTradedDates 見介面上的說明。
func (c *exchangeReference) StockTradedDates(
	ctx context.Context, symbol, market string, year int, month time.Month,
) (map[string]bool, error) {
	switch {
	case strings.Contains(market, marketOTCKeyword):
		return c.tpexTradedDates(ctx, symbol, year, month)
	case strings.Contains(market, marketListedKeyword):
		return c.twseTradedDates(ctx, symbol, year, month)
	default:
		// **不得預設成上市**：主檔查無該標的時 Market 是空字串，猜錯端點會得到一個
		// 看似正常的空結果，然後把整個月報成缺口。
		return nil, fmt.Errorf("%w: 未知市場別 %q（symbol=%s）", ErrVerificationUnavailable, market, symbol)
	}
}

type twseStockDayResponse struct {
	Stat string     `json:"stat"`
	Date string     `json:"date"`
	Data [][]string `json:"data"`
}

// twseTradedDates 查 TWSE 的 STOCK_DAY：`date=YYYYMM01`、`stockNo=`，資料在 `data`，
// 欄位是 `[日期, 成交股數, …, 收盤價, …]`。
func (c *exchangeReference) twseTradedDates(
	ctx context.Context, symbol string, year int, month time.Month,
) (map[string]bool, error) {
	q := url.Values{}
	q.Set("date", fmt.Sprintf("%04d%02d01", year, int(month)))
	q.Set("stockNo", symbol)
	q.Set("response", "json")

	var resp twseStockDayResponse
	if err := c.getJSON(ctx, SourceTWSEStockDay, c.twseURL+"?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	if resp.Stat != "" && resp.Stat != "OK" {
		return nil, fmt.Errorf("%w: twse stat=%q (symbol=%s)", ErrVerificationUnavailable, resp.Stat, symbol)
	}

	// **歸屬驗證**，與 TPEx 的 subtitle 檢查對稱。
	//
	// ⚠️ TWSE 一樣有「參數被忽略卻回 200 ＋ 格式正常」的前科（`queryYear` 就是）。
	// 沒有這道檢查時，拿到別的月份的資料會讓候選日期比對不到，
	// 於是該月被記成 **verified**——「拿錯資料卻顯示驗證成功」，
	// 正是本筆要消滅的形狀。
	//
	// ⛔ **`date` 是必要的，而且要嚴格解析**（不是「有值才檢查」＋ HasPrefix）：
	// 逐列檢查在 `data: []` 上是 no-op，所以**空回應完全沒有列可以交叉佐證歸屬**——
	// 那正是最需要 `date` 的情況。缺欄位或格式怪異（例如 `202608-invalid` 會被
	// HasPrefix 放行）時，別月的空回應就會被當成「該月無成交」而記成 verified。
	wantYearMonth := fmt.Sprintf("%04d%02d", year, int(month))
	if err := checkTWSEResponseMonth(resp.Date, year, month); err != nil {
		return nil, fmt.Errorf(
			"%w: twse 回應歸屬不符 date=%q (symbol=%s want=%s): %v",
			ErrVerificationUnavailable, resp.Date, symbol, wantYearMonth, err)
	}

	out := make(map[string]bool, len(resp.Data))
	for _, row := range resp.Data {
		// 欄位順序：0 日期、1 成交股數、…、6 收盤價。
		if len(row) < 7 {
			return nil, fmt.Errorf("%w: twse 欄位數異常 %v (symbol=%s)", ErrVerificationUnavailable, row, symbol)
		}
		day, err := parseROCDate(row[0])
		if err != nil {
			return nil, fmt.Errorf("%w: twse 日期無法解析 %q (symbol=%s)", ErrVerificationUnavailable, row[0], symbol)
		}
		// **逐列也要驗**：整份 date 對了不代表每一列都屬於那個月。
		if day.Year() != year || day.Month() != month {
			return nil, fmt.Errorf(
				"%w: twse 列的年月不符 %q (symbol=%s want=%s)",
				ErrVerificationUnavailable, row[0], symbol, wantYearMonth)
		}
		traded, err := hasTrade(row[1], row[6])
		if err != nil {
			// **讀不懂就是驗不了**，不得靜默當成「那天沒有成交」。
			// **兩層都要 wrap**：呼叫端用 ErrVerificationUnavailable 決定收斂，
			// 用 ErrUnrecognisedExchangeValue 分辨「來源改版」與其他不可用。
			return nil, fmt.Errorf("%w: %w (symbol=%s)", ErrVerificationUnavailable, err, symbol)
		}
		if !traded {
			continue
		}
		out[day.Format("2006-01-02")] = true
	}
	return out, nil
}

// checkTWSEResponseMonth 驗 STOCK_DAY 回應的 `date` 確實是請求的那個年月。
//
// 格式是 `YYYYMMDD`（實測 `20260801`）。**必須是可嚴格解析的完整日期**——
// 只比對前綴的話 `202608-invalid` 這種內容也會通過，而它代表的是
// 「我們讀不懂這份回應」，不是「這個月沒有成交」。
func checkTWSEResponseMonth(raw string, year int, month time.Month) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("缺少 date 欄位")
	}
	if len(raw) != 8 {
		return fmt.Errorf("date 長度應為 8（YYYYMMDD），得到 %d", len(raw))
	}
	y, err1 := strconv.Atoi(raw[:4])
	m, err2 := strconv.Atoi(raw[4:6])
	d, err3 := strconv.Atoi(raw[6:8])
	if err1 != nil || err2 != nil || err3 != nil {
		return errors.New("date 不是數字")
	}
	if _, err := newStrictDate(y, m, d, raw); err != nil {
		return err
	}
	if y != year || time.Month(m) != month {
		return fmt.Errorf("date 屬於 %04d-%02d", y, m)
	}
	return nil
}

type tpexTradingStockResponse struct {
	Stat   string `json:"stat"`
	Tables []struct {
		Subtitle string     `json:"subtitle"`
		Data     [][]string `json:"data"`
	} `json:"tables"`
}

// tpexTradedDates 查 TPEx 的 tradingStock。
//
// ⚠️ **參數與回傳形狀都與 TWSE 不同**：`code=`、`date=YYYY/MM/DD`（完整日期＋斜線，
// 需 URL-encode），資料在 `tables[0].data`，欄位是
// `[日期, 成交張數, 成交仟元, 開盤, 最高, 最低, 收盤, 漲跌, 筆數]`——
// **成交量單位是「張」不是「股」，換算前不要與 TWSE 互相比較**。
//
// ⚠️ **這個端點有一個與 `queryYear` 同類的陷阱**：參數名或格式錯誤時**不一定回錯誤**。
// 實測 `stkno=1785&d=115/08` 回 **HTTP 200 ＋ `data: []` ＋ subtitle「請輸入股票代碼及
// 資料年月」**——空結果不是錯誤。天真的實作會把它讀成「那個月完全沒有成交」，
// 於是**把整個月都報成缺口**。所以下面一定要驗歸屬。
func (c *exchangeReference) tpexTradedDates(
	ctx context.Context, symbol string, year int, month time.Month,
) (map[string]bool, error) {
	q := url.Values{}
	q.Set("code", symbol)
	q.Set("date", fmt.Sprintf("%04d/%02d/01", year, int(month)))
	q.Set("response", "json")

	var resp tpexTradingStockResponse
	if err := c.getJSON(ctx, SourceTPExStockDay, c.tpexURL+"?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	if resp.Stat != "" && resp.Stat != "ok" && !strings.EqualFold(resp.Stat, "ok") {
		return nil, fmt.Errorf("%w: tpex stat=%q (symbol=%s)", ErrVerificationUnavailable, resp.Stat, symbol)
	}
	if len(resp.Tables) == 0 {
		return nil, fmt.Errorf("%w: tpex 回傳沒有 tables (symbol=%s)", ErrVerificationUnavailable, symbol)
	}

	// **歸屬驗證**：subtitle 必須同時包含請求的 symbol 與年月（民國）。
	// 這一項足以擋掉錯參數，因為錯參數的 subtitle 是「請輸入股票代碼及資料年月」。
	subtitle := resp.Tables[0].Subtitle
	rocYearMonth := fmt.Sprintf("%d年%02d月", year-1911, int(month))
	if !strings.Contains(subtitle, symbol) || !strings.Contains(subtitle, rocYearMonth) {
		return nil, fmt.Errorf(
			"%w: tpex 回應歸屬不符 subtitle=%q (symbol=%s want=%s)",
			ErrVerificationUnavailable, subtitle, symbol, rocYearMonth)
	}

	// ⛔ **不得把「data 非空」也列為條件**：標的整月合法停止交易時，參數正確、歸屬正確、
	// data 就是空陣列——那是「該月無成交」的正確答案，不是來源不可用。
	// 把它判成 unavailable 會讓 2867 這類標的整月都變成驗不了。
	out := make(map[string]bool, len(resp.Tables[0].Data))
	for _, row := range resp.Tables[0].Data {
		// 欄位順序：0 日期、1 成交張數、…、6 收盤價。
		if len(row) < 7 {
			return nil, fmt.Errorf("%w: tpex 欄位數異常 %v (symbol=%s)", ErrVerificationUnavailable, row, symbol)
		}
		day, err := parseROCDate(row[0])
		if err != nil {
			return nil, fmt.Errorf("%w: tpex 日期無法解析 %q (symbol=%s)", ErrVerificationUnavailable, row[0], symbol)
		}
		// 與 TWSE 對稱：subtitle 對了不代表每一列都屬於那個月。
		if day.Year() != year || day.Month() != month {
			return nil, fmt.Errorf(
				"%w: tpex 列的年月不符 %q (symbol=%s want=%s)",
				ErrVerificationUnavailable, row[0], symbol, rocYearMonth)
		}
		traded, err := hasTrade(row[1], row[6])
		if err != nil {
			// **讀不懂就是驗不了**，不得靜默當成「那天沒有成交」。
			// **兩層都要 wrap**：呼叫端用 ErrVerificationUnavailable 決定收斂，
			// 用 ErrUnrecognisedExchangeValue 分辨「來源改版」與其他不可用。
			return nil, fmt.Errorf("%w: %w (symbol=%s)", ErrVerificationUnavailable, err, symbol)
		}
		if !traded {
			continue
		}
		out[day.Format("2006-01-02")] = true
	}
	return out, nil
}

// ErrUnrecognisedExchangeValue 代表欄位的內容**無法辨識**，通常是來源 schema 變動。
//
// ⚠️ **與「已知佔位符」必須分開**：`""`、`--`、`---` 是交易所表達「那天沒有成交」的
// 正常寫法；而一個沒看過的字串代表**我們讀不懂這一列**。把後者也當成「沒有成交」，
// 會讓一次格式變動把真正的缺口全部洗成 verified——那正是本筆要消滅的
// 「驗不了卻記成驗過了」。
var ErrUnrecognisedExchangeValue = errors.New("unrecognised exchange value")

// hasTrade 判定「那天有沒有成交」：**成交量 > 0 且 收盤價 > 0**。
//
// ⚠️ **只看日期有沒有出現在回傳裡是不夠的**：停止交易或無量的標的可能仍以零量列存在，
// 只看 key 會把合法的無成交誤報成缺口。
//
// 收盤價那一項是為了擋掉上游的零價列，與 toStoreCandles 擋零價是同一個理由。
//
// 回傳 error 時代表**讀不懂這一列**，呼叫端必須升成 ErrVerificationUnavailable，
// 不得當成「沒有成交」。
func hasTrade(volumeRaw, closeRaw string) (bool, error) {
	volume, err := parseExchangeNumber(volumeRaw)
	if err != nil {
		if errors.Is(err, errExchangePlaceholder) {
			return false, nil // 已知佔位符＝那天沒有成交
		}
		return false, fmt.Errorf("%w: 成交量 %q", ErrUnrecognisedExchangeValue, volumeRaw)
	}
	closePrice, err := parseExchangeNumber(closeRaw)
	if err != nil {
		if errors.Is(err, errExchangePlaceholder) {
			return false, nil
		}
		return false, fmt.Errorf("%w: 收盤價 %q", ErrUnrecognisedExchangeValue, closeRaw)
	}
	return volume > 0 && closePrice > 0, nil
}

// errExchangePlaceholder 是「這格是交易所的佔位符」，屬於**正常**情況。
var errExchangePlaceholder = errors.New("exchange placeholder")

// parseExchangeNumber 解析交易所回傳的數字：可能帶千分位逗號、前後空白，
// 也可能是 `--` 或空字串這類佔位。
//
// **佔位符與解析失敗回不同的錯誤**——呼叫端要靠這個區別決定是「沒有成交」還是「讀不懂」。
func parseExchangeNumber(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" || s == "--" || s == "---" {
		return 0, errExchangePlaceholder
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// parseROCDate 解析民國日期 `115/08/26`。
func parseROCDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("非民國日期格式 %q", s)
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}, fmt.Errorf("非民國日期格式 %q", s)
	}
	return newStrictDate(y+1911, m, d, s)
}

// newStrictDate 建立日期並**反向檢查**年月日與輸入完全一致。
//
// ⚠️ **time.Date 會自動正規化**：`2026-02-31` 會變成 `2026-03-03`，而且不回任何錯誤。
// 少了這個檢查，來源的格式或內容錯誤會被**靜默轉成另一個合法日期**——
// 那比 verification_unavailable 更糟，因為它看起來完全正常，
// 卻讓整個比對建立在一個不存在的日子上。
func newStrictDate(year, month, day int, raw string) (time.Time, error) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("日期超出範圍 %q", raw)
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, fmt.Errorf("日期不存在 %q", raw)
	}
	return t, nil
}
