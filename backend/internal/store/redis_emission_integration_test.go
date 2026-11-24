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
// 正是 docs/architecture.md「寫入失敗的一致性契約」 要避免的（原記於 issue.md I-102 的「測試接縫」，已收斂）。
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

// TestLuaReserveUntilAgainstRealRedis 驗 B 規則的保留（I-140）在真實 Redis 上的四種情況、連線延遲與重試。
// 一樣只由 scripts/test-redis-emission.sh 執行（⛔ 不指向 live）。
func TestLuaReserveUntilAgainstRealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR 未設定，跳過真實 Redis 整合測試")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := NewRedis(mustRedisConfig(addr))
	key := fmt.Sprintf("signal:emitted:test-until-%d", time.Now().UnixNano())

	now := time.Now().Truncate(time.Second)
	exp := now.Add(time.Second) // 到期時刻：1 秒後（整秒）
	defer c.ReleaseEmission(ctx, key, EmissionUntilValue("token-A", exp))

	// 1 鍵不存在 → 取得
	if out := c.ReserveEmissionUntil(ctx, key, "token-A", now, exp); out.Status != ReserveReserved {
		t.Fatalf("A 應取得：%v (%v)", out.Status, out.Err)
	}
	// 2 同一 token 重試（回應遺失後客戶端重送）→ 已取得、值與到期時刻⛔ 不變
	if out := c.ReserveEmissionUntil(ctx, key, "token-A", now.Add(300*time.Millisecond), exp.Add(time.Hour)); out.Status != ReserveReserved {
		t.Fatalf("同一 token 重試要回已取得：%v (%v)", out.Status, out.Err)
	}
	if got, _ := c.Get(ctx, key); got != EmissionUntilValue("token-A", exp) {
		t.Fatalf("重試⛔ 不能改值或到期時刻：%q", got)
	}
	// 3 其他 token、now 早於值裡的到期時刻 → already_exists
	if out := c.ReserveEmissionUntil(ctx, key, "token-B", exp.Add(-time.Millisecond), exp.Add(15*time.Minute)); out.Status != ReserveAlreadyExists {
		t.Fatalf("到期前其他 token 要失敗：%v (%v)", out.Status, out.Err)
	}
	// 連線延遲：等到 Redis 的牆上時間已經過了到期時刻（鍵靠寬限還在），候選在到期前 0.5 秒取樣——
	// 仍要 already_exists（判斷用傳入的 now 與值裡的到期時刻，⛔ 不受 Redis 時鐘影響）。
	time.Sleep(time.Until(exp.Add(1500 * time.Millisecond)))
	if out := c.ReserveEmissionUntil(ctx, key, "token-B", exp.Add(-500*time.Millisecond), exp.Add(15*time.Minute)); out.Status != ReserveAlreadyExists {
		t.Fatalf("到期前 0.5 秒取樣、指令晚到 2 秒，仍要抑制：%v (%v)", out.Status, out.Err)
	}
	// 拿 token 本身（⛔ 不是完整的值）釋放不掉。
	if out := c.ReleaseEmission(ctx, key, "token-A"); out.Status != DeleteNotOwner {
		t.Fatalf("compare-and-delete 比的是整個值：%v", out.Status)
	}
	// 4 其他 token、now ≥ 值裡的到期時刻 → 覆寫成功
	exp2 := exp.Add(15 * time.Minute)
	if out := c.ReserveEmissionUntil(ctx, key, "token-B", exp, exp2); out.Status != ReserveReserved {
		t.Fatalf("到期後其他 token 要取得：%v (%v)", out.Status, out.Err)
	}
	if got, _ := c.Get(ctx, key); got != EmissionUntilValue("token-B", exp2) {
		t.Fatalf("要覆寫成新的值：%q", got)
	}
	// 鍵的實際存活期＝到期＋寬限
	if ttl, err := c.rdb.PTTL(ctx, key).Result(); err != nil || ttl < 15*time.Minute || ttl > 15*time.Minute+EmissionUntilGrace+time.Second {
		t.Fatalf("存活期要約等於到期＋寬限：%v %v", ttl, err)
	}
	if out := c.ReleaseEmission(ctx, key, EmissionUntilValue("token-B", exp2)); out.Status != DeleteDeleted {
		t.Fatalf("用完整的值要刪得掉：%v", out.Status)
	}
}
