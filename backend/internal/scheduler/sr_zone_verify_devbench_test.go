package scheduler

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/store"
)

// TestSRZoneVerifyWindowCostOnPostgres 量 sr_zone_verify 在真實 postgres 上、
// 660 筆等級窗口的實際耗時——收盤驗證改成「時間窗口」時，預設 30 天這個值需要
// 一次真實量測而不是線性推估來支撐（實測數字見 docs/architecture.md 的排程說明段）。
//
// **需要一個真的 postgres，所以用環境變數 gate 住**（比照
// internal/database/migrate_postgres_test.go 的 POSTGRES_MIGRATION_DSN 慣例）：
// 沒設 SR_ZONE_VERIFY_BENCH_DSN 就 skip，`backend/scripts/test.sh` 的日常驗證不受影響。
//
// **這條會寫資料**：走的是完整的 runSRZoneVerification，會更新 zone 的
// status/broken_at/broken_price，也會寫一筆 job_runs。**只能對 dev 或拋棄式資料庫跑，
// 不要指向 live。**
//
// 環境變數：
//   - `SR_ZONE_VERIFY_BENCH_DSN`（必填）——postgres 連線字串，沒設就 skip。
//   - `SR_ZONE_VERIFY_BENCH_MAX`（選填）——覆寫單輪上限，預設
//     `defaultSRZoneVerifyMaxAnalyses`。設小值可以在資料不多時驗到「撞上限截斷」
//     那條路徑（此時 `symbols_total` 應等於上限而不是窗口內總量）。
//     **設超過 `maxSRZoneVerifyMaxAnalyses` 也不會生效**——排程會 clamp，
//     期望值這邊同樣算過 clamp，所以兩邊一致。
//
// 量的是「一整輪」而不是單筆平均：ListRefsSince 一次、然後每筆 5 個查詢 ＋ 每個 zone
// 一次 UpdateZoneStatus，成本主要在 DB 往返次數，只有整輪跑完才看得到真實總量。
func TestSRZoneVerifyWindowCostOnPostgres(t *testing.T) {
	dsn := os.Getenv("SR_ZONE_VERIFY_BENCH_DSN")
	if dsn == "" {
		t.Skip("未設 SR_ZONE_VERIFY_BENCH_DSN，跳過 sr_zone_verify 的 postgres 實測")
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連線 postgres 失敗: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	srRepo := store.NewSRZoneRepo(db)
	jobRuns := store.NewJobRunRepo(db)

	days := defaultSRZoneVerifyDays
	since := time.Now().AddDate(0, 0, -days)

	// 上限可覆寫，才能在資料不多時也驗到「撞上限截斷」那條路徑。
	limit := defaultSRZoneVerifyMaxAnalyses
	if v := os.Getenv("SR_ZONE_VERIFY_BENCH_MAX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			t.Fatalf("SR_ZONE_VERIFY_BENCH_MAX=%q 不是正整數", v)
		}
		limit = n
	}

	// **設定值還要再過一次 production 的硬 clamp**：srZoneVerifyMaxAnalyses() 會把
	// 超過 maxSRZoneVerifyMaxAnalyses 的設定截掉，所以排程實際用的是這個值。
	// 少了這一步，設 50000 ＋ 窗口 20000 筆時 benchmark 會期望 20000、
	// 排程正確地只處理 10000，反而把正確行為判成失敗。
	effectiveLimit := min(limit, maxSRZoneVerifyMaxAnalyses)
	if effectiveLimit != limit {
		t.Logf("設定上限 %d 超過硬上限 %d，實際生效 %d", limit, maxSRZoneVerifyMaxAnalyses, effectiveLimit)
	}

	// inWindow 是窗口內的總量，processed 是這一輪**實際會處理**的筆數。
	// **兩者必須分開**：資料量超過生效上限時排程只處理該上限筆數，拿 inWindow 去斷言
	// symbols_total 會把「上限正確生效」誤判成失敗，平均耗時也會被稀釋失真。
	var inWindow int
	if err := db.GetContext(ctx, &inWindow,
		`SELECT count(*) FROM stock_sr_zone_analyses WHERE created_at >= $1`, since); err != nil {
		t.Fatalf("統計分析筆數失敗: %v", err)
	}
	if inWindow == 0 {
		t.Skipf("窗口 %d 天內沒有任何分析——空跑量不到東西，先灌資料再跑（見本檔註解）", days)
	}
	processed := min(inWindow, effectiveLimit)

	// zone 數只算實際被選到的那批，否則截斷時會把沒處理到的 zone 也算進成本。
	var zones int
	if err := db.GetContext(ctx, &zones,
		`SELECT count(*) FROM stock_sr_zones WHERE analysis_id IN (
			SELECT id FROM stock_sr_zone_analyses WHERE created_at >= $1
			ORDER BY created_at DESC, id DESC LIMIT $2
		)`, since, processed); err != nil {
		t.Fatalf("統計 zone 筆數失敗: %v", err)
	}
	t.Logf("窗口 %d 天內共 %d 筆分析；設定上限 %d、生效上限 %d，本輪處理 %d 筆、zone %d 個",
		days, inWindow, limit, effectiveLimit, processed, zones)
	if processed < inWindow {
		t.Logf("**窗口被上限截斷**：%d → %d（正是 job_runs.symbols_total 貼著上限的情境）",
			inWindow, processed)
	}

	s := &Scheduler{
		jobRuns:        jobRuns,
		srZoneRepo:     srRepo,
		srZoneVerifier: analysis.NewSRZoneVerifier(srRepo, store.NewCandleRepo(db)),
		log:            zap.NewNop(),
	}
	// **這裡刻意傳未經 clamp 的 limit**，不傳 effectiveLimit：讓 production 的
	// srZoneVerifyMaxAnalyses() clamp 真的被執行到。期望值那邊已經自己算過 clamp，
	// 兩者對不上就是 clamp 壞了——這條順帶變成 clamp 的實跑檢查。
	s.SetSRZoneVerify(config.SRZoneVerifyConfig{Days: days, MaxAnalyses: limit})

	start := time.Now()
	s.runSRZoneVerification(ctx)
	elapsed := time.Since(start)

	// 從 job_runs 讀排程自己記下的數字——那正是 architecture.md 教維運看的欄位。
	runs, err := jobRuns.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatalf("讀 job_runs 失敗: %v", err)
	}
	var found bool
	for _, r := range runs {
		if r.JobName != "sr_zone_verify" {
			continue
		}
		found = true
		t.Logf("job_runs：status=%s symbols_total=%d symbols_failed=%d",
			r.Status, r.SymbolsTotal, r.SymbolsFailed)
		if r.SymbolsTotal != processed {
			t.Errorf("symbols_total = %d，期望等於本輪實際處理筆數 %d（窗口內 %d 筆、設定上限 %d、生效上限 %d）",
				r.SymbolsTotal, processed, inWindow, limit, effectiveLimit)
		}
		if r.SymbolsFailed != 0 {
			t.Errorf("symbols_failed = %d，期望 0", r.SymbolsFailed)
		}
	}
	if !found {
		t.Fatal("job_runs 裡沒有 sr_zone_verify 的紀錄")
	}

	perAnalysis := elapsed / time.Duration(processed)
	t.Logf("整輪耗時 %v（處理 %d 筆、zone %d 個，平均每筆 %v）", elapsed, processed, zones, perAnalysis)
}
