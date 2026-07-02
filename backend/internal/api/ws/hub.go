package ws

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Event struct {
	Type   string      `json:"type"`
	Symbol string      `json:"symbol"`
	Data   interface{} `json:"data"`
}

type clientMsg struct {
	Action  string   `json:"action"`
	Symbols []string `json:"symbols"`
}

type client struct {
	conn       *websocket.Conn
	send       chan []byte
	subscribed map[string]bool
}

type Hub struct {
	clients    map[*client]struct{}
	broadcast  chan Event
	register   chan *client
	unregister chan *client
	mu         sync.RWMutex
	log        *zap.Logger
}

func NewHub(log *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		broadcast:  make(chan Event, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		log:        log,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()

		case evt := <-h.broadcast:
			b, _ := json.Marshal(evt)
			h.mu.RLock()
			for c := range h.clients {
				if c.subscribed[evt.Symbol] || c.subscribed["*"] {
					select {
					case c.send <- b:
					default:
						// 緩衝滿，跳過這個 client
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 從 Signal Engine 呼叫，推送事件給訂閱的 clients
func (h *Hub) Broadcast(evt Event) {
	h.broadcast <- evt
}

// ServeWS 升級為 WebSocket 連線
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws upgrade failed", zap.Error(err))
		return
	}

	c := &client{
		conn:       conn,
		send:       make(chan []byte, 64),
		subscribed: make(map[string]bool),
	}
	h.register <- c

	go c.writePump(h)
	go c.readPump(h)
}

func (c *client) writePump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg clientMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		h.mu.Lock()
		switch msg.Action {
		case "subscribe":
			// 防禦性上限：即時監聽最多同時 store.MaxWatchedSymbols 檔，跟
			// watchlist 的 watched 欄位上限一致（那裡是真正的把關點，這裡
			// 只是避免任何 client 端 bug 或誤用送出超量訂閱）
			for _, sym := range msg.Symbols {
				if _, ok := c.subscribed[sym]; !ok && len(c.subscribed) >= store.MaxWatchedSymbols {
					h.log.Warn("ws subscribe rejected: limit reached",
						zap.String("symbol", sym), zap.Int("limit", store.MaxWatchedSymbols))
					continue
				}
				c.subscribed[sym] = true
			}
		case "unsubscribe":
			for _, sym := range msg.Symbols {
				delete(c.subscribed, sym)
			}
		}
		h.mu.Unlock()
	}
}
