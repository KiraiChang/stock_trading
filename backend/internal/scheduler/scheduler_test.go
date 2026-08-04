package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/store"
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
