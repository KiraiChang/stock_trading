// bouncereplay 是 I-140 的 A／B 離線重播（docs/plans/i140-intraday-bounce-dedup.md 第 7.3 節）。
//
// ⛔ 只在 dev 使用，⛔ 不要指向 live：三個資料庫的名稱都必須以 i140_ 開頭（隔離的 dev PostgreSQL 裡另建的 database），
// 否則拒絕執行。來源庫只讀；兩個重播庫會先跑 migration，signals 必須是空的。
//
//	bouncereplay -mode precheck -src "$SRC" -from 2026-11-23 -to 2026-11-27 -expected-days DAYS -out DIR
//	bouncereplay -mode replay   -src "$SRC" -a "$A" -b "$B" -from 2026-11-23 -to 2026-11-27 -expected-days DAYS -out DIR
//	bouncereplay -mode fixed    -src "$SRC" -a "$A" -b "$B" -from 2026-11-17 -to 2026-11-20 -expected-days DAYS \
//	                            -watchlist WATCHLIST_CSV -out DIR
//
// fixed 是補充重播：窗口⛔ 沒有逐輪存證，每天固定 55 輪（09:00～13:30 每 5 分鐘）、分母（自選清單裡當天 09:00 之前已加入的）
// 每一檔都評估，保真度較低、⛔ 不對帳、⛔ 不裁決；結束碼 0＝跑完而且完整、3＝缺資料（分母裡任一檔整日缺 1 分 K）、
// 比較不完整（A 或 B 有評估失敗、降級、讀取失敗，或行情不足 A／B 不一致）或自我檢查失敗。
//
// DAYS 是交接的預定交易日清單（一行一個 YYYY-MM-DD）。
//
// 結束碼：0＝通過（覆蓋率達到門檻、對帳⛔ 沒有原因不明的不一致、B 的回歸檢查都通過）；2＝參數或安全檢查不通過；
// 3＝預檢的上限達不到門檻（⛔ 不進入正式重播）、自我檢查失敗、覆蓋率沒有達到門檻、原因不明的不一致，或 B 的回歸檢查失敗；1＝其他錯誤。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/bouncereplay"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/pkg/timeutil"
)

func main() { os.Exit(run()) }

func run() int {
	mode := flag.String("mode", "precheck", "precheck、replay 或 fixed（補充重播）")
	srcDSN := flag.String("src", "", "來源庫 DSN（live 交接資料，唯讀）")
	aDSN := flag.String("a", "", "A 的重播庫 DSN")
	bDSN := flag.String("b", "", "B 的重播庫 DSN")
	from := flag.String("from", "", "窗口第一天（台北，YYYY-MM-DD）")
	to := flag.String("to", "", "窗口最後一天（台北，含）")
	out := flag.String("out", "", "輸出目錄（報告與逐筆 CSV）")
	daysFile := flag.String("expected-days", "", "預定交易日清單（一行一個 YYYY-MM-DD；依它檢查完整性）")
	watchFile := flag.String("watchlist", "", "fixed：交接的自選清單（watchlist.csv，表頭 symbol,added_at；每天的分母）")
	barLag := flag.Duration("bar-lag", bouncereplay.DefaultBarLag, "fixed：每一輪只用 ts ≤ 輪次 − bar-lag 的 1 分 K")
	flag.Parse()

	fail := func(code int, f string, a ...any) int {
		fmt.Fprintf(os.Stderr, "bouncereplay: "+f+"\n", a...)
		return code
	}
	f, err1 := time.ParseInLocation("2006-01-02", *from, timeutil.TaipeiTZ)
	t, err2 := time.ParseInLocation("2006-01-02", *to, timeutil.TaipeiTZ)
	if err1 != nil || err2 != nil || t.Before(f) || *srcDSN == "" || *out == "" || *daysFile == "" || (*mode != "precheck" && *mode != "replay" && *mode != "fixed") || *barLag < 0 {
		return fail(2, "參數不對（-mode precheck|replay|fixed -src -from -to -expected-days -out，replay 與 fixed 另要 -a -b）")
	}
	df, err := os.Open(*daysFile)
	if err != nil {
		return fail(2, "預定交易日清單：%v", err)
	}
	expected, err := bouncereplay.ParseExpectedDays(df, f, t)
	df.Close()
	if err != nil {
		return fail(2, "%v", err)
	}
	var watch []bouncereplay.WatchEntry
	if *mode == "fixed" {
		if *watchFile == "" {
			return fail(2, "fixed 要 -watchlist")
		}
		wf, err := os.Open(*watchFile)
		if err != nil {
			return fail(2, "自選清單：%v", err)
		}
		watch, err = bouncereplay.ParseWatchlist(wf)
		wf.Close()
		if err != nil {
			return fail(2, "%v", err)
		}
	}
	if *mode != "precheck" && (*aDSN == "" || *bDSN == "") {
		return fail(2, "%s 要 -a 與 -b", *mode)
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return fail(1, "建立輸出目錄：%v", err)
	}
	ctx := context.Background()
	log := zap.NewNop()

	open := func(dsn string) (*sqlx.DB, string, error) {
		db, err := sqlx.Connect("pgx", dsn)
		if err != nil {
			return nil, "", fmt.Errorf("連線失敗") // ⛔ 不印 DSN
		}
		var name string
		if err := db.GetContext(ctx, &name, "SELECT current_database()"); err != nil {
			db.Close()
			return nil, "", err
		}
		if err := bouncereplay.CheckDatabaseName(name); err != nil {
			db.Close()
			return nil, name, err
		}
		return db, name, nil
	}
	src, srcName, err := open(*srcDSN)
	if err != nil {
		return fail(2, "來源庫：%v", err)
	}
	defer src.Close()
	if _, err := src.ExecContext(ctx, "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY"); err != nil {
		return fail(1, "來源庫設成唯讀：%v", err)
	}

	if *mode == "fixed" {
		return runFixed(ctx, src, srcName, f, t, expected, watch, *barLag, *aDSN, *bDSN, *out, open, fail, log)
	}

	in, err := bouncereplay.Load(ctx, src, bouncereplay.LoadOptions{From: f, To: t.AddDate(0, 0, 1)})
	if err != nil {
		return fail(1, "讀取輸入：%v", err)
	}
	in.ExpectedDays = expected
	fmt.Printf("來源 %s：評估 %d 筆、訊號 %d 筆、%d 檔\n", srcName, len(in.Evals), len(in.Signals), len(in.Candles))

	pre := bouncereplay.Precheck(in)
	pf, err := os.Create(filepath.Join(*out, "precheck.md"))
	if err != nil {
		return fail(1, "%v", err)
	}
	ok := bouncereplay.WritePrecheck(pf, pre) && pre.OK()
	pf.Close()
	if *mode == "precheck" || !ok {
		fmt.Println("預檢完成：" + filepath.Join(*out, "precheck.md"))
		if !ok {
			// 覆蓋率的上限就達不到門檻：⛔ 不進入正式重播（第 7.3 節：停下來回報，由使用者裁決 a／b／c）。
			for _, x := range pre.Failures {
				fmt.Println("⚠️ " + x)
			}
			fmt.Println("預檢不通過（完整性或覆蓋率上限），⛔ 不進入正式重播")
			return 3
		}
		return 0
	}

	var dbs []*sqlx.DB
	for _, d := range []string{*aDSN, *bDSN} {
		db, name, err := open(d)
		if err != nil {
			return fail(2, "重播庫：%v", err)
		}
		defer db.Close()
		if name == srcName {
			return fail(2, "重播庫不能跟來源庫相同（%s）", name)
		}
		if err := database.RunMigrations(ctx, db, "postgres", log); err != nil {
			return fail(1, "重播庫 %s migration：%v", name, err)
		}
		var n int
		if err := db.GetContext(ctx, &n, "SELECT count(*) FROM signals"); err != nil || n != 0 {
			return fail(2, "重播庫 %s 的 signals 不是空的（%d 筆）：每次重播要用新建的 database", name, n)
		}
		dbs = append(dbs, db)
	}
	var aName, bName string
	_ = dbs[0].GetContext(ctx, &aName, "SELECT current_database()")
	_ = dbs[1].GetContext(ctx, &bName, "SELECT current_database()")
	if aName == bName {
		return fail(2, "A、B 不能用同一個重播庫")
	}

	var checks []string
	for _, p := range []struct {
		name string
		pol  signal.DedupPolicy
	}{{"A", signal.DedupPriceKey}, {"B", signal.DedupIntradayBounceBySymbol}} {
		desc, err := bouncereplay.SelfCheck(ctx, in, p.pol, log)
		if err != nil {
			return fail(3, "%s：%v（%s）", p.name, err, desc)
		}
		checks = append(checks, fmt.Sprintf("%s 通過（第一次送出有廣播、1 分鐘後被本機保留鎖擋下；%s）", p.name, desc))
	}

	r := bouncereplay.NewRunner(in, dbs[0], dbs[1], log)
	if err := r.Run(ctx); err != nil {
		return fail(1, "重播：%v", err)
	}
	rf, err := os.Create(filepath.Join(*out, "report.md"))
	if err != nil {
		return fail(1, "%v", err)
	}
	sum := bouncereplay.WriteReport(rf, r.Records, in.ExpectedDays, r.Notes, checks)
	rf.Close()
	cf, err := os.Create(filepath.Join(*out, "records.csv"))
	if err != nil {
		return fail(1, "%v", err)
	}
	if err := bouncereplay.WriteCSV(cf, r.Records); err != nil {
		return fail(1, "%v", err)
	}
	cf.Close()
	fmt.Println("重播完成：" + filepath.Join(*out, "report.md"))
	if !sum.Pass {
		for _, f := range sum.Failures {
			fmt.Println("⛔ 不通過：" + f)
		}
		return 3
	}
	return 0
}

// openReplayDBs 打開 A、B 兩個重播庫：名稱要 i140_ 開頭、⛔ 不能跟來源庫或彼此相同、先跑 migration、signals 要是空的。
func openReplayDBs(ctx context.Context, srcName string, dsns []string, open func(string) (*sqlx.DB, string, error)) ([]*sqlx.DB, int, error) {
	var dbs []*sqlx.DB
	names := map[string]bool{srcName: true}
	for _, d := range dsns {
		db, name, err := open(d)
		if err != nil {
			return dbs, 2, fmt.Errorf("重播庫：%v", err)
		}
		dbs = append(dbs, db)
		if names[name] {
			return dbs, 2, fmt.Errorf("重播庫 %s 跟來源庫或另一個重播庫相同", name)
		}
		names[name] = true
		if err := database.RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
			return dbs, 1, fmt.Errorf("重播庫 %s migration：%v", name, err)
		}
		var n int
		if err := db.GetContext(ctx, &n, "SELECT count(*) FROM signals"); err != nil || n != 0 {
			return dbs, 2, fmt.Errorf("重播庫 %s 的 signals 不是空的（%d 筆）：每次重播要用新建的 database", name, n)
		}
	}
	return dbs, 0, nil
}

// runFixed 是補充重播（固定每 5 分鐘一輪）。
func runFixed(ctx context.Context, src *sqlx.DB, srcName string, f, t time.Time, expected []string, watch []bouncereplay.WatchEntry, barLag time.Duration,
	aDSN, bDSN, out string, open func(string) (*sqlx.DB, string, error), fail func(int, string, ...any) int, log *zap.Logger) int {
	in, err := bouncereplay.LoadFixed(ctx, src, bouncereplay.LoadOptions{From: f, To: t.AddDate(0, 0, 1)}, watch)
	if err != nil {
		return fail(1, "讀取輸入：%v", err)
	}
	in.ExpectedDays = expected
	if err := bouncereplay.SynthesizeFixedEvals(in, expected, barLag); err != nil {
		return fail(1, "合成輪次：%v", err)
	}
	fmt.Printf("來源 %s：合成評估 %d 筆、訊號 %d 筆、自選清單 %d 檔（補充重播，保真度較低）\n", srcName, len(in.Evals), len(in.Signals), len(in.Watchlist))
	dbs, code, err := openReplayDBs(ctx, srcName, []string{aDSN, bDSN}, open)
	for _, db := range dbs {
		defer db.Close()
	}
	if err != nil {
		return fail(code, "%v", err)
	}
	// 自我檢查做不了（例如整段缺資料、⛔ 沒有任何一輪會送出）也照樣重播、寫報告：失敗列進報告的完整性，結束碼 3。
	var checks, checkFails []string
	for _, p := range []struct {
		name string
		pol  signal.DedupPolicy
	}{{"A", signal.DedupPriceKey}, {"B", signal.DedupIntradayBounceBySymbol}} {
		desc, err := bouncereplay.SelfCheck(ctx, in, p.pol, log)
		if err != nil {
			x := fmt.Sprintf("%s：%v", p.name, err)
			if desc != "" {
				x += "（" + desc + "）"
			}
			checkFails = append(checkFails, x)
			continue
		}
		checks = append(checks, fmt.Sprintf("%s 通過（%s）", p.name, desc))
	}
	r := bouncereplay.NewFixedRunner(in, dbs[0], dbs[1], log)
	if err := r.Run(ctx); err != nil {
		return fail(1, "重播：%v", err)
	}
	rf, err := os.Create(filepath.Join(out, "report_fixed.md"))
	if err != nil {
		return fail(1, "%v", err)
	}
	sum := bouncereplay.WriteFixedReport(rf, in, r.Records, expected, barLag, r.Notes, checks, checkFails)
	rf.Close()
	cf, err := os.Create(filepath.Join(out, "records_fixed.csv"))
	if err != nil {
		return fail(1, "%v", err)
	}
	if err := bouncereplay.WriteCSV(cf, r.Records); err != nil {
		return fail(1, "%v", err)
	}
	cf.Close()
	fmt.Println("補充重播完成：" + filepath.Join(out, "report_fixed.md"))
	if !sum.OK {
		for _, x := range sum.Failures {
			fmt.Println("⚠️ " + x)
		}
		return 3
	}
	return 0
}
