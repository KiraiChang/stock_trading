package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trading/backend/internal/config"
)

// FugleQuoteClient 實作 QuoteSource：Tier 1 廣度掃描用的 REST 輪詢 client。
type FugleQuoteClient struct {
	apiKey    string
	baseURL   string
	http      *http.Client
	limiter   *rateLimiter
	rateLimit int
}

func NewFugleQuoteClient(cfg config.FugleConfig) *FugleQuoteClient {
	rl := cfg.QuoteRateLimit
	if rl <= 0 {
		rl = 60
	}
	return &FugleQuoteClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.RESTBaseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		limiter:   newRateLimiter(rl),
		rateLimit: rl,
	}
}

func (c *FugleQuoteClient) RateLimit() int { return c.rateLimit }

// FetchQuote 拉取單檔股票即時報價（GET /intraday/quote/{symbol}），主要用於
// 延遲驗證（比對 quote 回應時間與本地時間差）。
func (c *FugleQuoteClient) FetchQuote(ctx context.Context, symbol string) (*fugleQuoteResponse, error) {
	var resp fugleQuoteResponse
	if err := c.doGet(ctx, "/intraday/quote/"+symbol, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FetchIntradayCandles 拉取當日 1 分K（GET /intraday/candles/{symbol}?timeframe=1），
// 作為 Tier 1 廣度掃描取代 FinMind 分K 拉取的資料來源。
func (c *FugleQuoteClient) FetchIntradayCandles(ctx context.Context, symbol string) ([]Candle, error) {
	var resp fugleIntradayCandleResponse
	path := fmt.Sprintf("/intraday/candles/%s?timeframe=1", symbol)
	if err := c.doGet(ctx, path, &resp); err != nil {
		return nil, err
	}

	candles := make([]Candle, 0, len(resp.Data))
	for _, bar := range resp.Data {
		ts, err := time.Parse(time.RFC3339, bar.Date)
		if err != nil {
			continue
		}
		candles = append(candles, Candle{
			Symbol:    symbol,
			Timeframe: "1m",
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Timestamp: ts,
		})
	}
	return candles, nil
}

// doGet 依 rate limiter 節流後對 Fugle REST API 發出請求。Fugle 的 429/錯誤格式
// 與 FinMind 不同（{"message": "..."}），故不共用 finmind.go 的重試邏輯，
// 僅做一次呼叫並回傳明確錯誤，重試交由呼叫端（Tier 1 掃描迴圈）決定。
func (c *FugleQuoteClient) doGet(ctx context.Context, path string, out interface{}) error {
	if err := c.limiter.wait(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fugle http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("fugle read body error: http_status=%d: %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr fugleErrorResponse
		_ = json.Unmarshal(body, &apiErr)
		msg := apiErr.Message
		if msg == "" {
			msg = truncateBody(body)
		}
		return fmt.Errorf("fugle http error: status=%d msg=%s", resp.StatusCode, msg)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("fugle decode error: body=%s: %w", truncateBody(body), err)
	}
	return nil
}
