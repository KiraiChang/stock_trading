package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// I-140：signals 的 timeframe 與 created_at（送出時間，由程式寫入、截到整秒），以及 B 規則找冷卻錨點的查詢。
// PostgreSQL 與 MySQL 的同一組案例在 internal/database 的 TestPostgresMigrations…／TestMySQLMigrations…SignalTimeframe。

func sigTF(s string) NullString {
	return NullString{NullString: sql.NullString{String: s, Valid: true}}
}

func TestSignalRepoTimeframeAndCreatedAt(t *testing.T) {
	ctx := context.Background()
	repo := NewSignalRepo(backtestRepoTestDB(t))
	base := time.Date(2031, 1, 6, 1, 1, 0, 0, time.UTC)
	ins := func(s Signal) Signal {
		t.Helper()
		if s.Direction == "" {
			s.Direction = "WATCH"
		}
		s.Price, s.Volume = 100, 1000
		if s.Timestamp.IsZero() {
			s.Timestamp = base
		}
		if err := repo.Insert(ctx, &s); err != nil {
			t.Fatalf("insert: %v", err)
		}
		return s
	}

	anchor := ins(Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 95.5, Timeframe: sigTF("1m"), CreatedAt: base.Add(700 * time.Millisecond)})
	if !anchor.CreatedAt.Equal(base) {
		t.Fatalf("Insert 要把 created_at 截到整秒：%v", anchor.CreatedAt)
	}
	// 之後寫入的：timeframe 是 NULL（I-140 之前的舊列）、1d、別的類型（超過 20 筆）——都⛔ 不能蓋過 1m 的錨點。
	ins(Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 96, CreatedAt: base.Add(time.Minute)})
	ins(Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 97, Timeframe: sigTF("1d"), CreatedAt: base.Add(2 * time.Minute)})
	for i := 0; i < 25; i++ {
		ins(Signal{Symbol: "2330", SignalType: "VOLUME_SPIKE", Timeframe: sigTF("1m"), CreatedAt: base.Add(3 * time.Minute)})
	}
	ins(Signal{Symbol: "2454", SignalType: "SUPPORT_BOUNCE", Timeframe: sigTF("1m"), CreatedAt: base.Add(4 * time.Minute)})

	got, err := repo.LatestBySymbolTypeTimeframe(ctx, "2330", "SUPPORT_BOUNCE", "1m")
	if err != nil || got == nil {
		t.Fatalf("要找到 1m 的錨點：%+v %v", got, err)
	}
	if got.ID != anchor.ID || !got.CreatedAt.Equal(base) || got.Timeframe.String != "1m" || got.Support != 95.5 {
		t.Fatalf("錨點要是那一列、created_at 讀回整秒：%+v", got)
	}
	if none, err := repo.LatestBySymbolTypeTimeframe(ctx, "0050", "SUPPORT_BOUNCE", "1m"); err != nil || none != nil {
		t.Fatalf("沒有列時回 nil：%+v %v", none, err)
	}

	// 同一檔後來又寫一筆 1m → 錨點換成新的那一列（依 id）。
	later := ins(Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 94, Timeframe: sigTF("1m"), CreatedAt: base.Add(15 * time.Minute)})
	if got, _ := repo.LatestBySymbolTypeTimeframe(ctx, "2330", "SUPPORT_BOUNCE", "1m"); got == nil || got.ID != later.ID {
		t.Fatalf("要取最新寫入的一列：%+v", got)
	}

	// 複審 P2-2：寫入順序跟 created_at 相反時，錨點依 created_at（⛔ 不依 id）。
	// 較晚寫入的那筆用 +08:00 表示：沒轉 UTC 的話 SQLite 文字比較會把 "…09:04…+08:00" 排在 "…01:20…+00:00" 後面。
	tpe := time.FixedZone("TPE", 8*3600)
	newer := ins(Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: sigTF("1m"), CreatedAt: base.Add(19 * time.Minute)})
	older := ins(Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: sigTF("1m"), CreatedAt: base.Add(3 * time.Minute).In(tpe)})
	if got, err := repo.LatestBySymbolTypeTimeframe(ctx, "1101", "SUPPORT_BOUNCE", "1m"); err != nil || got == nil || got.ID != newer.ID {
		t.Fatalf("錨點要是 created_at 最新的 %d（⛔ 不是後寫入的 %d）：%+v %v", newer.ID, older.ID, got, err)
	}
	// 同一秒：取較晚寫入的（id 大）。
	tie := ins(Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: sigTF("1m"), CreatedAt: base.Add(19*time.Minute + 300*time.Millisecond)})
	if got, _ := repo.LatestBySymbolTypeTimeframe(ctx, "1101", "SUPPORT_BOUNCE", "1m"); got == nil || got.ID != tie.ID {
		t.Fatalf("同一秒要取 id 大的 %d：%+v", tie.ID, got)
	}

	// 既有查詢也帶出 timeframe 與 created_at；NULL 讀回 Valid=false。
	rows, err := repo.GetBySymbol(ctx, "2330", 50)
	if err != nil {
		t.Fatal(err)
	}
	seenNull := false
	for _, r := range rows {
		if r.CreatedAt.IsZero() {
			t.Fatalf("created_at 要讀得回來：%+v", r)
		}
		if !r.Timeframe.Valid {
			seenNull = true
		}
	}
	if !seenNull {
		t.Fatal("timeframe 是 NULL 的列要讀回 Valid=false")
	}

	// 沒填 created_at 的呼叫端（測試、工具）：用現在、截到整秒。
	s := ins(Signal{Symbol: "3008", SignalType: "VOLUME_SPIKE"})
	if s.CreatedAt.IsZero() || s.CreatedAt.Nanosecond() != 0 {
		t.Fatalf("沒填時要補現在、截到整秒：%v", s.CreatedAt)
	}
}
