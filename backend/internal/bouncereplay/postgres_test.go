package bouncereplay

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// TestPostgresBounceReplay 在真的 PostgreSQL 上走一遍：來源庫的讀取（Load 的 SQL）、外部訊號的原樣注入（created_at 保留微秒）、
// NUMERIC 的精度與完整重播。只由 scripts/test-bouncereplay-pg.sh 執行（拋棄式 postgres），⛔ 不指向 dev 或 live。
func TestPostgresBounceReplay(t *testing.T) {
	dsn := os.Getenv("BOUNCEREPLAY_PG_DSN")
	if dsn == "" {
		t.Skip("BOUNCEREPLAY_PG_DSN 未設定（用 scripts/test-bouncereplay-pg.sh）")
	}
	ctx := context.Background()
	admin, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	open := func(name string) *sqlx.DB {
		if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(dsn)
		u.Path = "/" + name
		db, err := sqlx.Connect("pgx", u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		if err := database.RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
			t.Fatal(err)
		}
		return db
	}
	src, dbA, dbB := open("i140_t_src"), open("i140_t_a"), open("i140_t_b")

	live := simulateLive(t, []string{"2330", "0050"}, 9, fixedTiming)
	daily := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC)
	live.Signals = append(live.Signals, LiveSignal{ID: 999, Symbol: "2330", SignalType: "BREAKDOWN", Direction: "SELL", Price: 99,
		Support: 99.5, Strength: 1, TS: daily, CreatedAt: base.Add(-10*time.Minute + 123456*time.Microsecond)}) // 08:50 的日 K（起始歷史）
	sortSignals(live)

	// 寫進來源庫：K 棒、逐輪存證、signals（created_at 照原樣）。
	cr := store.NewCandleRepo(src)
	for _, cs := range live.Candles {
		if err := cr.BulkInsert(ctx, cs); err != nil {
			t.Fatal(err)
		}
	}
	rounds := map[uint64]bool{}
	for _, e := range live.Evals {
		if !rounds[e.RunID] {
			rounds[e.RunID] = true
			if _, err := src.ExecContext(ctx, `INSERT INTO intraday_rounds (run_id, source, scheduled_at, outcome) VALUES ($1, 'scheduled', $2, 'executed')`,
				e.RunID, e.LastBarTS); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := src.ExecContext(ctx, `INSERT INTO intraday_symbol_runs
			(run_id, symbol, source, data_at, data_status, last_bar_ts, fingerprint, eval_at, eval_status)
			VALUES ($1, $2, 'scheduled', $3, 'written', $4, $5, $6, $7)`,
			e.RunID, e.Symbol, e.DataAt, e.LastBarTS, e.Fingerprint, e.EvalAt, e.EvalStatus); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range live.Signals {
		if err := insertRaw(ctx, src, s); err != nil {
			t.Fatal(err)
		}
	}

	in, err := Load(ctx, src, LoadOptions{From: base, To: base.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Evals) != len(live.Evals) || len(in.Signals) != len(live.Signals) || len(in.Candles) != 2 {
		t.Fatalf("Load：evals %d/%d signals %d/%d candles %d", len(in.Evals), len(live.Evals), len(in.Signals), len(live.Signals), len(in.Candles))
	}
	if !in.Signals[0].CreatedAt.Equal(base.Add(-10*time.Minute + 123456*time.Microsecond)) {
		t.Fatalf("created_at 要保留微秒：%v", in.Signals[0].CreatedAt)
	}
	in.ExpectedDays = []string{"2026-11-23"}
	if pre := Precheck(in); pre.Total.InputDiffs != 0 || len(pre.Short) != 0 || !pre.OK() {
		t.Fatalf("從 PostgreSQL 讀回的 K 棒算出的指紋要跟 live 相同：%+v", pre)
	}

	r := NewRunner(in, dbA, dbB, zap.NewNop())
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if r.attr.origin[in.Signals[0].ID] != OriginHistory {
		t.Fatalf("08:50 的日 K 是起始歷史：%v", r.attr.origin)
	}
	for _, x := range r.Records {
		// 09:00、09:05 在日 K 送出之後 15 分鐘內（保留鎖驗不了）→ 未確認；其餘都要可對帳且一致。
		if x.Symbol == "2330" && x.DataAt.Before(base.Add(5*time.Minute+time.Second)) {
			if x.Group != GroupUnconfirmed {
				t.Fatalf("run %d：日 K 送出後 15 分鐘內要未確認：%s", x.RunID, x.Group)
			}
			continue
		}
		if x.Group != GroupReconcilable || x.AMatch != MatchOK {
			t.Fatalf("run %d %s：group=%s（%s）match=%s（%s）", x.RunID, x.Symbol, x.Group, x.GroupReason, x.AMatch, x.MatchDetail)
		}
	}
	// 補充重播的讀取（LoadFixed 的 SQL）：自選清單的每一檔（2454 沒有 K 棒也留著）、暖身的 K 棒與同一份 signals；清單外有 1 分 K 的列成 Unlisted。
	fin, err := LoadFixed(ctx, src, LoadOptions{From: base, To: base.Add(24 * time.Hour)}, oldWatch("0050", "2330", "2454"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.Candles) != 3 || len(fin.Candles["2454"]) != 0 || len(fin.Candles["2330"]) != len(in.Candles["2330"]) || len(fin.Signals) != len(in.Signals) || len(fin.Unlisted) != 0 {
		t.Fatalf("LoadFixed：candles %d（2330 %d/%d）signals %d/%d unlisted %v", len(fin.Candles), len(fin.Candles["2330"]), len(in.Candles["2330"]), len(fin.Signals), len(in.Signals), fin.Unlisted)
	}
	fin2, err := LoadFixed(ctx, src, LoadOptions{From: base, To: base.Add(24 * time.Hour)}, oldWatch("2330"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fin2.Unlisted) != 1 || fin2.Unlisted[0] != "0050" || len(fin2.Candles) != 1 {
		t.Fatalf("清單外有 1 分 K 的要列成 Unlisted：%v %d", fin2.Unlisted, len(fin2.Candles))
	}
	for _, x := range fin2.Signals {
		if x.Symbol != "2330" {
			t.Fatalf("signals 只讀清單內的：%+v", x)
		}
	}

	var got time.Time
	if err := dbA.GetContext(ctx, &got, `SELECT created_at FROM signals WHERE signal_type = 'BREAKDOWN'`); err != nil {
		t.Fatal(err)
	}
	if !got.Equal(base.Add(-10*time.Minute + 123456*time.Microsecond)) {
		t.Fatalf("注入重播庫的 created_at 要保留原始精度：%v", got)
	}
}
