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

// fugleWSEnvelope 為所有 WS 訊息共通的外層格式。
//
// 實測發現不同 event 的欄位位置不一致：subscribed 事件的 id/channel/symbol
// 包在 data 裡面（見 fugleSubscribedData）；但 snapshot 事件的 id/channel
// 卻是跟 event/data 同層的欄位，data 內容則是整包歷史K棒（見 handleSnapshot）。
// 因此這裡把 ID/Channel 也定義在外層，兩種格式都能對上。
type fugleWSEnvelope struct {
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
	ID      string          `json:"id,omitempty"`
	Channel string          `json:"channel,omitempty"`
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

// fugleCandleData 為「單一根K棒」推送格式的猜測欄位，用於 subscribed/snapshot
// 之外、目前尚未實測觀察到的 event（推測是盤中即時更新用，收盤後測試只會
// 收到 heartbeat，需在盤中重跑 cmd/fugle-check 才能確認實際格式並修正這裡）。
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
