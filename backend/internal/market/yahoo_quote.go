package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/pkg/timeutil"
)

const (
	// 非官方 API，無文件化 rate limit，預設保守；批次請求計為一次。
	yahooDefaultRateLimit = 20
	yahooDefaultBatchSize = 40
	// 系統 symbol（如 "2330"）不含交易所尾碼，送 Yahoo 前補上。TWSE 為 .TW；
	// 上櫃（TPEx）需 .TWO，可由呼叫端顯式帶含尾碼的 symbol 覆寫（見 toYahooSymbol）。
	yahooDefaultSuffix = ".TW"
)

// YahooQuoteClient 實作 BatchQuoteSource：以 Yahoo 股市內部端點批次拉取盤中 1 分K。
type YahooQuoteClient struct {
	baseURL   string
	http      *http.Client
	limiter   *rateLimiter
	rateLimit int
	batchSize int
}

func NewYahooQuoteClient(cfg config.YahooConfig) *YahooQuoteClient {
	rl := cfg.RateLimit
	if rl <= 0 {
		rl = yahooDefaultRateLimit
	}
	bs := cfg.BatchSize
	if bs <= 0 {
		bs = yahooDefaultBatchSize
	}
	return &YahooQuoteClient{
		baseURL: cfg.BaseURL,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
		limiter:   newRateLimiter(rl),
		rateLimit: rl,
		batchSize: bs,
	}
}

func (c *YahooQuoteClient) RateLimit() int { return c.rateLimit }
func (c *YahooQuoteClient) BatchSize() int { return c.batchSize }

// FetchIntradayCandlesBatch 一次拉取多檔當日 1 分K。symbols 為系統格式（如 "2330"）；
// 回傳 map 以系統格式為 key，無資料（或全為 null 棒）的 symbol 不會出現在 map 中。
func (c *YahooQuoteClient) FetchIntradayCandlesBatch(ctx context.Context, symbols []string) (map[string][]Candle, error) {
	if len(symbols) == 0 {
		return map[string][]Candle{}, nil
	}

	// 系統 symbol → Yahoo symbol，並保留反查表以便把回應的 "2330.TW" 對映回原始請求格式
	ysyms := make([]string, 0, len(symbols))
	reverse := make(map[string]string, len(symbols))
	for _, s := range symbols {
		ys := toYahooSymbol(s)
		ysyms = append(ysyms, ys)
		reverse[ys] = s
	}

	entries, err := c.fetch(ctx, ysyms)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]Candle, len(entries))
	for _, e := range entries {
		sysSym, ok := reverse[e.Symbol]
		if !ok {
			sysSym = fromYahooSymbol(e.Symbol)
		}
		candles := buildYahooCandles(sysSym, e.Chart)
		if len(candles) > 0 {
			result[sysSym] = candles
		}
	}
	return result, nil
}

// fetch 對 Yahoo 端點發出單次批次請求，symbols 為 Yahoo 格式（含尾碼）。
func (c *YahooQuoteClient) fetch(ctx context.Context, symbols []string) ([]yahooChartEntry, error) {
	if err := c.limiter.wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(symbols), nil)
	if err != nil {
		return nil, err
	}
	// Yahoo 端點對缺少瀏覽器特徵的請求可能回 403，帶常見 UA/Accept 降低被擋機率。
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("yahoo read body error: http_status=%d: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo http error: status=%d msg=%s", resp.StatusCode, truncateBody(body))
	}

	var entries []yahooChartEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("yahoo decode error: body=%s: %w", truncateBody(body), err)
	}
	return entries, nil
}

// buildURL 組出 matrix 參數格式（以 ";" 分隔）的請求 URL。symbols 為 JSON 陣列，
// [ ] " 需 URL-encode 為 %5B %5D %22；autoRefresh 為前端 cache-buster 時間戳。
func (c *YahooQuoteClient) buildURL(symbols []string) string {
	quoted := make([]string, len(symbols))
	for i, s := range symbols {
		quoted[i] = "%22" + s + "%22"
	}
	symbolsParam := "%5B" + strings.Join(quoted, ",") + "%5D"
	return fmt.Sprintf("%s;autoRefresh=%d;symbols=%s;type=tick",
		c.baseURL, time.Now().UnixMilli(), symbolsParam)
}

// buildYahooCandles 將單檔 chart 組成 1 分K；任一 OHLC 為 null 的棒（盤前/盤後或缺值）跳過。
// timestamp 為 Unix 秒（絕對時刻），統一轉為台北時區以與 FinMind 分K 的時間表示一致。
func buildYahooCandles(symbol string, ch yahooChart) []Candle {
	if len(ch.Indicators.Quote) == 0 {
		return nil
	}
	q := ch.Indicators.Quote[0]
	candles := make([]Candle, 0, len(ch.Timestamp))
	for i := range ch.Timestamp {
		o := floatAt(q.Open, i)
		h := floatAt(q.High, i)
		l := floatAt(q.Low, i)
		cl := floatAt(q.Close, i)
		if o == nil || h == nil || l == nil || cl == nil {
			continue
		}
		var vol int64
		if v := int64At(q.Volume, i); v != nil {
			vol = *v
		}
		candles = append(candles, Candle{
			Symbol:    symbol,
			Timeframe: "1m",
			Open:      *o,
			High:      *h,
			Low:       *l,
			Close:     *cl,
			Volume:    vol,
			// 端點不提供逐棒成交金額，intraday VWAP 無法由此計算（比照 FinMind 分K）。
			Amount:    0,
			Timestamp: time.Unix(ch.Timestamp[i], 0).In(timeutil.TaipeiTZ),
		})
	}
	return candles
}

func floatAt(a []*float64, i int) *float64 {
	if i < len(a) {
		return a[i]
	}
	return nil
}

func int64At(a []*int64, i int) *int64 {
	if i < len(a) {
		return a[i]
	}
	return nil
}

// toYahooSymbol 將系統 symbol（如 "2330"）轉為 Yahoo 格式（如 "2330.TW"）。
// 已含尾碼（含 "."）者原樣保留，讓呼叫端可顯式指定 .TWO（上櫃）等交易所。
func toYahooSymbol(sym string) string {
	if strings.Contains(sym, ".") {
		return sym
	}
	return sym + yahooDefaultSuffix
}

// fromYahooSymbol 將 Yahoo 格式（如 "2330.TW"）轉回系統 symbol（去除交易所尾碼）。
func fromYahooSymbol(ysym string) string {
	if i := strings.Index(ysym, "."); i >= 0 {
		return ysym[:i]
	}
	return ysym
}
