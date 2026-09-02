package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// 這支驗的是**真實 Redis 上的 Lua compare-and-delete**。
//
// ⚠️ **預設跳過**：`backend/scripts/test.sh` 跑在沒有 Redis 的容器裡，
// 硬性依賴會讓整條驗證變成需要外部服務。要跑它用**專用腳本**：
//
//	scripts/test-redis-emission.sh
//
// 那支會起一個**拋棄式 Redis**（獨立 network、不掛 volume、跑完即刪）再設
// REDIS_TEST_ADDR 執行。
//
// ⛔ **不要把 REDIS_TEST_ADDR 指向 live 的 redis-redis-1／trading-net。**
// 這支測試會寫入 key，指到 live 就是拿正式環境當測試資料，違反 CLAUDE.md 的
// 環境隔離規則。**2026-09-02 的第一版註解就是這樣寫的，已修正。**
//
// ⛔ **也不要把它改成無條件執行**——那等於在單元測試裡加一個未經確認的外部相依，
// 正是 I-102 計畫書「測試接縫」那節要避免的。
func TestLuaCompareAndDeleteAgainstRealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR 未設定，跳過真實 Redis 整合測試")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := NewRedis(mustRedisConfig(addr))
	key := fmt.Sprintf("signal:emitted:test-%d", time.Now().UnixNano())
	defer c.ReleaseEmission(ctx, key, "cleanup")

	// A 取得保留
	if out := c.ReserveEmission(ctx, key, "token-A", 30*time.Second); out.Status != ReserveReserved {
		t.Fatalf("A 應取得保留，得到 %v (%v)", out.Status, out.Err)
	}
	// 同一個 key 再 reserve 應該失敗（SET NX）
	if out := c.ReserveEmission(ctx, key, "token-B", 30*time.Second); out.Status != ReserveAlreadyExists {
		t.Fatalf("第二次應回 already_exists，得到 %v (%v)", out.Status, out.Err)
	}

	// **關鍵**：拿別人的 token 來釋放，不得刪掉 A 的保留。
	if out := c.ReleaseEmission(ctx, key, "token-B"); out.Status != DeleteNotOwner {
		t.Fatalf("不同 token 應回 not_owner，得到 %v (%v)", out.Status, out.Err)
	}
	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("讀回 key 失敗：%v", err)
	}
	if got != "token-A" {
		t.Errorf("替代 token 被刪掉了：key=%q want token-A", got)
	}

	// 自己的 token 釋放得掉
	if out := c.ReleaseEmission(ctx, key, "token-A"); out.Status != DeleteDeleted {
		t.Fatalf("自己的 token 應刪得掉，得到 %v (%v)", out.Status, out.Err)
	}
	if got, _ := c.Get(ctx, key); got != "" {
		t.Errorf("釋放後 key 應消失，得到 %q", got)
	}
}
