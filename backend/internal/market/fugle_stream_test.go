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
	maxConnErr := errors.New("fugle ws read (auth) failed: websocket: close 1001 (going away): Maximum number of connections reached")
	normalErr := errors.New("fugle ws dial failed: dial tcp: lookup api.fugle.tw: i/o timeout")
	abnormalErr := errors.New("fugle ws read failed: websocket: close 1006 (abnormal closure): unexpected EOF")

	// max-connections：套用固定長冷卻，並把退避基準重置回 base（冷卻後恢復即時反應）。
	// 這是本次修復的核心——名額被前一條 1006 幽靈連線佔著時，不能每 2s 硬撞。
	wait, next := nextReconnectDelay(4*time.Second, false, maxConnErr, maxDur)
	if wait != fugleMaxConnCooldown {
		t.Fatalf("max-conn wait = %v, want %v", wait, fugleMaxConnCooldown)
	}
	if next != fugleReconnectBaseDelay {
		t.Fatalf("max-conn nextBackoff = %v, want base %v", next, fugleReconnectBaseDelay)
	}

	// 曾成功認證過才斷線（例如健康連線 1006 掉線）：回到 base，維持斷線後即時重連。
	wait, next = nextReconnectDelay(32*time.Second, true, abnormalErr, maxDur)
	if wait != fugleReconnectBaseDelay || next != fugleReconnectBaseDelay {
		t.Fatalf("authenticated reset = (%v,%v), want (%v,%v)", wait, next, fugleReconnectBaseDelay, fugleReconnectBaseDelay)
	}

	// 一般錯誤（dial/DNS 失敗且未認證）：用當下 backoff 當 wait，下一輪加倍。
	wait, next = nextReconnectDelay(4*time.Second, false, normalErr, maxDur)
	if wait != 4*time.Second {
		t.Fatalf("normal wait = %v, want 4s", wait)
	}
	if next != 8*time.Second {
		t.Fatalf("normal nextBackoff = %v, want 8s", next)
	}

	// 指數退避的 wait 與 nextBackoff 都不超過 maxDur。
	wait, next = nextReconnectDelay(40*time.Second, false, normalErr, maxDur)
	if wait != 40*time.Second {
		t.Fatalf("capped wait = %v, want 40s", wait)
	}
	if next != maxDur {
		t.Fatalf("capped nextBackoff = %v, want %v", next, maxDur)
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
