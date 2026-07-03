package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
)

const (
	// 官方文件記載伺服器每 30 秒送一次 heartbeat，超過 3 輪沒收到任何訊息視為斷線
	fugleReadIdleTimeout    = 90 * time.Second
	fugleAuthTimeout        = 10 * time.Second
	fugleReconnectBaseDelay = 2 * time.Second
)

// FugleStreamClient 實作 StreamingSource：Tier 2 熱點用的 WebSocket 即時推送 client。
// 免費方案限制 1 條連線、最多同時訂閱 5 檔，斷線時會自動以指數退避重連並
// 重新訂閱斷線前的名單。
type FugleStreamClient struct {
	apiKey          string
	wsURL           string
	maxSubs         int
	reconnectMaxDur time.Duration
	log             *zap.Logger

	// OnRawMessage 若設定，每一筆收到的原始訊息都會回呼（不影響正常訂閱處理），
	// 供 cmd/fugle-check 在驗證延遲/確認 payload 格式時使用。
	OnRawMessage func(raw []byte)

	mu        sync.Mutex
	conn      *websocket.Conn
	writeMu   sync.Mutex
	callbacks map[string]func(Candle) // symbol -> callback
	channelID map[string]string       // symbol -> channel ID（訂閱後取得，unsubscribe 用）
	idSymbol  map[string]string       // channel ID -> symbol（推送訊息缺 symbol 欄位時回查用）

	closeOnce sync.Once
	closed    chan struct{}
}

func NewFugleStreamClient(cfg config.FugleConfig, log *zap.Logger) *FugleStreamClient {
	maxSubs := cfg.MaxSubscriptions
	if maxSubs <= 0 {
		maxSubs = 5
	}
	reconnectMax := time.Duration(cfg.ReconnectMaxSec) * time.Second
	if reconnectMax <= 0 {
		reconnectMax = 60 * time.Second
	}
	return &FugleStreamClient{
		apiKey:          cfg.APIKey,
		wsURL:           cfg.WSEndpoint,
		maxSubs:         maxSubs,
		reconnectMaxDur: reconnectMax,
		log:             log,
		callbacks:       make(map[string]func(Candle)),
		channelID:       make(map[string]string),
		idSymbol:        make(map[string]string),
		closed:          make(chan struct{}),
	}
}

func (c *FugleStreamClient) MaxSubscriptions() int { return c.maxSubs }

// Start 建立連線並在背景執行讀取迴圈，斷線時以指數退避自動重連。
// ctx 取消或呼叫 Close() 會停止重連迴圈。
func (c *FugleStreamClient) Start(ctx context.Context) {
	go c.runLoop(ctx)
}

func (c *FugleStreamClient) runLoop(ctx context.Context) {
	delay := fugleReconnectBaseDelay
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		default:
		}

		if err := c.connectAndServe(ctx); err != nil {
			if strings.Contains(err.Error(), "Maximum number of connections reached") {
				c.log.Warn("fugle stream disconnected",
					zap.Error(err),
					zap.String("hint", "免費方案同一組 API Key 僅允許 1 條 WebSocket 連線；"+
						"常見成因為 cmd/fugle-check 與本服務同時使用同一組 Key，"+
						"或前一個 process 尚未送出正常關閉導致舊連線名額尚未釋放，稍候將由 backoff 自動重試"),
				)
			} else {
				c.log.Warn("fugle stream disconnected", zap.Error(err))
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		case <-time.After(delay):
		}
		delay *= 2
		if delay > c.reconnectMaxDur {
			delay = c.reconnectMaxDur
		}
	}
}

func (c *FugleStreamClient) connectAndServe(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, fugleAuthTimeout)
	defer cancel()
	conn, _, err := websocket.DefaultDialer.DialContext(dialCtx, c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("fugle ws dial failed: %w", err)
	}
	defer conn.Close()

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	if err := c.writeJSON(newFugleAuthRequest(c.apiKey)); err != nil {
		return fmt.Errorf("fugle ws send auth failed: %w", err)
	}
	if err := c.waitAuthenticated(conn); err != nil {
		return err
	}
	c.log.Info("fugle ws authenticated")

	c.resubscribeAll()

	conn.SetReadDeadline(time.Now().Add(fugleReadIdleTimeout))
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("fugle ws read failed: %w", err)
		}
		conn.SetReadDeadline(time.Now().Add(fugleReadIdleTimeout))
		if c.OnRawMessage != nil {
			c.OnRawMessage(raw)
		}
		c.handleMessage(raw)
	}
}

func (c *FugleStreamClient) waitAuthenticated(conn *websocket.Conn) error {
	deadline := time.Now().Add(fugleAuthTimeout)
	conn.SetReadDeadline(deadline)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("fugle ws auth timeout")
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("fugle ws read (auth) failed: %w", err)
		}
		if c.OnRawMessage != nil {
			c.OnRawMessage(raw)
		}
		var env fugleWSEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		switch env.Event {
		case "authenticated":
			return nil
		case "error":
			return fmt.Errorf("fugle ws auth error: %s", string(env.Data))
		}
	}
}

func (c *FugleStreamClient) writeJSON(v interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("fugle ws not connected")
	}
	return conn.WriteJSON(v)
}

// Subscribe 訂閱單一股票的即時分K推送（candles channel）。同一個 symbol 重複訂閱
// 會覆蓋原本的 callback。呼叫端（hotlist manager）負責確保同時訂閱數不超過
// MaxSubscriptions()。
func (c *FugleStreamClient) Subscribe(ctx context.Context, symbol string, onCandle func(Candle)) error {
	c.mu.Lock()
	c.callbacks[symbol] = onCandle
	c.mu.Unlock()

	return c.writeJSON(newFugleSubscribeRequest("candles", symbol))
}

func (c *FugleStreamClient) Unsubscribe(ctx context.Context, symbol string) error {
	c.mu.Lock()
	id, ok := c.channelID[symbol]
	delete(c.callbacks, symbol)
	delete(c.channelID, symbol)
	if ok {
		delete(c.idSymbol, id)
	}
	c.mu.Unlock()

	if !ok {
		return nil // 尚未收到 subscribed 回應（或本來就沒訂閱），無需送出 unsubscribe
	}
	return c.writeJSON(newFugleUnsubscribeRequest(id))
}

// resubscribeAll 在（重）連線建立且認證成功後，重新訂閱目前所有 callback 中的股票，
// 用於斷線重連時恢復訂閱狀態。
func (c *FugleStreamClient) resubscribeAll() {
	c.mu.Lock()
	symbols := make([]string, 0, len(c.callbacks))
	for sym := range c.callbacks {
		symbols = append(symbols, sym)
	}
	c.channelID = make(map[string]string)
	c.idSymbol = make(map[string]string)
	c.mu.Unlock()

	for _, sym := range symbols {
		if err := c.writeJSON(newFugleSubscribeRequest("candles", sym)); err != nil {
			c.log.Warn("fugle resubscribe failed", zap.String("symbol", sym), zap.Error(err))
		}
	}
}

func (c *FugleStreamClient) handleMessage(raw []byte) {
	var env fugleWSEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		c.log.Warn("fugle ws decode envelope failed", zap.Error(err))
		return
	}

	switch env.Event {
	case "heartbeat", "pong", "subscriptions":
		// 存活訊號／查詢回應，讀取迴圈已重設 read deadline，不需額外處理
	case "subscribed":
		c.handleSubscribed(env.Data)
	case "unsubscribed":
		// callbacks/channelID 已在 Unsubscribe() 呼叫時清除，這裡不需處理
	case "error":
		c.log.Warn("fugle ws error event", zap.ByteString("data", env.Data))
	case "snapshot":
		// 訂閱 candles channel 後，Fugle 會先送一個 snapshot 事件，把當天
		// 至今的整包 1 分K 一次推送過來（結構與 REST /intraday/candles 相同，
		// 但 id/channel 是跟 event 同層而非包在 data 裡，見 fugleWSEnvelope）
		c.handleSnapshot(env)
	default:
		// 官方文件與實測都尚未觀察到「盤中即時更新」用的 event 名稱
		// （收盤後測試只會收到 snapshot + heartbeat），這裡假設它可能長得
		// 像單一根K棒（見 fugleCandleData），盤中重跑 cmd/fugle-check 後
		// 依實際 raw JSON 校正。
		c.handleData(env.Event, env.Data)
	}
}

func (c *FugleStreamClient) handleSnapshot(env fugleWSEnvelope) {
	var resp fugleIntradayCandleResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		c.log.Warn("fugle ws snapshot decode failed", zap.Error(err))
		return
	}

	symbol := resp.Symbol
	if symbol == "" && env.ID != "" {
		c.mu.Lock()
		symbol = c.idSymbol[env.ID]
		c.mu.Unlock()
	}
	if symbol == "" {
		c.log.Debug("fugle ws snapshot missing symbol", zap.String("id", env.ID))
		return
	}

	c.mu.Lock()
	cb := c.callbacks[symbol]
	c.mu.Unlock()
	if cb == nil {
		return
	}

	for _, bar := range resp.Data {
		ts, err := time.Parse(time.RFC3339, bar.Date)
		if err != nil {
			continue
		}
		cb(Candle{
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
	c.log.Info("fugle ws snapshot applied", zap.String("symbol", symbol), zap.Int("bars", len(resp.Data)))
}

func (c *FugleStreamClient) handleSubscribed(raw json.RawMessage) {
	var single fugleSubscribedData
	if err := json.Unmarshal(raw, &single); err == nil && single.Symbol != "" {
		c.mu.Lock()
		c.channelID[single.Symbol] = single.ID
		c.idSymbol[single.ID] = single.Symbol
		c.mu.Unlock()
		return
	}

	var multi []fugleSubscribedData
	if err := json.Unmarshal(raw, &multi); err == nil {
		c.mu.Lock()
		for _, s := range multi {
			c.channelID[s.Symbol] = s.ID
			c.idSymbol[s.ID] = s.Symbol
		}
		c.mu.Unlock()
	}
}

func (c *FugleStreamClient) handleData(event string, raw json.RawMessage) {
	var data fugleCandleData
	if err := json.Unmarshal(raw, &data); err != nil {
		c.log.Debug("fugle ws unrecognized message", zap.String("event", event), zap.ByteString("raw", raw))
		return
	}
	if data.Channel != "" && data.Channel != "candles" && event != "candles" {
		return // 非 candles channel 的推送（例如 trades/books），Tier 2 目前只需要分K
	}

	symbol := data.Symbol
	if symbol == "" && data.ID != "" {
		c.mu.Lock()
		symbol = c.idSymbol[data.ID]
		c.mu.Unlock()
	}
	if symbol == "" {
		c.log.Debug("fugle ws candle data missing symbol", zap.ByteString("raw", raw))
		return
	}

	c.mu.Lock()
	cb := c.callbacks[symbol]
	c.mu.Unlock()
	if cb == nil {
		return
	}

	ts, err := time.Parse(time.RFC3339, data.Date)
	if err != nil {
		ts = time.Now()
	}
	cb(Candle{
		Symbol:    symbol,
		Timeframe: "1m",
		Open:      data.Open,
		High:      data.High,
		Low:       data.Low,
		Close:     data.Close,
		Volume:    data.Volume,
		Timestamp: ts,
	})
}

func (c *FugleStreamClient) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}
