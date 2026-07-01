package market

import "encoding/json"

// ── REST 回應結構（依 https://developer.fugle.tw 官方文件確認） ──────────

// fugleErrorResponse 為 Fugle REST API 非 200 回應的錯誤格式
type fugleErrorResponse struct {
	Message string `json:"message"`
}

// fugleQuoteResponse 對應 GET /intraday/quote/{symbol}
type fugleQuoteResponse struct {
	Date          string  `json:"date"`
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	ClosePrice    float64 `json:"closePrice"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Total         struct {
		TradeValue  float64 `json:"tradeValue"`
		TradeVolume int64   `json:"tradeVolume"`
	} `json:"total"`
	// LastTrade 等欄位官方文件未附完整範例，實測時以 raw JSON 為準（見 cmd/fugle-check）
}

// fugleIntradayCandleResponse 對應 GET /intraday/candles/{symbol}
type fugleIntradayCandleResponse struct {
	Date   string                   `json:"date"`
	Symbol string                   `json:"symbol"`
	Data   []fugleIntradayCandleBar `json:"data"`
}

type fugleIntradayCandleBar struct {
	Date    string  `json:"date"` // RFC3339，例如 "2023-05-29T09:00:00.000+08:00"
	Open    float64 `json:"open"`
	High    float64 `json:"high"`
	Low     float64 `json:"low"`
	Close   float64 `json:"close"`
	Volume  int64   `json:"volume"`
	Average float64 `json:"average"`
}

// ── WebSocket 協定結構（依官方 websocket-api/getting-started 文件確認） ──

// fugleWSEnvelope 為所有 WS 訊息共通的外層格式
type fugleWSEnvelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type fugleAuthRequest struct {
	Event string `json:"event"`
	Data  struct {
		APIKey string `json:"apikey"`
	} `json:"data"`
}

func newFugleAuthRequest(apiKey string) fugleAuthRequest {
	req := fugleAuthRequest{Event: "auth"}
	req.Data.APIKey = apiKey
	return req
}

type fugleSubscribeRequest struct {
	Event string `json:"event"`
	Data  struct {
		Channel string `json:"channel"`
		Symbol  string `json:"symbol"`
	} `json:"data"`
}

func newFugleSubscribeRequest(channel, symbol string) fugleSubscribeRequest {
	req := fugleSubscribeRequest{Event: "subscribe"}
	req.Data.Channel = channel
	req.Data.Symbol = symbol
	return req
}

type fugleSubscribedData struct {
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
}

type fugleUnsubscribeRequest struct {
	Event string `json:"event"`
	Data  struct {
		ID string `json:"id"`
	} `json:"data"`
}

func newFugleUnsubscribeRequest(channelID string) fugleUnsubscribeRequest {
	req := fugleUnsubscribeRequest{Event: "unsubscribe"}
	req.Data.ID = channelID
	return req
}

// fugleCandleData 為 candles channel 推送資料的猜測欄位。
//
// 官方文件（截至查證當下）只說明了連線/認證/訂閱/心跳協定，並未附上
// candles/trades channel 實際推送訊息的 JSON 範例，此結構依 REST
// intraday candles 回應格式與業界慣例推測。正式上線前務必用
// cmd/fugle-check 實際連線觀察 raw JSON，確認欄位名稱（尤其是否有
// isClose/lastPrice 等欄位）後再修正這裡的欄位對應。
type fugleCandleData struct {
	Channel string  `json:"channel"` // 若推送訊息用 event:"data" 包一層 channel 欄位才會有值
	ID      string  `json:"id"`      // 訂閱時取得的 channel ID，用於在 symbol 缺欄位時回查
	Symbol  string  `json:"symbol"`
	Date    string  `json:"date"`
	Open    float64 `json:"open"`
	High    float64 `json:"high"`
	Low     float64 `json:"low"`
	Close   float64 `json:"close"`
	Volume  int64   `json:"volume"`
	Average float64 `json:"average"`
}
