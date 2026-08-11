package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

const (
	fetchMaxRetries  = 3
	fetchBaseBackoff = time.Second
)

// ErrInsufficientTier 代表 FinMind token 等級不足以存取該 dataset（例如
// TaiwanStockKBar 需要 Sponsor 級），這是帳號權限問題，重試或換 symbol 都沒用。
var ErrInsufficientTier = errors.New("finmind: token tier insufficient for this dataset")

// isTierError 判斷 FinMind 回應是否為「帳號等級不足」（訊息範例：
// "Your level is register. Please update your user level. Detail information:..."）
func isTierError(status int, msg string) bool {
	return status == 400 && (strings.Contains(msg, "user level") || strings.Contains(msg, "Sponsor"))
}

type FinMindClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
	limiter *rateLimiter
}

func NewFinMindClient(cfg config.FinMindConfig) *FinMindClient {
	return &FinMindClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: newRateLimiter(cfg.RateLimit),
	}
}

// rateLimiter 依「每分鐘請求數」節流，讓 intraday / daily close / backfill
// 共用同一個節流器，避免對 FinMind 發出爆量請求觸發限流。
type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRateLimiter(perMinute int) *rateLimiter {
	if perMinute <= 0 {
		perMinute = 1
	}
	return &rateLimiter{interval: time.Minute / time.Duration(perMinute)}
}

func (l *rateLimiter) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(l.next) {
		wait = l.next.Sub(now)
		l.next = l.next.Add(l.interval)
	} else {
		l.next = now.Add(l.interval)
	}
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type finmindResp struct {
	Msg    string            `json:"msg"`
	Status int               `json:"status"`
	Data   []json.RawMessage `json:"data"`
}

// fetch 對 FinMind API 發出請求，節流後執行，並對逾時/5xx/429/402（限流、額度用盡）
// 做有限次數的指數退避重試；其他錯誤（如參數錯誤）不重試。
func (c *FinMindClient) fetch(ctx context.Context, params url.Values) ([]json.RawMessage, error) {
	params.Set("token", c.apiKey)
	reqURL := c.baseURL + "/data?" + params.Encode()

	var lastErr error
	for attempt := 0; attempt <= fetchMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := fetchBaseBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if err := c.limiter.wait(ctx); err != nil {
			return nil, err
		}

		data, retryable, err := c.doFetch(ctx, reqURL)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, fmt.Errorf("finmind fetch failed after %d retries: %w", fetchMaxRetries, lastErr)
}

func (c *FinMindClient) doFetch(ctx context.Context, reqURL string) ([]json.RawMessage, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("finmind read body error: http_status=%d: %w", resp.StatusCode, err)
	}

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, true, fmt.Errorf("finmind http error: status=%d body=%s", resp.StatusCode, truncateBody(body))
	}

	var result finmindResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, false, fmt.Errorf("finmind decode error: http_status=%d body=%s: %w", resp.StatusCode, truncateBody(body), err)
	}
	if resp.StatusCode != http.StatusOK || result.Status != 200 {
		msg := result.Msg
		if msg == "" {
			// FinMind 對某些錯誤（如缺少/錯誤參數）不會回傳 {msg,status} 格式，
			// 這種情況把原始回應內容附上，避免看到空白訊息無從排查
			msg = truncateBody(body)
		}
		if isTierError(result.Status, msg) {
			return nil, false, fmt.Errorf("%w: %s", ErrInsufficientTier, msg)
		}
		// 402 = 額度用盡, 429 = 請求過於頻繁，兩者稍等後重試通常會恢復
		retryable := result.Status == 402 || result.Status == 429
		return nil, retryable, fmt.Errorf("finmind error: http_status=%d api_status=%d msg=%s", resp.StatusCode, result.Status, msg)
	}
	return result.Data, false, nil
}

func truncateBody(body []byte) string {
	const maxLen = 300
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// FetchDailyCandles 拉取日K資料
func (c *FinMindClient) FetchDailyCandles(ctx context.Context, symbol string, start, end time.Time) ([]Candle, error) {
	params := url.Values{
		"dataset":    {"TaiwanStockPrice"},
		"data_id":    {symbol},
		"start_date": {start.Format("2006-01-02")},
		"end_date":   {end.Format("2006-01-02")},
	}

	rows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	candles := make([]Candle, 0, len(rows))
	for _, row := range rows {
		var raw RawDailyCandle
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02", raw.Date, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}
		candles = append(candles, Candle{
			Symbol:    symbol,
			Timeframe: "1d",
			Open:      raw.Open,
			High:      raw.High,
			Low:       raw.Low,
			Close:     raw.Close,
			Volume:    raw.Volume,
			Amount:    raw.Amount,
			Timestamp: ts,
		})
	}
	return candles, nil
}

// FetchMinuteCandles 拉取分K資料
// dataset=TaiwanStockKBar（v4 API 用此取代已下架的 TaiwanStockPriceMinute），
// 需要 FinMind Sponsor 級以上的 token，且限制單次請求一天資料
func (c *FinMindClient) FetchMinuteCandles(ctx context.Context, symbol string, date time.Time) ([]Candle, error) {
	dateStr := date.Format("2006-01-02")
	params := url.Values{
		"dataset":    {"TaiwanStockKBar"},
		"data_id":    {symbol},
		"start_date": {dateStr},
	}

	rows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	candles := make([]Candle, 0, len(rows))
	for _, row := range rows {
		var raw RawMinuteCandle
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", raw.Date+" "+raw.Minute, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}
		candles = append(candles, Candle{
			Symbol:    symbol,
			Timeframe: "1m",
			Open:      raw.Open,
			High:      raw.High,
			Low:       raw.Low,
			Close:     raw.Close,
			Volume:    int64(raw.Volume),
			// TaiwanStockKBar 不提供成交金額，intraday VWAP 目前無法用此欄位計算
			Amount:    0,
			Timestamp: ts,
		})
	}
	return candles, nil
}

// RawSplitPrice 是 dataset=TaiwanStockSplitPrice 的原始欄位。
// 這個 dataset 在 register tier 就能用，**而且可以不帶 data_id 整批抓**
// （對照：TaiwanStockPriceAdj 與 TaiwanStockDividendResult 的整批查詢都需要 Sponsor）。
type RawSplitPrice struct {
	Date        string  `json:"date"`
	StockID     string  `json:"stock_id"`
	Type        string  `json:"type"`
	BeforePrice float64 `json:"before_price"`
	AfterPrice  float64 `json:"after_price"`
}

// FetchSplitPrices 拉取區間內全市場的價格重訂事件（見 docs/todo.md T-042）。
//
// 全市場 2015～2026 只有 33 筆，所以這裡刻意不帶 data_id：一次請求抓完整段歷史，
// 比逐檔抓省掉數百次請求與對應的 rate limit 等待。
//
// **這個 dataset 不是只有「分割」**（2026-08-11 實測 33 筆的分佈）：
//
//	面額變更 22、反分割 6、分割 4、type 為空字串 1
//
// 四種都是同一件事——把價格重新表述，所以 after/before 一律是正確的調整係數：
// 面額變更 312→31.2（0.1）、反分割 3.28→22.96（7.0）、分割 188.65→47.16（0.25）。
// **係數可以大於 1**（反分割時股數變少、價格變高），對應的成交量調整
// `volume / factor` 也因此是縮小，方向仍然正確。
//
// ActionType 直接記 FinMind 給的 type 而不是一律寫 SPLIT——否則反分割與面額變更會被
// 標成分割，之後要分辨就得回頭重抓。type 為空時（實測有一筆 00631L）記為 UNKNOWN，
// 係數照算，因為前後價都是有效的。
func (c *FinMindClient) FetchSplitPrices(ctx context.Context, start, end time.Time) ([]store.CorporateAction, error) {
	params := url.Values{
		"dataset":    {"TaiwanStockSplitPrice"},
		"start_date": {start.Format("2006-01-02")},
		"end_date":   {end.Format("2006-01-02")},
	}

	rows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	actions := make([]store.CorporateAction, 0, len(rows))
	for _, row := range rows {
		var raw RawSplitPrice
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		// 價格為 0 或負數時算不出係數，而且會讓還原價變成 0。跳過並記錄，
		// 不要寫進事件表——一筆壞事件會污染該檔的整段歷史。
		if raw.BeforePrice <= 0 || raw.AfterPrice <= 0 {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02", raw.Date, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}
		actionType := strings.TrimSpace(raw.Type)
		if actionType == "" {
			actionType = store.CorporateActionUnknown
		}
		actions = append(actions, store.CorporateAction{
			Symbol:      raw.StockID,
			EventDate:   ts,
			ActionType:  actionType,
			BeforePrice: raw.BeforePrice,
			AfterPrice:  raw.AfterPrice,
			Factor:      raw.AfterPrice / raw.BeforePrice,
			// 分割／反分割／面額變更**都會改變股數**，所以成交量係數等於價格係數。
			// 漏設這欄會寫入 0，被 ck_corporate_actions_volume_factor 擋下——
			// 2026-08-11 正式環境就是這樣失敗的（Phase 1 時還沒有這個欄位）。
			VolumeFactor: raw.AfterPrice / raw.BeforePrice,
			Source:       "TaiwanStockSplitPrice",
		})
	}
	return actions, nil
}

// RawCapitalReduction 是 dataset=TaiwanStockCapitalReductionReferencePrice 的原始欄位。
//
// **逐檔查詢在 register tier 就能用，整批（不帶 data_id）才需要 Sponsor**——
// 與 TaiwanStockDividendResult 同一個模式。2026-08-11 實測確認。
type RawCapitalReduction struct {
	Date        string  `json:"date"`
	StockID     string  `json:"stock_id"`
	BeforePrice float64 `json:"ClosingPriceonTheLastTradingDay"`
	AfterPrice  float64 `json:"PostReductionReferencePrice"`
	Reason      string  `json:"ReasonforCapitalReduction"`
}

// FetchCapitalReductions 取單一標的的減資事件（見 docs/issue.md I-069）。
//
// **減資與反分割在數學上是同一件事**：股數變少、價格變高，所以係數 > 1，
// 而且成交量係數等於價格係數（股數確實改變）。因此不需要新的係數概念。
//
// 資料源正確性的佐證：三個已知案例的 `ClosingPriceonTheLastTradingDay`
// 與我們 candles 裡的前一根收盤價完全相同（6243 22.75、2603 80.80、2478 35.80）。
//
// **只能逐檔**（整批需 Sponsor tier），所以與除權息一樣受 rate limit 節制，
// 擴到全市場前需要增量更新。
func (c *FinMindClient) FetchCapitalReductions(ctx context.Context, symbol string) ([]store.CorporateAction, error) {
	params := url.Values{
		"dataset":    {"TaiwanStockCapitalReductionReferencePrice"},
		"data_id":    {symbol},
		"start_date": {"2000-01-01"},
		"end_date":   {timeutil.TodayTaipei().Format("2006-01-02")},
	}

	rows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	actions := make([]store.CorporateAction, 0, len(rows))
	for _, row := range rows {
		var raw RawCapitalReduction
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		// 價格為 0 或負數算不出係數，也會讓還原價變成 0。跳過而不是寫進事件表——
		// 一筆壞事件會污染該檔的整段歷史。
		if raw.BeforePrice <= 0 || raw.AfterPrice <= 0 {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02", raw.Date, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}
		factor := raw.AfterPrice / raw.BeforePrice
		actions = append(actions, store.CorporateAction{
			Symbol:      symbol,
			EventDate:   ts,
			ActionType:  store.CorporateActionCapitalReduction,
			BeforePrice: raw.BeforePrice,
			AfterPrice:  raw.AfterPrice,
			Factor:      factor,
			// 減資改變股數，成交量係數與價格係數相同。
			VolumeFactor: factor,
			Source:       "TaiwanStockCapitalReductionReferencePrice",
		})
	}
	return actions, nil
}
