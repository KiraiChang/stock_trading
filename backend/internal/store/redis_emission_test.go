package store

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/trading/backend/internal/config"
)

// 這一組守的是三個 Redis 操作的 status／cause 不變量（見 docs/architecture.md「寫入失敗的一致性契約」）。
//
// **Redis 停用（rdb == nil）與退避是最重要的兩格**：既有方法把它們都折成 nil，
// 而設計上「停用不算 degraded、READONLY 算」——不分開就落實不了。

func disabledClient() *RedisClient { return &RedisClient{rdb: nil} }

// mustRedisConfig 建一個指向「沒有人在聽」的位址的設定——用來驗錯誤路徑，
// 不需要真的跑起一個 Redis。
func mustRedisConfig(addr string) config.RedisConfig {
	return config.RedisConfig{Addr: addr}
}

func TestDisabledRedisIsNotAFailure(t *testing.T) {
	ctx := context.Background()
	c := disabledClient()

	t.Run("reserve", func(t *testing.T) {
		out := c.ReserveEmission(ctx, "k", "tok", time.Minute)
		if out.Status != ReserveDisabled {
			t.Errorf("status = %v, want ReserveDisabled", out.Status)
		}
		if out.Err != nil {
			t.Errorf("disabled 不得帶 Err，得到 %v", out.Err)
		}
	})
	t.Run("enqueue", func(t *testing.T) {
		out := c.EnqueueSignal(ctx, "q", map[string]int{"a": 1})
		if out.Status != EnqueueDisabled {
			t.Errorf("status = %v, want EnqueueDisabled", out.Status)
		}
		if out.Err != nil {
			t.Errorf("disabled 不得帶 Err，得到 %v", out.Err)
		}
	})
	t.Run("release", func(t *testing.T) {
		out := c.ReleaseEmission(ctx, "k", "tok")
		if out.Status != DeleteDisabled {
			t.Errorf("status = %v, want DeleteDisabled", out.Status)
		}
		if out.Err != nil {
			t.Errorf("disabled 不得帶 Err，得到 %v", out.Err)
		}
	})
}

// TestBackoffCarriesSentinel 確認退避一定帶得出可分類的 cause。
//
// ⚠️ skipWrite() 只回布林、沒有底層 error；沒有 sentinel 的話錯誤分類器
// 產不出 readonly，stage_errors 也沒有東西可存。
func TestBackoffCarriesSentinel(t *testing.T) {
	ctx := context.Background()
	// 用一個位址故意打不通的 client，並直接把退避時間往前設。
	c := NewRedis(mustRedisConfig("127.0.0.1:1"))
	c.noteReadOnly()

	t.Run("reserve", func(t *testing.T) {
		out := c.ReserveEmission(ctx, "k", "tok", time.Minute)
		if out.Status != ReserveBackoff {
			t.Fatalf("status = %v, want ReserveBackoff", out.Status)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("backoff 必須帶 ErrRedisWriteBackoff sentinel，得到 %v", out.Err)
		}
	})
	t.Run("enqueue", func(t *testing.T) {
		out := c.EnqueueSignal(ctx, "q", map[string]int{"a": 1})
		if out.Status != EnqueueBackoff {
			t.Fatalf("status = %v, want EnqueueBackoff", out.Status)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("backoff 必須帶 sentinel，得到 %v", out.Err)
		}
	})
	t.Run("release", func(t *testing.T) {
		out := c.ReleaseEmission(ctx, "k", "tok")
		if out.Status != DeleteBackoff {
			t.Fatalf("status = %v, want DeleteBackoff", out.Status)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("backoff 必須帶 sentinel，得到 %v", out.Err)
		}
	})
}

// TestErrorStatusCarriesCause：連不上時必須回 error ＋ 非空 cause，
// **不能像既有方法那樣回 nil**。
func TestErrorStatusCarriesCause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c := NewRedis(mustRedisConfig("127.0.0.1:1")) // 沒有人在聽

	out := c.ReserveEmission(ctx, "k", "tok", time.Minute)
	if out.Status != ReserveError {
		t.Fatalf("status = %v, want ReserveError", out.Status)
	}
	if out.Err == nil {
		t.Error("error 狀態必須帶非空 cause，供 stage_errors 與內部 log 使用")
	}
}

// TestFirstReadOnlyErrorReturnsBackoffNotSuccess 是完成條件 5-10 的核心：
// **第一次**收到 READONLY 就必須回 backoff，不能像既有 handleWriteErr 那樣
// 設完退避就 return nil（等於把那次失敗當成成功）。
func TestFirstReadOnlyErrorReturnsBackoffNotSuccess(t *testing.T) {
	c := &RedisClient{}

	if c.skipWrite() {
		t.Fatal("前提：一開始不該在退避中")
	}
	if !c.classifyWriteErr(errors.New("READONLY You can't write against a read only replica.")) {
		t.Fatal("第一次收到 READONLY 就要回 backoff（而不是被吞成成功）")
	}
	if !c.skipWrite() {
		t.Error("第一次 READONLY 之後應開始退避")
	}

	// 對照組：一般錯誤不是 backoff，也不該啟動退避。
	c2 := &RedisClient{}
	if c2.classifyWriteErr(errors.New("dial tcp: connection refused")) {
		t.Error("一般錯誤不該被當成 backoff")
	}
	if c2.skipWrite() {
		t.Error("一般錯誤不該啟動退避")
	}
}

// TestAllThreeOpsCarryCauseOnError 補齊完成條件 5-14：
// **三個操作**的 error 狀態都要帶非空 cause，不只 ReserveEmission。
func TestAllThreeOpsCarryCauseOnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Run("enqueue", func(t *testing.T) {
		c := NewRedis(mustRedisConfig("127.0.0.1:1"))
		out := c.EnqueueSignal(ctx, "q", map[string]int{"a": 1})
		if out.Status != EnqueueError {
			t.Fatalf("status = %v, want EnqueueError", out.Status)
		}
		if out.Err == nil {
			t.Error("error 狀態必須帶非空 cause")
		}
	})

	t.Run("release", func(t *testing.T) {
		c := NewRedis(mustRedisConfig("127.0.0.1:1"))
		out := c.ReleaseEmission(ctx, "k", "tok")
		if out.Status != DeleteError {
			t.Fatalf("status = %v, want DeleteError", out.Status)
		}
		if out.Err == nil {
			t.Error("error 狀態必須帶非空 cause")
		}
	})
}

// ── operation-level 的第一次 READONLY ────────────────────────────
//
// ⚠️ **只測 classifyWriteErr 抓不到呼叫點漏接**——三個操作現在都接上了，
// 但那是靠人看出來的。這裡用一個**假的 RESP server**，讓 EnqueueSignal 與
// ReserveEmission 真的從連線收到 READONLY，把「有沒有接上」也釘住。
//
// 假 server 不需要外部服務：對任何指令一律回 `-READONLY ...`。

func startReadOnlyRedis(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
					// 對任何指令都回 READONLY——包含 go-redis 開場的 HELLO。
					if _, err := c.Write([]byte("-READONLY You can't write against a read only replica.\r\n")); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestOperationsReturnBackoffOnFirstReadOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := startReadOnlyRedis(t)

	t.Run("EnqueueSignal", func(t *testing.T) {
		c := NewRedis(mustRedisConfig(addr))
		out := c.EnqueueSignal(ctx, "q", map[string]int{"a": 1})
		if out.Status != EnqueueBackoff {
			t.Fatalf("第一次 READONLY 應回 EnqueueBackoff，得到 %v (%v)", out.Status, out.Err)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("cause 要能用 errors.Is 辨識，得到 %v", out.Err)
		}
		if !c.skipWrite() {
			t.Error("第一次 READONLY 之後應開始退避")
		}
	})

	t.Run("ReserveEmission", func(t *testing.T) {
		c := NewRedis(mustRedisConfig(addr))
		out := c.ReserveEmission(ctx, "k", "tok", time.Minute)
		if out.Status != ReserveBackoff {
			t.Fatalf("第一次 READONLY 應回 ReserveBackoff，得到 %v (%v)", out.Status, out.Err)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("cause 要能用 errors.Is 辨識，得到 %v", out.Err)
		}
	})

	t.Run("ReleaseEmission", func(t *testing.T) {
		c := NewRedis(mustRedisConfig(addr))
		out := c.ReleaseEmission(ctx, "k", "tok")
		if out.Status != DeleteBackoff {
			t.Fatalf("第一次 READONLY 應回 DeleteBackoff，得到 %v (%v)", out.Status, out.Err)
		}
		if !errors.Is(out.Err, ErrRedisWriteBackoff) {
			t.Errorf("cause 要能用 errors.Is 辨識，得到 %v", out.Err)
		}
	})
}
