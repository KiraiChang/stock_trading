package store

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// I-139：手動設定只新增、⛔ 不改寫；某一天目前的狀態＝最新一列。
func TestMarketDayOverrideRepo(t *testing.T) {
	ctx := context.Background()
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/overrides.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(ctx, db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	repo := NewMarketDayOverrideRepo(db)

	if got, err := repo.Latest(ctx, "2026-11-24"); err != nil || got != nil {
		t.Fatalf("沒有任何列時要回 nil, nil，得到 %+v, %v", got, err)
	}
	ins := func(date, action, reason string) uint64 {
		t.Helper()
		row, err := repo.Insert(ctx, MarketDayOverride{TradeDate: date, Action: action, Reason: reason, UserID: 7})
		if err != nil || row.ID == 0 {
			t.Fatalf("insert %s %s：%+v err=%v", date, action, row, err)
		}
		// 回傳的是這次寫入的那一列（含 DB 給的 id 與 created_at）
		if row.TradeDate != date || row.Action != action || row.Reason != reason || row.UserID != 7 || row.CreatedAt.IsZero() {
			t.Fatalf("Insert 要回傳這次寫入的整列，得到 %+v", row)
		}
		return row.ID
	}
	ins("2026-11-24", MarketDayClosed, "颱風全日休市")
	ins("2026-11-25", MarketDayOpen, "推翻日曆")
	last := ins("2026-11-24", MarketDayCleared, "誤設，取消")

	got, err := repo.Latest(ctx, "2026-11-24")
	if err != nil || got == nil {
		t.Fatalf("Latest：%+v, %v", got, err)
	}
	if got.ID != last || got.Action != MarketDayCleared || got.Effective() || got.UserID != 7 || got.Reason != "誤設，取消" || got.CreatedAt.IsZero() {
		t.Fatalf("最新一列要是取消那一列，得到 %+v", got)
	}
	if o, _ := repo.Latest(ctx, "2026-11-25"); !o.Effective() || o.Action != MarketDayOpen {
		t.Fatalf("11-25 要是生效中的手動開市，得到 %+v", o)
	}

	all, err := repo.ListSince(ctx, "2026-11-24")
	if err != nil || len(all) != 3 {
		t.Fatalf("ListSince 要列出全部三列（操作紀錄⛔ 不覆蓋），得到 %d 列、%v", len(all), err)
	}
	if all[0].Action != MarketDayClosed || all[1].Action != MarketDayCleared || all[2].TradeDate != "2026-11-25" {
		t.Fatalf("排序要依 trade_date、id：%+v", all)
	}
	if later, _ := repo.ListSince(ctx, "2026-11-25"); len(later) != 1 {
		t.Fatalf("from 之前的日期⛔ 不列，得到 %d 列", len(later))
	}

	// 值域與日期格式由 CHECK 擋住
	if _, err := repo.Insert(ctx, MarketDayOverride{TradeDate: "2026-11-26", Action: "maybe", Reason: "x", UserID: 1}); err == nil {
		t.Fatal("action 值域外要被 CHECK 擋下")
	}
	if _, err := repo.Insert(ctx, MarketDayOverride{TradeDate: "20261126", Action: MarketDayClosed, Reason: "x", UserID: 1}); err == nil {
		t.Fatal("trade_date 格式錯誤要被 CHECK 擋下")
	}
}
