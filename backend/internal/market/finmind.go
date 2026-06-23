package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/pkg/timeutil"
)

type FinMindClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewFinMindClient(cfg config.FinMindConfig) *FinMindClient {
	return &FinMindClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type finmindResp struct {
	Msg    string            `json:"msg"`
	Status int               `json:"status"`
	Data   []json.RawMessage `json:"data"`
}

func (c *FinMindClient) fetch(ctx context.Context, params url.Values) ([]json.RawMessage, error) {
	params.Set("token", c.apiKey)
	reqURL := c.baseURL + "/data?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result finmindResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("finmind error: %s", result.Msg)
	}
	return result.Data, nil
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
func (c *FinMindClient) FetchMinuteCandles(ctx context.Context, symbol string, date time.Time) ([]Candle, error) {
	dateStr := date.Format("2006-01-02")
	params := url.Values{
		"dataset":    {"TaiwanStockPriceMinute"},
		"data_id":    {symbol},
		"start_date": {dateStr},
		"end_date":   {dateStr},
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
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", raw.Date, timeutil.TaipeiTZ)
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
			Volume:    raw.Volume,
			Amount:    raw.Amount,
			Timestamp: ts,
		})
	}
	return candles, nil
}
