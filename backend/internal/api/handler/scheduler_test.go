package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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

// GetRecent **必須照 limit 截斷並依 started_at 由新到舊排序**，因為真實 repo 是
// `ORDER BY started_at DESC LIMIT ?`。stub 忽略 limit 的話，「一天的紀錄放不進視窗」
// 這類問題在測試裡永遠不會出現——GetStatus 靠這個視窗判斷 job 有沒有跑過。
func (s *jobRunRepoStub) GetRecent(_ context.Context, limit int) ([]store.JobRun, error) {
	sorted := append([]store.JobRun(nil), s.runs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StartedAt.After(sorted[j].StartedAt)
	})
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted, nil
}
func (s *jobRunRepoStub) DeleteBefore(context.Context, time.Time) (int64, error) { return 0, nil }

// AbortRunning 只有 main.go 會呼叫（啟動時回收孤兒紀錄），handler 自己不會用到。
func (s *jobRunRepoStub) AbortRunning(context.Context) (int64, error) { return 0, nil }

// GetLatestPerJob 模擬真實 SQL 的語意：每個 job_name 取最新一筆
// （ORDER BY started_at DESC, id DESC）。**這裡刻意不套用任何筆數上限**——
// 真實查詢回的是「有紀錄的 job_name 各一列」，筆數不隨同一個 job 累積多少紀錄而增加
// （固定 len(knownSchedulerJobs) 列是 GetStatus 遍歷 knownSchedulerJobs 補出來的，不是這句 SQL）。
// stub 若加了上限，就驗不出「視窗放不下一天的紀錄」那一整類問題。
func (s *jobRunRepoStub) GetLatestPerJob(_ context.Context) ([]store.JobRun, error) {
	latest := map[string]store.JobRun{}
	for _, r := range s.runs {
		cur, ok := latest[r.JobName]
		if !ok || r.StartedAt.After(cur.StartedAt) || (r.StartedAt.Equal(cur.StartedAt) && r.ID > cur.ID) {
			latest[r.JobName] = r
		}
	}
	out := make([]store.JobRun, 0, len(latest))
	for _, r := range latest {
		out = append(out, r)
	}
	return out, nil
}

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

// TestSchedulerStatusFindsMorningJobsBeyondIntradayWindow 驗「一天的紀錄塞不進取數視窗」
// 時，早上跑過的 job 不會被誤報成從未執行。
//
// GetStatus 取最近 N 筆再在記憶體分組，取不到紀錄的已註冊 job 會被標成 never_run
// ＋ stale=true。而 intraday 每 5 分鐘一筆、09:00–13:30 一天就 **55 筆**，
// 比視窗還多——於是每天過了 13:30，06:30 的 corporate_action_sync／stock_symbol_sync
// 與 08:50 的 pre_market 全被擠出視窗，狀態頁顯示成「該跑卻沒跑」。
//
// 這正是 api-reference.md 警告過的「訓練使用者忽略 stale 旗標」：真的卡住時反而看不出來。
func TestSchedulerStatusFindsMorningJobsBeyondIntradayWindow(t *testing.T) {
	sched := schedulerWithSREvaluation(true)
	defer sched.Stop()

	day := time.Now().Truncate(time.Hour)
	runs := []store.JobRun{
		// 早上這三支各跑一次，全部成功。
		{JobName: "corporate_action_sync", Status: "success", StartedAt: day.Add(-7 * time.Hour)},
		{JobName: "stock_symbol_sync", Status: "success", StartedAt: day.Add(-7 * time.Hour)},
		{JobName: "pre_market", Status: "success", StartedAt: day.Add(-6 * time.Hour)},
	}
	// 盤中 55 筆（09:00–13:30 每 5 分鐘），全部晚於上面三支。
	for i := 0; i < 55; i++ {
		runs = append(runs, store.JobRun{
			JobName:   "intraday",
			Status:    "success",
			StartedAt: day.Add(-5*time.Hour + time.Duration(i)*5*time.Minute),
		})
	}

	got := statusByJob(t, sched, runs)

	// **只斷言 pre_market**：corporate_action_sync 與 stock_symbol_sync 需要注入
	// adjuster / stockSyncer 才會註冊，在這個 stub 環境沒註冊，會回 disabled——
	// 那是另一種正確行為，拿來斷言會讓這條測試變成假綠。pre_market 是無條件註冊、
	// 且唯一早於 intraday 的 job，所以它是這個視窗問題最乾淨的探針。
	// **live 上受害的不只它**：那邊 adjuster 有注入，06:30 的 corporate_action_sync
	// 同樣被擠出視窗，而它的 stale 門檻是 80 小時，誤報的殺傷力更大。
	if js := got["pre_market"]; js.Status == "never_run" {
		t.Errorf("pre_market 今天早上跑過且成功，卻被報成 never_run——紀錄被 intraday 擠出取數視窗")
	} else if js.Stale {
		t.Errorf("pre_market 今天早上才跑過，不該標 stale（status=%q）", js.Status)
	}

	// 對照：intraday 自己在視窗內，狀態必須是正常的。這條同時確認上面的失敗
	// 真的來自「視窗被 intraday 佔滿」，而不是 stub 或排序寫錯。
	if js := got["intraday"]; js.Status != "success" {
		t.Fatalf("intraday 應在視窗內且為 success，得到 %q——測試前提不成立", js.Status)
	}
}
