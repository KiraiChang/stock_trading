package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/scheduler"
	"github.com/trading/backend/internal/store"
)

type jobRunRepoStub struct{ runs []store.JobRun }

func (s *jobRunRepoStub) Start(context.Context, string) (uint64, error) { return 1, nil }
func (s *jobRunRepoStub) Finish(context.Context, uint64, string, int, int, string) error {
	return nil
}
func (s *jobRunRepoStub) GetRecent(context.Context, int) ([]store.JobRun, error) {
	return s.runs, nil
}
func (s *jobRunRepoStub) DeleteBefore(context.Context, time.Time) (int64, error) { return 0, nil }

// Start() 只把 closure 註冊進 cron，不 deref 任何 repo，所以依賴全給 nil。
func schedulerWithSREvaluation(enabled bool) *scheduler.Scheduler {
	s := scheduler.New(
		nil, nil, nil, nil, nil, nil, nil, "0 21 * * *", nil, "", false,
		nil, nil, nil, nil,
		config.SREvaluationConfig{Enabled: enabled, Cron: "30 22 * * 1-5"},
		false, zap.NewNop(),
	)
	s.Start()
	return s
}

func statusByJob(t *testing.T, sched *scheduler.Scheduler, runs []store.JobRun) map[string]jobStatus {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewSchedulerHandler(&jobRunRepoStub{runs: runs}, sched, zap.NewNop())
	r := gin.New()
	r.GET("/scheduler/status", h.GetStatus)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/scheduler/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body struct {
		Jobs []jobStatus `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := map[string]jobStatus{}
	for _, j := range body.Jobs {
		out[j.JobName] = j
	}
	return out
}

// 未註冊的排程沒有 job_runs 紀錄，但那是「沒開」不是「卡住」。
// 標成 stale 會訓練使用者忽略這個旗標，真的有 job 卡住時反而看不出來。
func TestSchedulerStatusMarksDisabledJobsInsteadOfStale(t *testing.T) {
	sched := schedulerWithSREvaluation(false)
	defer sched.Stop()

	got := statusByJob(t, sched, nil)

	for _, name := range []string{"sr_evaluation", "evaluation_universe_sync", "corporate_action_sync"} {
		js, ok := got[name]
		if !ok {
			t.Fatalf("%s 不在回應裡", name)
		}
		if js.Status != "disabled" {
			t.Errorf("%s 應為 disabled，實際 %q", name, js.Status)
		}
		if js.Stale {
			t.Errorf("%s 未註冊卻標成 stale", name)
		}
	}
}

// 有註冊卻沒紀錄才是 never_run ＋ stale——那才是「該跑卻沒跑」。
func TestSchedulerStatusKeepsNeverRunStaleForRegisteredJobs(t *testing.T) {
	sched := schedulerWithSREvaluation(true)
	defer sched.Stop()

	got := statusByJob(t, sched, nil)

	for _, name := range []string{"pre_market", "intraday", "daily_close", "sr_zone_verify", "sr_evaluation"} {
		js := got[name]
		if js.Status != "never_run" {
			t.Errorf("%s 應為 never_run，實際 %q", name, js.Status)
		}
		if !js.Stale {
			t.Errorf("%s 已註冊卻沒跑過，應標 stale", name)
		}
	}
}

// 「跑過、後來關了」不是 stale：紀錄再舊也不代表排程卡住。
func TestSchedulerStatusDoesNotStaleDisabledJobWithOldRun(t *testing.T) {
	sched := schedulerWithSREvaluation(false)
	defer sched.Stop()

	old := time.Now().Add(-30 * 24 * time.Hour)
	got := statusByJob(t, sched, []store.JobRun{
		{JobName: "sr_evaluation", Status: "success", StartedAt: old,
			FinishedAt: sql.NullTime{Time: old, Valid: true}},
	})

	js := got["sr_evaluation"]
	if js.Status != "success" {
		t.Fatalf("有紀錄時應回實際狀態，得到 %q", js.Status)
	}
	if js.Stale {
		t.Error("排程已關閉，舊紀錄不該標 stale")
	}
}

// 反面：已註冊且紀錄過舊，必須標 stale——這是這個旗標真正要抓的情況。
func TestSchedulerStatusStalesRegisteredJobWithOldRun(t *testing.T) {
	sched := schedulerWithSREvaluation(true)
	defer sched.Stop()

	old := time.Now().Add(-30 * 24 * time.Hour)
	got := statusByJob(t, sched, []store.JobRun{
		{JobName: "sr_evaluation", Status: "success", StartedAt: old,
			FinishedAt: sql.NullTime{Time: old, Valid: true}},
	})

	if !got["sr_evaluation"].Stale {
		t.Error("已註冊且逾期未跑，應標 stale")
	}
}

// sr_zone_verify 沒有自己的 cron，但寫獨立的 job_runs 紀錄。
// 不列進 knownSchedulerJobs 的話它的失敗只能靠直接查 DB 才看得到。
func TestSchedulerStatusIncludesSRZoneVerify(t *testing.T) {
	sched := schedulerWithSREvaluation(false)
	defer sched.Stop()

	old := time.Now().Add(-time.Hour)
	got := statusByJob(t, sched, []store.JobRun{
		{JobName: "sr_zone_verify", Status: "partial", SymbolsTotal: 50, SymbolsFailed: 3,
			StartedAt: old, FinishedAt: sql.NullTime{Time: old, Valid: true}},
	})

	js, ok := got["sr_zone_verify"]
	if !ok {
		t.Fatal("sr_zone_verify 不在 /scheduler/status 的回應裡")
	}
	if js.Status != "partial" || js.SymbolsFailed != 3 {
		t.Fatalf("狀態沒帶出來：%+v", js)
	}
	// 跟著 daily_close 註冊，所以不是 disabled
	if js.Stale {
		t.Error("一小時前跑過，不該是 stale")
	}
}
