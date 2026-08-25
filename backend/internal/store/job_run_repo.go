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
}

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
