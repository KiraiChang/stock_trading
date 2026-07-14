package market

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestNextReconnectDelay(t *testing.T) {
	maxDur := 60 * time.Second
	baseCooldown := fugleMaxConnCooldown
	maxConnErr := errors.New("fugle ws read (auth) failed: websocket: close 1001 (going away): Maximum number of connections reached")
	normalErr := errors.New("fugle ws dial failed: dial tcp: lookup api.fugle.tw: i/o timeout")
	abnormalErr := errors.New("fugle ws read failed: websocket: close 1006 (abnormal closure): unexpected EOF")

	// max-connections：以當前 cooldown 為 wait，冷卻加倍帶進下一輪，backoff 重置回 base。
	// 這是本次修復的核心——名額被前一條 1006 幽靈連線佔著時，不能每 2s 硬撞；且連續撞到
	// 要讓安靜窗愈拉愈長，確保最終超過伺服器釋放 timeout 而非永久鎖死。
	wait, next, cooldown := nextReconnectDelay(4*time.Second, baseCooldown, false, maxConnErr, maxDur, baseCooldown)
	if wait != baseCooldown {
		t.Fatalf("max-conn wait = %v, want %v", wait, baseCooldown)
	}
	if next != fugleReconnectBaseDelay {
		t.Fatalf("max-conn nextBackoff = %v, want base %v", next, fugleReconnectBaseDelay)
	}
	if cooldown != 2*baseCooldown {
		t.Fatalf("max-conn nextCooldown = %v, want %v", cooldown, 2*baseCooldown)
	}

	// 連續再撞一次 max-connections：冷卻從 cooldown 再加倍（遞增），wait 用當前 cooldown。
	wait, _, cooldown = nextReconnectDelay(4*time.Second, cooldown, false, maxConnErr, maxDur, baseCooldown)
	if wait != 2*baseCooldown {
		t.Fatalf("escalated max-conn wait = %v, want %v", wait, 2*baseCooldown)
	}
	if cooldown != 4*baseCooldown {
		t.Fatalf("escalated nextCooldown = %v, want %v", cooldown, 4*baseCooldown)
	}

	// 遞增冷卻的 nextCooldown 不超過上限 fugleMaxConnCooldownCap。
	_, _, cooldown = nextReconnectDelay(4*time.Second, fugleMaxConnCooldownCap, false, maxConnErr, maxDur, baseCooldown)
	if cooldown != fugleMaxConnCooldownCap {
		t.Fatalf("capped nextCooldown = %v, want %v", cooldown, fugleMaxConnCooldownCap)
	}

	// 曾成功認證過才斷線（例如健康連線 1006 掉線）：回到 base 即時重連，且冷卻重置回 base
	// （清掉前一輪連續 maxconn 的遞增狀態）。
	wait, next, cooldown = nextReconnectDelay(32*time.Second, 4*baseCooldown, true, abnormalErr, maxDur, baseCooldown)
	if wait != fugleReconnectBaseDelay || next != fugleReconnectBaseDelay {
		t.Fatalf("authenticated reset = (%v,%v), want (%v,%v)", wait, next, fugleReconnectBaseDelay, fugleReconnectBaseDelay)
	}
	if cooldown != baseCooldown {
		t.Fatalf("authenticated cooldown reset = %v, want %v", cooldown, baseCooldown)
	}

	// 一般錯誤（dial/DNS 失敗且未認證）：用當下 backoff 當 wait，下一輪加倍，冷卻重置回 base。
	wait, next, cooldown = nextReconnectDelay(4*time.Second, 4*baseCooldown, false, normalErr, maxDur, baseCooldown)
	if wait != 4*time.Second {
		t.Fatalf("normal wait = %v, want 4s", wait)
	}
	if next != 8*time.Second {
		t.Fatalf("normal nextBackoff = %v, want 8s", next)
	}
	if cooldown != baseCooldown {
		t.Fatalf("normal cooldown reset = %v, want %v", cooldown, baseCooldown)
	}

	// 指數退避的 wait 與 nextBackoff 都不超過 maxDur。
	wait, next, _ = nextReconnectDelay(40*time.Second, baseCooldown, false, normalErr, maxDur, baseCooldown)
	if wait != 40*time.Second {
		t.Fatalf("capped wait = %v, want 40s", wait)
	}
	if next != maxDur {
		t.Fatalf("capped nextBackoff = %v, want %v", next, maxDur)
	}
}

// TestPingLoopSendsPing 驗證保活：pingLoop 會在間隔內主動送出 WebSocket ping 控制訊框，
// 降低被中介因閒置剪斷造成的 1006。
func TestPingLoopSendsPing(t *testing.T) {
	upgrader := websocket.Upgrader{}
	gotPing := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPingHandler(func(string) error {
			select {
			case gotPing <- struct{}{}:
			default:
			}
			return nil
		})
		// ReadMessage 會處理進來的 ping 控制訊框並觸發 PingHandler。
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	c := &FugleStreamClient{log: zap.NewNop(), pingInterval: 20 * time.Millisecond}
	done := make(chan struct{})
	go c.pingLoop(conn, done)
	defer close(done)

	select {
	case <-gotPing:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for ping frame from pingLoop")
	}
}

func TestIsMaxConnectionsErr(t *testing.T) {
	if !isMaxConnectionsErr(errors.New("... Maximum number of connections reached")) {
		t.Fatal("expected max-connections error to be detected")
	}
	if isMaxConnectionsErr(errors.New("some other error")) {
		t.Fatal("did not expect non-max-connections error to match")
	}
	if isMaxConnectionsErr(nil) {
		t.Fatal("nil error must not match")
	}
}

// TestGracefulCloseSendsCloseFrame 驗證修復的另一半：我方主動斷線時會送出
// WebSocket 正常關閉（Close 控制訊框），讓伺服器立即釋放名額，而不是只關 TCP
// 讓名額殘留。
func TestGracefulCloseSendsCloseFrame(t *testing.T) {
	upgrader := websocket.Upgrader{}
	gotClose := make(chan *websocket.CloseError, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				var ce *websocket.CloseError
				if errors.As(err, &ce) {
					gotClose <- ce
				} else {
					gotClose <- nil
				}
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	c := &FugleStreamClient{log: zap.NewNop()}
	c.gracefulClose(conn)

	select {
	case ce := <-gotClose:
		if ce == nil {
			t.Fatal("expected a websocket CloseError from graceful close, got a non-close read error")
		}
		if ce.Code != websocket.CloseNormalClosure {
			t.Fatalf("expected normal closure (1000), got close code %d", ce.Code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for close frame from graceful close")
	}
}
