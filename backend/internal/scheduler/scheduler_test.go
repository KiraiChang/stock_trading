package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type schedulerWatchlistStub struct {
	symbols []string
	err     error
}

func (s *schedulerWatchlistStub) GetAll(ctx context.Context) ([]store.WatchlistItem, error) {
	return nil, nil
}

func (s *schedulerWatchlistStub) Add(ctx context.Context, symbol, name, sector string) error {
	return nil
}

func (s *schedulerWatchlistStub) Update(ctx context.Context, symbol, name, sector string) error {
	return nil
}

func (s *schedulerWatchlistStub) Remove(ctx context.Context, symbol string) error {
	return nil
}

func (s *schedulerWatchlistStub) Symbols(ctx context.Context) ([]string, error) {
	return append([]string(nil), s.symbols...), s.err
}

func (s *schedulerWatchlistStub) SetWatched(ctx context.Context, symbol string, watched bool) error {
	return nil
}

type schedulerJobRunFinish struct {
	runID         uint64
	status        string
	symbolsTotal  int
	symbolsFailed int
	errMsg        string
}

type schedulerJobRunRepoStub struct {
	nextID   uint64
	started  []string
	finished []schedulerJobRunFinish
}

func (s *schedulerJobRunRepoStub) Start(ctx context.Context, jobName string) (uint64, error) {
	s.started = append(s.started, jobName)
	if s.nextID == 0 {
		s.nextID = 1
	}
	return s.nextID, nil
}

func (s *schedulerJobRunRepoStub) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	s.finished = append(s.finished, schedulerJobRunFinish{
		runID:         runID,
		status:        status,
		symbolsTotal:  symbolsTotal,
		symbolsFailed: symbolsFailed,
		errMsg:        errMsg,
	})
	return nil
}

func (s *schedulerJobRunRepoStub) GetRecent(ctx context.Context, limit int) ([]store.JobRun, error) {
	return nil, nil
}

func (s *schedulerJobRunRepoStub) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

type schedulerSREvaluationJobRepoStub struct {
	created     *store.SREvaluationJob
	markRunning []string
	markFailed  []string
	done        *store.SREvaluationJob
	doneReport  store.RawJSON
}

func (s *schedulerSREvaluationJobRepoStub) Create(ctx context.Context, job *store.SREvaluationJob) (uint64, error) {
	copy := *job
	s.created = &copy
	return 1, nil
}

func (s *schedulerSREvaluationJobRepoStub) MarkRunning(ctx context.Context, jobID string) error {
	s.markRunning = append(s.markRunning, jobID)
	return nil
}

func (s *schedulerSREvaluationJobRepoStub) MarkDone(ctx context.Context, jobID string, report store.RawJSON, runID, schemaVersion, pipelineVersion string, rows, sources int) error {
	s.done = &store.SREvaluationJob{
		JobID:           jobID,
		Status:          "done",
		RunID:           store.NullString{NullString: sql.NullString{String: runID, Valid: runID != ""}},
		SchemaVersion:   store.NullString{NullString: sql.NullString{String: schemaVersion, Valid: schemaVersion != ""}},
		PipelineVersion: store.NullString{NullString: sql.NullString{String: pipelineVersion, Valid: pipelineVersion != ""}},
		Rows:            store.NewNullInt64(int64(rows)),
		Sources:         store.NewNullInt64(int64(sources)),
	}
	s.doneReport = report
	return nil
}

func (s *schedulerSREvaluationJobRepoStub) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	s.markFailed = append(s.markFailed, jobID+":"+errMsg)
	return nil
}

func (s *schedulerSREvaluationJobRepoStub) Get(ctx context.Context, jobID string) (*store.SREvaluationJob, error) {
	return nil, nil
}

func (s *schedulerSREvaluationJobRepoStub) List(ctx context.Context, limit int) ([]store.SREvaluationJob, error) {
	return nil, nil
}

type schedulerChipScoreRepoStub struct {
	rowsBySymbol map[string][]store.ChipScore
	requested    []string
}

func (s *schedulerChipScoreRepoStub) Upsert(ctx context.Context, score *store.ChipScore) error {
	return nil
}

func (s *schedulerChipScoreRepoStub) BulkUpsert(ctx context.Context, scores []store.ChipScore) error {
	return nil
}

func (s *schedulerChipScoreRepoStub) GetByDate(ctx context.Context, symbol string, date time.Time) (*store.ChipScore, error) {
	return nil, nil
}

func (s *schedulerChipScoreRepoStub) GetLatest(ctx context.Context, symbol string) (*store.ChipScore, error) {
	return nil, nil
}

func (s *schedulerChipScoreRepoStub) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]store.ChipScore, error) {
	s.requested = append(s.requested, symbol)
	return append([]store.ChipScore(nil), s.rowsBySymbol[symbol]...), nil
}

type schedulerModelGovernanceRepoStub struct {
	rowsBySymbol map[string][]store.SRModelGovernance
	requested    []string
}

func (s *schedulerModelGovernanceRepoStub) ListRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]store.SRModelGovernance, error) {
	s.requested = append(s.requested, symbol+":"+timeframe)
	return append([]store.SRModelGovernance(nil), s.rowsBySymbol[symbol]...), nil
}

func TestRunSREvaluationUsesWatchlistInjectsContextAndMarksDone(t *testing.T) {
	var captured analysis.SREvaluationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-scoring/evaluate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"run_id":"sr_replay_001",
			"schema_version":"sr_zone_decision_replay_p0",
			"pipeline_version":"sr_zone_decision_replay_p0",
			"rows":7,
			"sources":2
		}`))
	}))
	defer server.Close()

	jobRuns := &schedulerJobRunRepoStub{nextID: 42}
	evalJobs := &schedulerSREvaluationJobRepoStub{}
	chipRepo := &schedulerChipScoreRepoStub{rowsBySymbol: map[string][]store.ChipScore{
		"2330": {
			{
				Symbol:             "2330",
				TradeDate:          time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				InstitutionalScore: 1.5,
				TotalScore:         3.5,
				Signal:             "BULLISH",
			},
		},
	}}
	governanceRepo := &schedulerModelGovernanceRepoStub{rowsBySymbol: map[string][]store.SRModelGovernance{
		"2330": {
			{
				Symbol:             "2330",
				Timeframe:          "1d",
				AnalyzedAt:         time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				ModelVersion:       "v-test",
				ModelConfigHash:    "hash123",
				HealthState:        "DEGRADED",
				AllowEntry:         store.NullBool{NullBool: sql.NullBool{Bool: true, Valid: true}},
				MaxEntryState:      "SMALL_ENTRY",
				ConfidenceGateJSON: store.RawJSON(`{"allow_entry":true,"max_entry_state":"SMALL_ENTRY"}`),
				QualityFlags:       store.RawJSON(`[]`),
				WarningFlags:       store.RawJSON(`["MODEL_DEGRADED"]`),
				BlockingFlags:      store.RawJSON(`[]`),
			},
		},
	}}
	s := New(
		nil,
		nil,
		&schedulerWatchlistStub{symbols: []string{"2330", "0050"}},
		jobRuns,
		nil,
		nil,
		nil,
		"",
		nil,
		"",
		false,
		analysis.NewClient(server.URL),
		evalJobs,
		chipRepo,
		governanceRepo,
		config.SREvaluationConfig{
			Timeframe:      "1d",
			Limit:          90,
			DecisionReplay: true,
			ReplayMaxRows:  12,
			WriteDB:        true,
		},
		false,
		zap.NewNop(),
	)

	s.runSREvaluation(context.Background())

	if !reflect.DeepEqual(captured.Symbols, []string{"2330", "0050"}) {
		t.Fatalf("expected watchlist symbols, got %+v", captured.Symbols)
	}
	if captured.Timeframe != "1d" || captured.Limit != 90 || !captured.DecisionReplay || captured.ReplayMaxRows != 12 || !captured.WriteDB {
		t.Fatalf("unexpected request: %+v", captured)
	}
	if len(captured.ChipScoresBySymbol["2330"]) != 1 {
		t.Fatalf("expected chip context for 2330, got %+v", captured.ChipScoresBySymbol)
	}
	if len(captured.ModelGovernanceBySymbol["2330"]) != 1 {
		t.Fatalf("expected model governance context for 2330, got %+v", captured.ModelGovernanceBySymbol)
	}
	if !reflect.DeepEqual(chipRepo.requested, []string{"2330", "0050"}) {
		t.Fatalf("expected chip context request for all symbols, got %+v", chipRepo.requested)
	}
	if !reflect.DeepEqual(governanceRepo.requested, []string{"2330:1d", "0050:1d"}) {
		t.Fatalf("expected governance context request for all symbols, got %+v", governanceRepo.requested)
	}

	if evalJobs.created == nil {
		t.Fatal("expected evaluation job to be created")
	}
	if evalJobs.created.Mode != "decision_replay" || evalJobs.created.Status != "pending" {
		t.Fatalf("unexpected created job: %+v", evalJobs.created)
	}
	if evalJobs.created.Symbols != `["2330","0050"]` {
		t.Fatalf("unexpected job symbols: %s", evalJobs.created.Symbols)
	}
	if len(evalJobs.markRunning) != 1 || evalJobs.markRunning[0] != evalJobs.created.JobID {
		t.Fatalf("expected mark running for created job, got %+v", evalJobs.markRunning)
	}
	if evalJobs.done == nil || evalJobs.done.JobID != evalJobs.created.JobID {
		t.Fatalf("expected done job, got %+v", evalJobs.done)
	}
	if evalJobs.done.RunID.String != "sr_replay_001" || evalJobs.done.Rows.Int64 != 7 || evalJobs.done.Sources.Int64 != 2 {
		t.Fatalf("unexpected done projection: %+v", evalJobs.done)
	}
	if !json.Valid([]byte(evalJobs.doneReport)) {
		t.Fatalf("expected valid done report JSON, got %s", evalJobs.doneReport)
	}
	if !reflect.DeepEqual(jobRuns.started, []string{"sr_evaluation"}) {
		t.Fatalf("unexpected job run starts: %+v", jobRuns.started)
	}
	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "success" || jobRuns.finished[0].symbolsTotal != 2 || jobRuns.finished[0].symbolsFailed != 0 {
		t.Fatalf("unexpected job run finish: %+v", jobRuns.finished)
	}
}

func TestRunSREvaluationMarksFailedWhenAnalysisRequestFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	jobRuns := &schedulerJobRunRepoStub{nextID: 7}
	evalJobs := &schedulerSREvaluationJobRepoStub{}
	s := New(
		nil,
		nil,
		&schedulerWatchlistStub{symbols: []string{"2330"}},
		jobRuns,
		nil,
		nil,
		nil,
		"",
		nil,
		"",
		false,
		analysis.NewClient(server.URL),
		evalJobs,
		nil,
		nil,
		config.SREvaluationConfig{Symbols: []string{"2330"}, Timeframe: "1d", Limit: 30},
		false,
		zap.NewNop(),
	)

	s.runSREvaluation(context.Background())

	if evalJobs.created == nil {
		t.Fatal("expected evaluation job to be created")
	}
	if len(evalJobs.markRunning) != 1 {
		t.Fatalf("expected mark running, got %+v", evalJobs.markRunning)
	}
	if len(evalJobs.markFailed) != 1 {
		t.Fatalf("expected mark failed, got %+v", evalJobs.markFailed)
	}
	if len(jobRuns.finished) != 1 {
		t.Fatalf("expected one job run finish, got %+v", jobRuns.finished)
	}
	finish := jobRuns.finished[0]
	if finish.status != "failed" || finish.symbolsTotal != 1 || finish.symbolsFailed != 1 || finish.errMsg == "" {
		t.Fatalf("unexpected failed job run finish: %+v", finish)
	}
}

func TestRunSREvaluationFailsWhenWatchlistFallbackErrors(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{nextID: 3}
	expectedErr := errors.New("watchlist unavailable")
	s := New(
		nil,
		nil,
		&schedulerWatchlistStub{err: expectedErr},
		jobRuns,
		nil,
		nil,
		nil,
		"",
		nil,
		"",
		false,
		nil,
		&schedulerSREvaluationJobRepoStub{},
		nil,
		nil,
		config.SREvaluationConfig{},
		false,
		zap.NewNop(),
	)

	s.runSREvaluation(context.Background())

	if len(jobRuns.finished) != 1 {
		t.Fatalf("expected one job run finish, got %+v", jobRuns.finished)
	}
	finish := jobRuns.finished[0]
	if finish.status != "failed" || finish.symbolsTotal != 1 || finish.symbolsFailed != 1 || finish.errMsg != expectedErr.Error() {
		t.Fatalf("unexpected watchlist failure finish: %+v", finish)
	}
}

// Start() 只是把 closure 註冊進 cron，不會 deref 任何 repo，所以依賴全給 nil 即可。
// chipSyncCron 固定給合法字串，讓「基準排程數」在各案例間穩定。
func newStartTestScheduler(srEvaluation config.SREvaluationConfig) *Scheduler {
	return New(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"0 21 * * *",
		nil,
		"",
		false,
		nil,
		nil,
		nil,
		nil,
		srEvaluation,
		false,
		zap.NewNop(),
	)
}

// startedCronEntries 跑完整個 Start() 後回報 cron 註冊筆數。Start() 會啟動 cron
// goroutine，測試結束前一定要 Stop()，否則 goroutine 會殘留到其他測試。
func startedCronEntries(t *testing.T, s *Scheduler) int {
	t.Helper()
	s.Start()
	t.Cleanup(s.Stop)
	return len(s.cron.Entries())
}

// sr_evaluation 排程「預設關閉」是 T-002 P2 的安全前提（避免開發環境一啟動就狂打
// Python decision replay），這裡用開/關的差值鎖住，不硬編其他排程的數量。
func TestStartRegistersSREvaluationCronOnlyWhenEnabled(t *testing.T) {
	disabled := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{
		Enabled: false,
		Cron:    "0 2 * * *",
	}))
	enabled := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{
		Enabled: true,
		Cron:    "0 2 * * *",
	}))

	if enabled != disabled+1 {
		t.Fatalf("cron entries disabled=%d enabled=%d, want enabled = disabled+1", disabled, enabled)
	}
}

// cron 字串寫錯時只記 log、不註冊、也不能讓整個 scheduler 起不來——其他排程仍要正常註冊。
func TestStartSkipsSREvaluationCronWhenExpressionInvalid(t *testing.T) {
	disabled := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{Enabled: false}))
	invalid := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{
		Enabled: true,
		Cron:    "not-a-cron-expression",
	}))

	if invalid != disabled {
		t.Fatalf("cron entries disabled=%d invalid=%d, want equal (invalid cron must not register)", disabled, invalid)
	}
}

// 手動入口（API 的 RunSREvaluation）必須與 cron 走同一條 runSREvaluation，
// 否則手動觸發會繞過 evaluation job 與 job_runs 紀錄。
func TestRunSREvaluationManualEntryPointSharesCronPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"manual_001","rows":1,"sources":1}`))
	}))
	defer server.Close()

	jobRuns := &schedulerJobRunRepoStub{nextID: 11}
	evalJobs := &schedulerSREvaluationJobRepoStub{}
	s := New(
		nil,
		nil,
		&schedulerWatchlistStub{symbols: []string{"2330"}},
		jobRuns,
		nil,
		nil,
		nil,
		"",
		nil,
		"",
		false,
		analysis.NewClient(server.URL),
		evalJobs,
		nil,
		nil,
		config.SREvaluationConfig{},
		false,
		zap.NewNop(),
	)

	s.RunSREvaluation()

	if evalJobs.created == nil {
		t.Fatal("manual entry point should create an evaluation job")
	}
	if evalJobs.done == nil || evalJobs.done.RunID.String != "manual_001" {
		t.Fatalf("manual entry point should mark job done with report run_id, got %+v", evalJobs.done)
	}
	if !reflect.DeepEqual(jobRuns.started, []string{"sr_evaluation"}) {
		t.Fatalf("manual entry point should record a sr_evaluation job run, got %+v", jobRuns.started)
	}
	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "success" {
		t.Fatalf("unexpected manual entry point job run finish: %+v", jobRuns.finished)
	}
}

// srEvaluationRequest / srEvaluationSymbols 只讀 s.srEvaluation 與 s.watchlist，
// 直接組 struct 即可，不必經過 New() 準備整組依賴。
func newSREvaluationTestScheduler(cfg config.SREvaluationConfig, watchlist store.WatchlistRepo) *Scheduler {
	return &Scheduler{
		watchlist:    watchlist,
		srEvaluation: cfg,
		log:          zap.NewNop(),
	}
}

func TestSREvaluationRequestDefaults(t *testing.T) {
	s := newSREvaluationTestScheduler(config.SREvaluationConfig{}, nil)

	request := s.srEvaluationRequest([]string{"2330"})

	if request.Timeframe != "1d" {
		t.Fatalf("timeframe = %q, want 1d", request.Timeframe)
	}
	if request.Limit != 1500 {
		t.Fatalf("limit = %d, want 1500", request.Limit)
	}
	if request.DecisionReplay || request.WriteDB {
		t.Fatalf("decision replay / write db should default to false: %+v", request)
	}
	// 未開 decision replay 時不該帶 replay 上限，避免 Python 端誤判成要跑 replay。
	if request.ReplayMaxRows != 0 {
		t.Fatalf("replay max rows = %d, want 0 when decision replay is off", request.ReplayMaxRows)
	}
}

func TestSREvaluationRequestDecisionReplayRows(t *testing.T) {
	s := newSREvaluationTestScheduler(config.SREvaluationConfig{DecisionReplay: true, WriteDB: true}, nil)

	request := s.srEvaluationRequest([]string{"2330"})

	if !request.DecisionReplay || !request.WriteDB {
		t.Fatalf("decision replay / write db not propagated: %+v", request)
	}
	if request.ReplayMaxRows != 200 {
		t.Fatalf("replay max rows = %d, want default 200", request.ReplayMaxRows)
	}

	explicit := newSREvaluationTestScheduler(
		config.SREvaluationConfig{DecisionReplay: true, ReplayMaxRows: 50},
		nil,
	)
	if got := explicit.srEvaluationRequest([]string{"2330"}).ReplayMaxRows; got != 50 {
		t.Fatalf("replay max rows = %d, want explicit 50", got)
	}

	// 設定寫了 ReplayMaxRows 但沒開 decision replay 時，仍要清成 0。
	replayOff := newSREvaluationTestScheduler(config.SREvaluationConfig{ReplayMaxRows: 50}, nil)
	if got := replayOff.srEvaluationRequest([]string{"2330"}).ReplayMaxRows; got != 0 {
		t.Fatalf("replay max rows = %d, want 0 when decision replay is off", got)
	}
}

func TestSREvaluationSymbolsPrefersConfigAndFallsBackToWatchlist(t *testing.T) {
	ctx := context.Background()

	configured := newSREvaluationTestScheduler(
		config.SREvaluationConfig{Symbols: []string{" 2330 ", "", "   ", "2454"}},
		&schedulerWatchlistStub{symbols: []string{"1101"}},
	)
	symbols, err := configured.srEvaluationSymbols(ctx)
	if err != nil {
		t.Fatalf("config symbols failed: %v", err)
	}
	if !reflect.DeepEqual(symbols, []string{"2330", "2454"}) {
		t.Fatalf("config symbols not trimmed/filtered: %+v", symbols)
	}

	fallback := newSREvaluationTestScheduler(
		config.SREvaluationConfig{},
		&schedulerWatchlistStub{symbols: []string{"1101", "2603"}},
	)
	symbols, err = fallback.srEvaluationSymbols(ctx)
	if err != nil {
		t.Fatalf("watchlist fallback failed: %v", err)
	}
	if !reflect.DeepEqual(symbols, []string{"1101", "2603"}) {
		t.Fatalf("expected watchlist fallback symbols, got %+v", symbols)
	}
}

// ── IsJobRegistered：區分「沒開」與「該開卻沒開」──
// 規格見 docs/api-reference.md「`status` 的三種『沒有執行紀錄』情形」。
func TestIsJobRegisteredReflectsActualRegistration(t *testing.T) {
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: false})
	s.Start()
	defer s.Stop()

	// 無條件註冊的四支，任何設定下都該是 true。
	// sr_zone_verify 沒有自己的 cron，跟著 daily_close 在 RunDailyClose 尾端跑。
	for _, name := range []string{"pre_market", "intraday", "daily_close", "sr_zone_verify"} {
		if !s.IsJobRegistered(name) {
			t.Errorf("%s 應該永遠註冊", name)
		}
	}
	// 預設關閉的兩支：沒開就是沒註冊，API 層據此顯示 disabled 而不是 stale
	if s.IsJobRegistered("sr_evaluation") {
		t.Error("sr_evaluation 未啟用時不該註冊")
	}
	if s.IsJobRegistered("evaluation_universe_sync") {
		t.Error("evaluation_universe_sync 未注入 repo 時不該註冊")
	}
	// 沒注入 adjuster / stockSyncer 時同樣不註冊
	if s.IsJobRegistered("corporate_action_sync") {
		t.Error("adjuster 未注入時不該註冊 corporate_action_sync")
	}
	if s.IsJobRegistered("stock_symbol_sync") {
		t.Error("stockSyncEnabled=false 時不該註冊")
	}
	// 沒見過的名稱回 false，不 panic
	if s.IsJobRegistered("nope") {
		t.Error("未知 job 應回 false")
	}
}

func TestIsJobRegisteredTrueWhenEnabled(t *testing.T) {
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: true, Cron: "30 22 * * 1-5"})
	s.SetEvaluationUniverse(nil, config.EvaluationUniverseConfig{})
	s.Start()
	defer s.Stop()

	if !s.IsJobRegistered("sr_evaluation") {
		t.Error("啟用後應註冊 sr_evaluation")
	}
	// repo 為 nil 時即使 Enabled 也不註冊——註冊條件是「repo != nil && Enabled」
	if s.IsJobRegistered("evaluation_universe_sync") {
		t.Error("repo 為 nil 時不該註冊")
	}
}

func TestIsJobRegisteredFalseWhenCronStringInvalid(t *testing.T) {
	// cron 打錯時 AddFunc 只記 log 不中止，job 靜默不執行。
	// 這種情況與「刻意關閉」在行為上相同，但 IsJobRegistered 同樣回 false——
	// 呼叫端要分辨成因得看啟動 log。
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: true, Cron: "not a cron"})
	s.Start()
	defer s.Stop()

	if s.IsJobRegistered("sr_evaluation") {
		t.Error("cron 字串非法時不該算註冊成功")
	}
}

func TestIsJobRegisteredIsSafeForConcurrentAccess(t *testing.T) {
	// main.go 是 `go sched.Start()` 與 HTTP server 並行啟動的：Start() 寫
	// registeredJobs 的同時 /scheduler/status 可能正在讀。Go 的 map 不支援並發讀寫，
	// 撞上是 `fatal error: concurrent map read and map write`——不可 recover。
	// 這支在 -race 下會抓到未上鎖的實作；沒有 -race 時仍可能觸發 runtime 的並發偵測。
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: true, Cron: "30 22 * * 1-5"})
	defer s.Stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); s.Start() }()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			for _, name := range []string{"pre_market", "sr_evaluation", "evaluation_universe_sync"} {
				_ = s.IsJobRegistered(name)
			}
		}
	}()
	wg.Wait()
}

// ── corporate_action_sync 的 cron 走 config（T-042）──
// 搬進 config 之前這支排程的 cron 是唯一硬編碼的，其他三支都走 config。

// schedulerWithAdjuster 注入一個不會被呼叫到的 adjuster——Start() 只檢查 nil，
// 不會 deref 裡面的依賴，所以空的就夠。
func schedulerWithAdjuster(t *testing.T, cfg config.CorporateActionConfig) *Scheduler {
	t.Helper()
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: false})
	s.SetAdjuster(market.NewAdjuster(nil, nil, nil, zap.NewNop()), cfg)
	return s
}

func TestCorporateActionCronFallsBackToDefaultWhenUnset(t *testing.T) {
	// config 漏設時不能讓這支排程消失——空字串傳給 AddFunc 會註冊失敗（只記 log），
	// 而漏跑一次就會讓該檔整段歷史出現假跳空。
	for _, cron := range []string{"", "   "} {
		s := schedulerWithAdjuster(t, config.CorporateActionConfig{Cron: cron})
		if got := s.corporateActionCron(); got != defaultCorporateActionCron {
			t.Errorf("cron=%q 應退回預設 %q，實際 %q", cron, defaultCorporateActionCron, got)
		}
		s.Start()
		if !s.IsJobRegistered("corporate_action_sync") {
			t.Errorf("cron=%q 時仍應註冊", cron)
		}
		s.Stop()
	}
}

func TestCorporateActionCronUsesConfiguredValue(t *testing.T) {
	s := schedulerWithAdjuster(t, config.CorporateActionConfig{Cron: "30 6,12 * * 1-5"})
	defer s.Stop()

	if got := s.corporateActionCron(); got != "30 6,12 * * 1-5" {
		t.Fatalf("應採用 config 的值，實際 %q", got)
	}
	s.Start()
	if !s.IsJobRegistered("corporate_action_sync") {
		t.Error("合法 cron 應註冊成功")
	}
}

// 非法 cron **不能**讓這支排程消失。
//
// 其他三支 cron 走 config 的排程註冊失敗時顯示成 disabled 是說得通的（它們都有
// enabled 開關）；本 job 沒有開關，disabled 對它是個不存在的狀態，操作者不會知道
// 還原係數已經停止重算。所以打錯字時用預設時間繼續跑，並留一筆 Error log。
func TestCorporateActionInvalidCronFallsBackToDefault(t *testing.T) {
	for _, spec := range []string{"not a cron", "6:30", "* * *"} {
		s := schedulerWithAdjuster(t, config.CorporateActionConfig{Cron: spec})

		if got := s.corporateActionCron(); got != defaultCorporateActionCron {
			t.Errorf("cron=%q 應退回預設 %q，實際 %q", spec, defaultCorporateActionCron, got)
		}

		s.Start()
		if !s.IsJobRegistered("corporate_action_sync") {
			t.Errorf("cron=%q 打錯字不該讓這支排程消失", spec)
		}
		s.Stop()
	}
}

// 反面：合法但**不同於預設**的值必須原樣採用，否則上面那條退回邏輯會演變成
// 「不管設什麼都跑預設」——那等於 config 沒有接上。
func TestCorporateActionValidCronIsNotOverriddenByFallback(t *testing.T) {
	s := schedulerWithAdjuster(t, config.CorporateActionConfig{Cron: "15 7 * * 1"})
	defer s.Stop()

	if got := s.corporateActionCron(); got != "15 7 * * 1" {
		t.Fatalf("合法值不該被退回預設，實際 %q", got)
	}
}

func TestCorporateActionNotRegisteredWithoutAdjuster(t *testing.T) {
	// 沒注入 adjuster 時不管 cron 設什麼都不註冊（既有行為，一併鎖住）。
	s := newStartTestScheduler(config.SREvaluationConfig{Enabled: false})
	defer s.Stop()

	s.Start()

	if s.IsJobRegistered("corporate_action_sync") {
		t.Error("未注入 adjuster 時不該註冊")
	}
}

// ── SR 分析排程（todo.md T-052）──

// 只實作用到的那一兩支，其餘由內嵌介面補齊：真的被呼叫到會 nil panic，
// 那正是「這支測試碰到了不該碰的東西」的訊號，比回假資料好。
type schedulerSRZoneRepoStub struct {
	store.SRZoneRepo
	analyses []store.SRZoneAnalysis
}

// **stub 也要照 timeframe 過濾**：不濾的話，P2 那條隔離性的測試會假綠。
func (s *schedulerSRZoneRepoStub) GetLatestByTimeframe(ctx context.Context, symbol, timeframe string) (*store.SRZoneAnalysis, error) {
	for i := range s.analyses {
		if s.analyses[i].Timeframe == "" || s.analyses[i].Timeframe == timeframe {
			return &s.analyses[i], nil
		}
	}
	return nil, nil
}

type schedulerChipLatestStub struct {
	store.ChipScoreRepo
	latest *store.ChipScore
}

func (s *schedulerChipLatestStub) GetLatest(ctx context.Context, symbol string) (*store.ChipScore, error) {
	return s.latest, nil
}

type schedulerCandleRepoStub struct {
	store.CandleRepo
	latest *store.Candle
}

func (s *schedulerCandleRepoStub) GetLatest(ctx context.Context, symbol, timeframe string) (*store.Candle, error) {
	return s.latest, nil
}

type schedulerSRAnalysisRunnerStub struct {
	calls   []string
	failFor map[string]bool
}

func (r *schedulerSRAnalysisRunnerStub) RunAnalysis(ctx context.Context, symbol, timeframe string, limit int) (uint64, error) {
	r.calls = append(r.calls, symbol)
	if r.failFor[symbol] {
		return 0, errors.New("boom")
	}
	return uint64(len(r.calls)), nil
}

func newSRAnalysisTestScheduler(
	symbols []string, jobRuns *schedulerJobRunRepoStub,
	candles *schedulerCandleRepoStub, analyses []store.SRZoneAnalysis,
	runner *schedulerSRAnalysisRunnerStub, chip *schedulerChipLatestStub,
) *Scheduler {
	s := New(nil, nil, &schedulerWatchlistStub{symbols: symbols}, jobRuns,
		&schedulerSRZoneRepoStub{analyses: analyses},
		nil, nil, "0 21 * * *", nil, "", false, nil, nil, chip, nil,
		config.SREvaluationConfig{}, false, zap.NewNop())
	s.SetSRAnalysis(runner, candles, config.SRAnalysisConfig{
		Enabled: true, Cron: "0 17 * * 1-5", ChipCron: "0 22 * * 1-5", Timeframe: "1d", Limit: 400,
	})
	return s
}

func todayCandle() *schedulerCandleRepoStub {
	return &schedulerCandleRepoStub{latest: &store.Candle{Timestamp: timeutil.TodayTaipei()}}
}

// 兩輪都要註冊，而且兩輪都不能因為對方存在而消失。
func TestStartRegistersBothSRAnalysisCronsOnlyWhenEnabled(t *testing.T) {
	base := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{}))

	s := newSRAnalysisTestScheduler(nil, &schedulerJobRunRepoStub{}, todayCandle(), nil,
		&schedulerSRAnalysisRunnerStub{}, nil)
	if got := startedCronEntries(t, s); got != base+2 {
		t.Fatalf("cron entries = %d, want base+2 (%d)", got, base+2)
	}
}

// runner 沒注入時整組不註冊——比照 adjuster / evaluationUniverse 的「未注入即等同導入前」。
func TestStartSkipsSRAnalysisWhenRunnerMissing(t *testing.T) {
	base := startedCronEntries(t, newStartTestScheduler(config.SREvaluationConfig{}))

	s := newStartTestScheduler(config.SREvaluationConfig{})
	s.SetSRAnalysis(nil, todayCandle(), config.SRAnalysisConfig{Enabled: true, Cron: "0 17 * * 1-5"})
	if got := startedCronEntries(t, s); got != base {
		t.Fatalf("cron entries = %d, want %d（runner 未注入時不該註冊）", got, base)
	}
}

// 今天的 K 棒還沒到就不要跑——假日、停牌、daily_close 尚未完成都落在這裡。
// **跳過不是失敗**：job_runs 的 total 要把跳過的扣掉，否則每個假日都會看到一筆 failed。
func TestSRAnalysisSkipsWhenLatestCandleIsNotToday(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{nextID: 1}
	runner := &schedulerSRAnalysisRunnerStub{}
	stale := &schedulerCandleRepoStub{latest: &store.Candle{
		Timestamp: timeutil.TodayTaipei().AddDate(0, 0, -1),
	}}
	s := newSRAnalysisTestScheduler([]string{"2330"}, jobRuns, stale, nil, runner, nil)

	s.runSRAnalysis(context.Background(), false)

	if len(runner.calls) != 0 {
		t.Fatalf("K 棒不是今天時不該分析，got %v", runner.calls)
	}
	if len(jobRuns.finished) != 1 || jobRuns.finished[0].symbolsTotal != 0 || jobRuns.finished[0].symbolsFailed != 0 {
		t.Fatalf("job_runs = %+v，want total=0 failed=0（跳過不算失敗）", jobRuns.finished)
	}
}

// **守衛是 per 時段，不是 per 日。** 17:00 已經分析過今天的 K 棒（用的是昨日籌碼），
// 22:00 那輪在**當日籌碼已入庫**的前提下仍然必須跑——這正是兩段式的意義；
// 用「今天分析過就跳過」會把它整輪擋掉。
func TestSRAnalysisChipSlotRunsWhenTodayChipArrived(t *testing.T) {
	yesterday := timeutil.TodayTaipei().AddDate(0, 0, -1).Format("2006-01-02")
	analyses := []store.SRZoneAnalysis{{
		AnalyzedAt:  timeutil.TodayTaipei(),
		ChipSummary: store.RawJSON(`{"trade_date":"` + yesterday + `"}`),
	}}
	runner := &schedulerSRAnalysisRunnerStub{}
	chip := &schedulerChipLatestStub{latest: &store.ChipScore{TradeDate: timeutil.TodayTaipei()}}
	s := newSRAnalysisTestScheduler([]string{"2330"}, &schedulerJobRunRepoStub{nextID: 1},
		todayCandle(), analyses, runner, chip)

	// 17:00 那輪：今天這根 K 棒已經算過了 → 跳過
	s.runSRAnalysis(context.Background(), false)
	if len(runner.calls) != 0 {
		t.Fatalf("17:00 那輪應跳過（今日 K 棒已分析），got %v", runner.calls)
	}

	// 22:00 那輪：當日籌碼已入庫、最新分析用的還是昨日籌碼 → 照跑
	s.runSRAnalysis(context.Background(), true)
	if len(runner.calls) != 1 {
		t.Fatalf("22:00 那輪應照跑（當日籌碼已入庫），got %v", runner.calls)
	}
}

// **當日籌碼還沒入庫就不能跑。** 21:00 的 chip sync 失敗或還沒寫完時，這一輪跑出來的
// 東西會與 17:00 那輪一模一樣——白算一次，還多推一次 observed_absences，
// 污染的正是 T-049 要用的 production 母體。
func TestSRAnalysisChipSlotSkipsWhenTodayChipNotLoaded(t *testing.T) {
	yesterday := timeutil.TodayTaipei().AddDate(0, 0, -1)
	analyses := []store.SRZoneAnalysis{{
		AnalyzedAt:  timeutil.TodayTaipei(),
		ChipSummary: store.RawJSON(`{"trade_date":"` + yesterday.Format("2006-01-02") + `"}`),
	}}

	for _, tc := range []struct {
		name string
		chip *schedulerChipLatestStub
	}{
		{"籌碼還停在昨天", &schedulerChipLatestStub{latest: &store.ChipScore{TradeDate: yesterday}}},
		{"這檔完全沒有籌碼資料", &schedulerChipLatestStub{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &schedulerSRAnalysisRunnerStub{}
			s := newSRAnalysisTestScheduler([]string{"2330"}, &schedulerJobRunRepoStub{nextID: 1},
				todayCandle(), analyses, runner, tc.chip)

			s.runSRAnalysis(context.Background(), true)

			if len(runner.calls) != 0 {
				t.Fatalf("當日籌碼未入庫時不該跑，got %v", runner.calls)
			}
		})
	}
}

// 籌碼是今天的、但最新那筆分析已經用過它了 → 再算一次結果相同。
func TestSRAnalysisChipSlotSkipsWhenTodayChipAlreadyUsed(t *testing.T) {
	today := timeutil.TodayTaipei()
	analyses := []store.SRZoneAnalysis{{
		AnalyzedAt:  today,
		ChipSummary: store.RawJSON(`{"trade_date":"` + today.Format("2006-01-02") + `"}`),
	}}
	runner := &schedulerSRAnalysisRunnerStub{}
	chip := &schedulerChipLatestStub{latest: &store.ChipScore{TradeDate: today}}
	s := newSRAnalysisTestScheduler([]string{"2330"}, &schedulerJobRunRepoStub{nextID: 1},
		todayCandle(), analyses, runner, chip)

	s.runSRAnalysis(context.Background(), true)

	if len(runner.calls) != 0 {
		t.Fatalf("今日籌碼已用過時應跳過，got %v", runner.calls)
	}
}

// 單檔失敗不能拖垮整批：其餘標的照跑，job_runs 記正確的 total / failed。
func TestSRAnalysisContinuesAfterSymbolFailure(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{nextID: 1}
	runner := &schedulerSRAnalysisRunnerStub{failFor: map[string]bool{"3105": true}}
	s := newSRAnalysisTestScheduler([]string{"2330", "3105", "6182"}, jobRuns,
		todayCandle(), nil, runner, nil)

	s.runSRAnalysis(context.Background(), false)

	if len(runner.calls) != 3 {
		t.Fatalf("失敗的那檔不該中斷其餘，got %v", runner.calls)
	}
	if len(jobRuns.finished) != 1 {
		t.Fatalf("want 1 finish, got %+v", jobRuns.finished)
	}
	f := jobRuns.finished[0]
	if f.symbolsTotal != 3 || f.symbolsFailed != 1 || f.status != "partial" {
		t.Fatalf("job_runs = %+v，want total=3 failed=1 status=partial", f)
	}
}

// **timeframe 要隔離。** 使用者今天手動跑過一次 5m 分析，不能讓 1d 的排程誤判
// 「今天已經分析過」而整批跳過——那會讓 sr_analysis.timeframe 這個設定失去意義。
func TestSRAnalysisIgnoresAnalysesFromOtherTimeframe(t *testing.T) {
	analyses := []store.SRZoneAnalysis{{
		Timeframe:  "5m",
		AnalyzedAt: timeutil.TodayTaipei(),
	}}
	runner := &schedulerSRAnalysisRunnerStub{}
	s := newSRAnalysisTestScheduler([]string{"2330"}, &schedulerJobRunRepoStub{nextID: 1},
		todayCandle(), analyses, runner, nil)

	s.runSRAnalysis(context.Background(), false)

	if len(runner.calls) != 1 {
		t.Fatalf("5m 的分析不該擋掉 1d 的排程，got %v", runner.calls)
	}
}
