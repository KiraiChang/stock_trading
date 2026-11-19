package database

import (
	"context"
	"testing"
)

// TestPostgresMigrationsTimestamptzOffsetIsSessionIndependent：python 的 backtrader 路徑把交易時間寫成帶 `+08:00`
// 的字串（issue.md I-135 的 D2）。這裡驗證它依賴的 PostgreSQL 行為：帶 offset 的字串寫進 TIMESTAMPTZ，
// 存下的時刻跟連線的 TimeZone 無關；⛔ 不帶時區的字串則會照連線的 TimeZone 解讀（原本的寫法因此依賴連線設定）。
//
// 名稱以 TestPostgresMigrations 開頭，才會被 scripts/test-postgres-migrations.sh 執行（它用 -test.run 篩選）。
// 用暫存表（同一條連線），⛔ 不動正式表。⚠️ MySQL 的 DATETIME 收到帶 offset 的值仍會換成連線的時區，
// 這項保證只限 PostgreSQL。
func TestPostgresMigrationsTimestamptzOffsetIsSessionIndependent(t *testing.T) {
	db := postgresTestDB(t)
	ctx := context.Background()
	conn, err := db.Connx(ctx)
	if err != nil {
		t.Fatalf("取得連線: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE tz_probe (zone TEXT, kind TEXT, ts TIMESTAMPTZ)`); err != nil {
		t.Fatalf("建暫存表: %v", err)
	}
	const withOffset = "2025-01-03T00:00:00+08:00"
	const naive = "2025-01-03T00:00:00"
	zones := []string{"UTC", "Asia/Taipei", "America/New_York"}
	for _, z := range zones {
		if _, err := conn.ExecContext(ctx, `SELECT set_config('TimeZone', $1, false)`, z); err != nil {
			t.Fatalf("設定 TimeZone %s: %v", z, err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO tz_probe VALUES ($1, 'offset', CAST($2 AS timestamptz)), ($1, 'naive', CAST($3 AS timestamptz))`,
			z, withOffset, naive); err != nil {
			t.Fatalf("寫入（TimeZone %s）: %v", z, err)
		}
	}

	epochs := func(kind string) map[string]int64 {
		rows, err := conn.QueryxContext(ctx, `SELECT zone, CAST(extract(epoch FROM ts) AS bigint) FROM tz_probe WHERE kind = $1`, kind)
		if err != nil {
			t.Fatalf("查詢: %v", err)
		}
		defer rows.Close()
		out := map[string]int64{}
		for rows.Next() {
			var z string
			var e int64
			if err := rows.Scan(&z, &e); err != nil {
				t.Fatal(err)
			}
			out[z] = e
		}
		return out
	}

	const want = int64(1735833600) // 2025-01-02T16:00:00Z
	for z, e := range epochs("offset") {
		if e != want {
			t.Errorf("帶 offset 的字串在 TimeZone %s 下存成 %d，預期 %d", z, e, want)
		}
	}
	naiveEpochs := epochs("naive")
	if naiveEpochs["UTC"] == naiveEpochs["Asia/Taipei"] || naiveEpochs["UTC"] == naiveEpochs["America/New_York"] {
		t.Errorf("不帶時區的字串應該依連線的 TimeZone 而不同，實際：%v", naiveEpochs)
	}
}
