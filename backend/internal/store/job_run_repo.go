package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type JobRunRepo interface {
	Start(ctx context.Context, jobName string) (uint64, error)
	Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error
	GetRecent(ctx context.Context, limit int) ([]JobRun, error)
	// GetLatestPerJob 回傳每個 job_name 的最新一筆執行紀錄。
	//
	// **不能用 GetRecent(N) 代替**：那是「最近 N 筆」，而 intraday 每 5 分鐘一筆、
	// 一天就 55 筆，會把早上跑過的 job 整批擠出視窗，讓 /scheduler/status 誤報成
	// never_run（見 docs/api-reference.md 的 GET /scheduler/status「取數方式」）。
	// **回傳的是「表裡有紀錄的 job_name 各一列」**：沒跑過的 job 不會出現，
	// 空表回 0 筆。「/scheduler/status 一定回 11 列」是 handler 遍歷
	// knownSchedulerJobs 補出來的，不是這個方法的保證。
	GetLatestPerJob(ctx context.Context) ([]JobRun, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
	// AbortRunning 把所有仍停在 `running` 的紀錄改寫成 `aborted`，回傳筆數。
	//
	// **要解的問題**：process 被外力砍掉（部署重啟、OOM kill、crash）時
	// `finishRun` 根本沒機會執行，那筆紀錄會永遠停在 `running`；而
	// `/scheduler/status` 的 stale 判定又明確排除 `running`
	// （`api/handler/scheduler.go` 的 `r.Status != "running"`），
	// 於是「跑到一半死掉」被顯示成「正在跑」，**不是 80 小時後才亮，是永遠不會亮**。
	// 由 `main.go` 在啟動時、任何東西能寫出新的 `running` 之前呼叫一次，
	// 讓 `running` 在資料上只代表「真的正在跑」。
	// 現況規格見 docs/api-reference.md 的 GET /scheduler/status。
	//
	// **這與 finishRun 的 ctx 修法是同一個病灶的兩個入口**：那次
	// （2026-08-24，原 I-084）堵的是「ctx 逾時後寫不進去」，這裡堵的是
	// 「連寫的機會都沒有」。兩個都要有。
	//
	// **前提：同一個 DB 只有一個 backend 實例。** 這個方法不分實例，
	// 第二台 backend 啟動會把第一台**正在跑**的 job 一起標成 `aborted`。
	// 要橫向擴充 backend 之前，這裡必須先加實例識別。
	AbortRunning(ctx context.Context) (int64, error)
}

// jobRunStatusMaxLen 是 `job_runs.status` 的欄位寬度。
//
// **三個引擎並不相同**（2026-08-25 review 更正）：
//
//	postgres/010_create_job_runs.sql  status VARCHAR(10)
//	mysql/010_create_job_runs.sql     status VARCHAR(10)
//	sqlite/010_create_job_runs.sql    status TEXT   ← 不限長度
//
// 也就是說 **sqlite 永遠不會擋超長的狀態值**，而單元測試只跑 sqlite（見 I-054）。
// 「寫進去再讀出來還一樣」這種測試在這裡**證明不了任何事**，真正會爆的是
// postgres 與 mysql，而那兩個引擎沒有 repo 層測試。所以長度改由下面的**編譯期斷言**守。
const jobRunStatusMaxLen = 10

// 編譯期擋住超長的狀態值：常數為負時 `uint(...)` 無法通過編譯
// （`constant -N overflows uint`）。新增狀態值時比照加一行。
//
// 這是取代不了的那道防線：sqlite 不擋、單元測試也就驗不到，
// 靠人記得「不能超過 10 個字」遲早會失守——2026-08-11 `corporate_action_sync`
// 撞 `VARCHAR(20)` 就是這樣進 live 的。
const _ = uint(jobRunStatusMaxLen - len(jobRunAbortedStatus))

// jobRunAbortedStatus 是被中斷的執行紀錄的狀態值。
//
// `interrupted`（11 字元）裝不下，所以用 `aborted`（7 字元）——postgres 會直接報錯、
// mysql 依 sql_mode 可能靜默截斷，兩種都比現在更糟。長度由上面的編譯期斷言把守。
//
// **不併進 `failed`**：`failed` 是「跑了但全軍覆沒」，`aborted` 是
// 「沒跑完、結果未知」——已完成的部分仍然有效（2026-08-25 那次是 135 檔中的 54 檔）。
// 兩者混用會讓「要不要重跑」失去判斷依據。
const jobRunAbortedStatus = "aborted"

// jobRunAbortedError 寫進 `error` 欄位，讓排程頁看得出這筆為什麼結束。
// `error` 是 TEXT，沒有長度限制。
const jobRunAbortedError = "process 在此 job 完成前結束，該輪未跑完（啟動時回收）"

type jobRunRepo struct {
	db     *sqlx.DB
	driver string
}

func NewJobRunRepo(db *sqlx.DB) JobRunRepo {
	return &jobRunRepo{db: db, driver: db.DriverName()}
}

func (r *jobRunRepo) Start(ctx context.Context, jobName string) (uint64, error) {
	// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO job_runs (job_name, status) VALUES ($1, 'running') RETURNING id
		`, jobName).Scan(&id)
		return id, err
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO job_runs (job_name, status) VALUES (?, 'running')
	`), jobName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *jobRunRepo) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE job_runs
		SET status=?, symbols_total=?, symbols_failed=?, error=?, finished_at=CURRENT_TIMESTAMP
		WHERE id=?
	`), status, symbolsTotal, symbolsFailed, errMsg, runID)
	return err
}

func (r *jobRunRepo) GetRecent(ctx context.Context, limit int) ([]JobRun, error) {
	var rows []JobRun
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, job_name, status, symbols_total, symbols_failed, error, started_at, finished_at
		FROM job_runs
		ORDER BY started_at DESC
		LIMIT ?
	`), limit)
	return rows, err
}

// GetLatestPerJob 用 window function 一次取出每個 job 的最新一筆。
//
// **ORDER BY 帶 id DESC 是必要的**：started_at 的精度到秒，同一秒內起跑的兩筆
// （例如手動觸發撞上排程）沒有它就沒有確定順序，狀態頁會在兩筆之間跳動。
//
// 三個引擎共用同一句：window function 需要 PostgreSQL、MySQL 8.0+、SQLite 3.25+，
// 三者都滿足（sqlite 走 modernc.org/sqlite，單元測試每次都會跑到這句）。
// 支撐它的是 idx_job_runs_job_name_started_at（migration 072）。
func (r *jobRunRepo) GetLatestPerJob(ctx context.Context) ([]JobRun, error) {
	var rows []JobRun
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, job_name, status, symbols_total, symbols_failed, error, started_at, finished_at
		FROM (
			SELECT id, job_name, status, symbols_total, symbols_failed, error, started_at, finished_at,
			       ROW_NUMBER() OVER (PARTITION BY job_name ORDER BY started_at DESC, id DESC) AS rn
			FROM job_runs
		) t
		WHERE rn = 1
	`)
	return rows, err
}

// DeleteBefore 刪除 cutoff 之前的執行紀錄。
//
// 保留期由呼叫端決定（scheduler 的 jobRunRetentionDays）。**不再是「只留當天」**：
// 那會讓排程健康史每天歸零，連「昨天那輪是不是 partial」都查不到
// （見 docs/api-reference.md 的「job_runs 保留 30 天」）。
func (r *jobRunRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		DELETE FROM job_runs WHERE started_at < ?
	`), cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// AbortRunning 見介面上的說明。
//
// **只動 status / error / finished_at，不碰 symbols_total 與 symbols_failed**：
// 那兩個數字在 Start() 之後、Finish() 之前都是 0，而 0 正確表達了「沒有回報過」。
// 硬填一個猜出來的值會讓「跑完但零標的」與「沒跑完」在資料上分不開。
func (r *jobRunRepo) AbortRunning(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE job_runs
		SET status=?, error=?, finished_at=CURRENT_TIMESTAMP
		WHERE status='running'
	`), jobRunAbortedStatus, jobRunAbortedError)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
