package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
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
	// finishCtxErr 記錄 Finish 收到的 ctx 當下的 Err()。finishRun 刻意不沿用 job 的 ctx
	// （2026-08-24 起），這裡是唯一驗得到那件事的地方。
	finishCtxErr []error
	deleteCutoff *time.Time
}

func (s *schedulerJobRunRepoStub) Start(ctx context.Context, jobName string) (uint64, error) {
	s.started = append(s.started, jobName)
	if s.nextID == 0 {
		s.nextID = 1
	}
	return s.nextID, nil
}

func (s *schedulerJobRunRepoStub) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	s.finishCtxErr = append(s.finishCtxErr, ctx.Err())
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

func (s *schedulerJobRunRepoStub) GetLatestPerJob(ctx context.Context) ([]store.JobRun, error) {
	return nil, nil
}

// deleteCutoff 記下最後一次 DeleteBefore 收到的 cutoff，讓 retention 的測試驗得到它。
func (s *schedulerJobRunRepoStub) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	s.deleteCutoff = &cutoff
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
	listErr  error
	sinceArg *time.Time
	limitArg int
}

// List 是 sr_zone_verify 取待驗清單的入口；listErr 讓測試重現「清單拿不到」那條路徑。
func (s *schedulerSRZoneRepoStub) List(ctx context.Context, symbol string, limit int) ([]store.SRZoneAnalysis, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.analyses, nil
}

// ListRefsSince 是 sr_zone_verify 實際走的入口（只回 id/symbol 的輕量 Ref）。
// sinceArg / limitArg 記下收到的參數，讓「窗口算對了沒」驗得到。
func (s *schedulerSRZoneRepoStub) ListRefsSince(ctx context.Context, since time.Time, limit int) ([]store.SRZoneAnalysisRef, error) {
	s.sinceArg, s.limitArg = &since, limit
	if s.listErr != nil {
		return nil, s.listErr
	}
	// 由同一份 analyses 導出 Ref，而不是另存一份：真實 repo 也是讀同一張表，
	// stub 分兩份資料會讓「清單」與「單筆查詢」在測試裡看到不同的世界。
	refs := make([]store.SRZoneAnalysisRef, 0, len(s.analyses))
	for _, a := range s.analyses {
		refs = append(refs, store.SRZoneAnalysisRef{ID: a.ID, Symbol: a.Symbol})
	}
	return refs, nil
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

// ── SR 排程的「整輪沒開始」必須記成 failed（2026-08-24）──
//
// 兩條 job 都曾經在「連輸入都拿不到」時把整輪記成成功或半成功：finishRun 依
// total/failed 推導狀態，total=0 會讓 failed >= total 不成立，於是
// (0, 0) 落到 success、(0, 1) 落到 partial。兩者都不誠實——那輪一檔都沒處理。
//
// **每條本體都配一個對照組**：修法是把 total 從 0 改成 1，若寫成「只要 total=0 就算
// failed」會誤傷合法的零標的輪（清單是空的、但查詢本身成功），對照組就是釘住那件事的。

// TestSRZoneVerifyFailsWhenListUnavailable 驗取不到待驗清單時整輪記 failed。
// 修改前傳的是 (0, 0, err)：total 與 failed 同為 0，狀態必然推導成 success，
// 只有 error 欄留著訊息——而讀 /scheduler/status 的人是先看 status 的。
func TestSRZoneVerifyFailsWhenListUnavailable(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := &Scheduler{
		jobRuns:    jobRuns,
		srZoneRepo: &schedulerSRZoneRepoStub{listErr: errors.New("db down")},
		log:        zap.NewNop(),
	}

	s.runSRZoneVerification(context.Background())

	if len(jobRuns.finished) != 1 {
		t.Fatalf("finished = %+v, 期望剛好一筆", jobRuns.finished)
	}
	if got := jobRuns.finished[0].status; got != "failed" {
		t.Fatalf("status = %q, 期望 failed（整輪沒開始跑）", got)
	}
	if jobRuns.finished[0].errMsg == "" {
		t.Fatal("error 欄是空的，查不出整輪失敗的原因")
	}
}

// TestSRZoneVerifySucceedsOnEmptyList 是上一條的對照組，**不可刪**。
// 清單查詢成功但沒有任何待驗分析（新環境、或全部驗過了）是合法的零標的輪，
// 必須維持 success。把修法寫成「total=0 一律 failed」時，只有這條會 fail。
func TestSRZoneVerifySucceedsOnEmptyList(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := &Scheduler{
		jobRuns:    jobRuns,
		srZoneRepo: &schedulerSRZoneRepoStub{analyses: nil},
		log:        zap.NewNop(),
	}

	s.runSRZoneVerification(context.Background())

	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "success" {
		t.Fatalf("finished = %+v, 期望 status=success（零標的但查詢正常）", jobRuns.finished)
	}
}

// TestSRAnalysisFailsWhenWatchlistUnavailable 驗 watchlist 讀不到時整輪記 failed。
// 修改前傳的是 (0, 1, err) → partial。但 partial 在 api-reference.md 的定義是
// 「這輪跑得不完整」，三種成因都預設整輪有跑；SR 分析的標的來源只有 watchlist，
// 讀不到就等於整輪沒有輸入。
//
// **與 corporate_action_sync 的降級不同**：那邊讀不到 watchlist 仍會跑當日分片，
// 記 partial 是對的（真的跑了一批）。
func TestSRAnalysisFailsWhenWatchlistUnavailable(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	runner := &schedulerSRAnalysisRunnerStub{}
	s := &Scheduler{
		jobRuns:          jobRuns,
		watchlist:        &schedulerWatchlistStub{err: errors.New("db down")},
		srAnalysisRunner: runner,
		log:              zap.NewNop(),
	}

	s.runSRAnalysis(context.Background(), false)

	if len(jobRuns.finished) != 1 {
		t.Fatalf("finished = %+v, 期望剛好一筆", jobRuns.finished)
	}
	if got := jobRuns.finished[0].status; got != "failed" {
		t.Fatalf("status = %q, 期望 failed（整輪沒有輸入）", got)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner 被呼叫 %d 次，整輪應該一檔都沒跑", len(runner.calls))
	}
}

// TestSRAnalysisSucceedsWhenWatchlistEmpty 是上一條的對照組，**不可刪**。
// watchlist 查得到但是空的（還沒加任何股票）同樣是合法的零標的輪，必須維持 success。
// 這條與 TestSRZoneVerifySucceedsOnEmptyList 一起，界定「整輪沒開始」與
// 「沒有東西要跑」的分界：**看的是查詢有沒有失敗，不是 total 是不是 0**。
func TestSRAnalysisSucceedsWhenWatchlistEmpty(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	runner := &schedulerSRAnalysisRunnerStub{}
	s := &Scheduler{
		jobRuns:          jobRuns,
		watchlist:        &schedulerWatchlistStub{symbols: nil},
		srAnalysisRunner: runner,
		log:              zap.NewNop(),
	}

	s.runSRAnalysis(context.Background(), false)

	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "success" {
		t.Fatalf("finished = %+v, 期望 status=success（watchlist 是空的但查詢正常）", jobRuns.finished)
	}
}

// ── 狀態誠實與逾時止血（2026-08-24）──

// TestFinishRunWritesEvenWhenContextDone 驗 job 的 ctx 逾時後仍寫得回結束狀態。
// 修改前 finishRun 沿用同一個 ctx，逾時那輪連「寫回」都失敗，job_runs 那筆
// **永遠卡在 running**，看起來像還在跑。這條不只影響
// corporate_action_sync——任何 job 的 ctx 逾時都會踩到。
func TestFinishRunWritesEvenWhenContextDone(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := &Scheduler{jobRuns: jobRuns, log: zap.NewNop()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.finishRun(ctx, 7, "any_job", 10, 3, "boom")

	if len(jobRuns.finished) != 1 {
		t.Fatalf("ctx 已取消時沒有寫回結束狀態: %+v", jobRuns.finished)
	}
	if got := jobRuns.finished[0]; got.status != "partial" || got.symbolsFailed != 3 {
		t.Errorf("finish = %+v, 期望 status=partial failed=3", got)
	}
	if err := jobRuns.finishCtxErr[0]; err != nil {
		t.Errorf("寫回時用的 ctx 仍帶著取消訊號 (%v)——finishRun 必須切斷它", err)
	}
}

// schedulerAdjusterCandleStub 只提供 Symbols，讓 RunCorporateActionSync 拿到固定清單。
type schedulerAdjusterCandleStub struct {
	store.CandleRepo
	symbols []string
	err     error
}

func (s *schedulerAdjusterCandleStub) Symbols(context.Context) ([]string, error) {
	return append([]string(nil), s.symbols...), s.err
}

// schedulerSplitSourceStub 讓分割批次成功且不產生事件——本組測試只關心逐檔那條路徑。
type schedulerSplitSourceStub struct{}

func (schedulerSplitSourceStub) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	return nil, nil
}

// schedulerActionRepoStub 是事件表的空實作：本組測試不驗事件落地，只驗 job_runs 的數字。
type schedulerActionRepoStub struct{}

func (schedulerActionRepoStub) Upsert(context.Context, []store.CorporateAction) error { return nil }

func (schedulerActionRepoStub) ListBySymbol(context.Context, string) ([]store.CorporateAction, error) {
	return nil, nil
}

func (schedulerActionRepoStub) Symbols(context.Context) ([]string, error) { return nil, nil }

// schedulerDividendStub 依 symbol 決定行為：failFor 的檔回錯誤，blockFor 的檔
// 等到 ctx 結束才回——後者用來製造「跑到一半逾時」而不依賴 sleep 的時序賭博。
type schedulerDividendStub struct {
	mu      sync.Mutex
	asked   []string
	failFor map[string]bool
	blockFor map[string]bool
}

func (s *schedulerDividendStub) FetchDividends(ctx context.Context, symbol string) ([]store.CorporateAction, error) {
	s.mu.Lock()
	s.asked = append(s.asked, symbol)
	s.mu.Unlock()
	if s.blockFor[symbol] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if s.failFor[symbol] {
		return nil, errors.New("boom")
	}
	return nil, nil
}

func (s *schedulerDividendStub) askedSymbols() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

func newCorporateActionTestScheduler(
	t *testing.T, jobRuns *schedulerJobRunRepoStub, symbols []string, div market.DividendSource,
) *Scheduler {
	t.Helper()
	candles := &schedulerAdjusterCandleStub{symbols: symbols}
	adj := market.NewAdjuster(schedulerSplitSourceStub{}, schedulerActionRepoStub{}, candles, zap.NewNop())
	adj.SetDividendSource(div)
	return &Scheduler{jobRuns: jobRuns, adjuster: adj, log: zap.NewNop()}
}

// TestRunCorporateActionSyncReportsSymbolCounts 驗寫進 job_runs 的是**標的數**。
// 修改前傳的是事件筆數，而欄位叫 symbols_total / symbols_failed；等 failed 開始帶
// 標的數之後，兩個不同單位的數字互比會讓狀態判定失準。
func TestRunCorporateActionSyncReportsSymbolCounts(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	div := &schedulerDividendStub{failFor: map[string]bool{"8088": true}}
	s := newCorporateActionTestScheduler(t, jobRuns, []string{"0050", "2330", "8088"}, div)
	// 本條只驗數字的單位，用 shard_count=1（每天全量）把分片變數排除掉。
	s.corporateActionCfg = config.CorporateActionConfig{ShardCount: 1}

	s.RunCorporateActionSync()

	if len(jobRuns.finished) != 1 {
		t.Fatalf("finish 次數 = %d, 期望 1: %+v", len(jobRuns.finished), jobRuns.finished)
	}
	got := jobRuns.finished[0]
	if got.symbolsTotal != 3 {
		t.Errorf("symbols_total = %d, 期望 3（標的數，不是事件筆數）", got.symbolsTotal)
	}
	if got.symbolsFailed != 1 {
		t.Errorf("symbols_failed = %d, 期望 1", got.symbolsFailed)
	}
	if got.status != "partial" {
		t.Errorf("status = %q, 期望 partial——修改前 failed 恆為 0，808 檔失敗也記 success", got.status)
	}
}

// TestRunCorporateActionSyncPartialOnTimeout 驗逾時被砍斷時：
// ① 剩下的檔不再被送出請求（止血）；② 沒輪到的檔算進 symbols_failed，
// 狀態是 partial 而不是 success。少了 ②，「跑 50 檔就停」會顯示成功。
func TestRunCorporateActionSyncPartialOnTimeout(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	// 第三檔一路擋到 ctx 逾時，之後的 9999 / 1101 應該完全不被問到。
	div := &schedulerDividendStub{blockFor: map[string]bool{"8088": true}}
	symbols := []string{"0050", "2330", "8088", "9999", "1101"}
	s := newCorporateActionTestScheduler(t, jobRuns, symbols, div)
	// 預算壓到 1 秒才驗得到逾時路徑；shard_count=1 保證五檔都在當日名單裡。
	s.corporateActionCfg = config.CorporateActionConfig{TimeoutSec: 1, ShardCount: 1}

	s.RunCorporateActionSync()

	asked := div.askedSymbols()
	if len(asked) != 3 {
		t.Errorf("逾時後仍繼續送出請求：%v", asked)
	}
	if len(jobRuns.finished) != 1 {
		t.Fatalf("finish 次數 = %d, 期望 1: %+v", len(jobRuns.finished), jobRuns.finished)
	}
	got := jobRuns.finished[0]
	if got.status != "partial" {
		t.Errorf("status = %q, 期望 partial", got.status)
	}
	if got.symbolsTotal != 5 {
		t.Errorf("symbols_total = %d, 期望 5（計畫要跑的檔數）", got.symbolsTotal)
	}
	// 8088 自己算失敗，9999 / 1101 是沒輪到的兩檔。
	if got.symbolsFailed != 3 {
		t.Errorf("symbols_failed = %d, 期望 3（1 檔失敗 ＋ 2 檔未處理）", got.symbolsFailed)
	}
	if got.errMsg == "" {
		t.Error("逾時時 error 欄不該是空的")
	}
	if jobRuns.finishCtxErr[0] != nil {
		t.Error("逾時後寫回結束狀態仍用了已逾時的 ctx")
	}
}

// TestRunCorporateActionSyncFailsWhenSymbolListUnavailable 驗連清單都拿不到時記 failed。
// 修改前這條路徑只記一行 log 就繼續往下走，最後仍以 success 收尾。
func TestRunCorporateActionSyncFailsWhenSymbolListUnavailable(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	candles := &schedulerAdjusterCandleStub{err: errors.New("db down")}
	adj := market.NewAdjuster(schedulerSplitSourceStub{}, schedulerActionRepoStub{}, candles, zap.NewNop())
	adj.SetDividendSource(&schedulerDividendStub{})
	s := &Scheduler{jobRuns: jobRuns, adjuster: adj, log: zap.NewNop()}

	s.RunCorporateActionSync()

	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "failed" {
		t.Fatalf("finish = %+v, 期望 status=failed", jobRuns.finished)
	}
}

// TestRunCorporateActionSyncPartialWhenWatchlistUnavailable 驗 watchlist 讀不到時：
// ① 當日分片照跑（不整輪放棄）；② 即使分片內零失敗，狀態也必須是 partial 而不是 success。
//
// ② 是這條的重點：watchlist 那批**根本沒進名單**，所以不可能被算進 symbols_failed，
// 純看 total/failed 一定推導出 success——「有跑，但跑的不是該跑的」又會顯示成正常。
func TestRunCorporateActionSyncPartialWhenWatchlistUnavailable(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	div := &schedulerDividendStub{} // 名單內的檔全部成功 → failed = 0
	symbols := []string{"0050", "2330", "8088"}
	s := newCorporateActionTestScheduler(t, jobRuns, symbols, div)
	s.watchlist = &schedulerWatchlistStub{err: errors.New("db down")}
	// shard_count=1（每天全量）把分片變數排除掉，讓斷言只盯著降級這件事。
	s.corporateActionCfg = config.CorporateActionConfig{ShardCount: 1}

	s.RunCorporateActionSync()

	if got := len(div.askedSymbols()); got != len(symbols) {
		t.Errorf("問到 %d 檔, 期望 %d——watchlist 失敗不該讓分片停跑", got, len(symbols))
	}
	if len(jobRuns.finished) != 1 {
		t.Fatalf("finish 次數 = %d, 期望 1: %+v", len(jobRuns.finished), jobRuns.finished)
	}
	got := jobRuns.finished[0]
	if got.status != "partial" {
		t.Errorf("status = %q, 期望 partial——名單不完整時零失敗也不算成功", got.status)
	}
	if got.symbolsFailed != 0 {
		t.Errorf("symbols_failed = %d, 期望 0（名單內的檔都成功了）", got.symbolsFailed)
	}
	if got.symbolsTotal != len(symbols) {
		t.Errorf("symbols_total = %d, 期望 %d（實際跑的名單大小）", got.symbolsTotal, len(symbols))
	}
	if !strings.Contains(got.errMsg, "watchlist") {
		t.Errorf("error = %q, 期望說明 watchlist 讀取失敗", got.errMsg)
	}
}

// TestFinishRunDegradedKeepsFailedStatus 驗 degraded 只補強、不會把狀態變樂觀：
// 全數失敗仍是 failed，不會被降級旗標改寫成 partial。
func TestFinishRunDegradedKeepsFailedStatus(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := &Scheduler{jobRuns: jobRuns, log: zap.NewNop()}

	s.finishRunDegraded(context.Background(), 1, "job", 3, 3, "boom", true)

	if len(jobRuns.finished) != 1 || jobRuns.finished[0].status != "failed" {
		t.Fatalf("finish = %+v, 期望 status=failed", jobRuns.finished)
	}
}

// ── 覆蓋率分片（2026-08-24）──

// TestCorporateActionShardOfDayMatchesWeekdayForFiveShards 驗 shardCount=5 時
// 片號就等於 weekday-1，與分片導入前規劃的「週一到週五各一片」完全一致。
func TestCorporateActionShardOfDayMatchesWeekdayForFiveShards(t *testing.T) {
	// 2026-08-24 是星期一。
	monday := time.Date(2026, 8, 24, 6, 30, 0, 0, timeutil.TaipeiTZ)
	for i := 0; i < 5; i++ {
		day := monday.AddDate(0, 0, i)
		if got := corporateActionShardOfDay(day, 5); got != i {
			t.Errorf("%s（%s）的片號 = %d, 期望 %d", day.Format("2006-01-02"), day.Weekday(), got, i)
		}
	}
}

// TestCorporateActionShardOfDayCoversAllShards 驗 shardCount=10 時連續 10 個工作日
// 恰好走完 0～9 各一次。這條擋的是「直接寫 weekday-1」那種寫法——它只產生 0～4，
// 片 5 以後永遠輪不到，等於把原本的破洞換個規模保留下來。
func TestCorporateActionShardOfDayCoversAllShards(t *testing.T) {
	const shardCount = 10
	day := time.Date(2026, 8, 24, 6, 30, 0, 0, timeutil.TaipeiTZ) // 星期一
	seen := map[int]int{}
	for len(seen) < shardCount && day.Before(time.Date(2026, 12, 31, 0, 0, 0, 0, timeutil.TaipeiTZ)) {
		if wd := day.Weekday(); wd != time.Saturday && wd != time.Sunday {
			seen[corporateActionShardOfDay(day, shardCount)]++
		}
		day = day.AddDate(0, 0, 1)
	}
	if len(seen) != shardCount {
		t.Fatalf("只走到 %d 片: %v——有片永遠輪不到", len(seen), seen)
	}
	for shard, times := range seen {
		if times != 1 {
			t.Errorf("片 %d 在一輪內出現 %d 次, 期望 1", shard, times)
		}
	}
}

// TestCorporateActionShardOfDayCrossesYearBoundary 驗跨年不跳號、不重複。
// 用 ISO 週數會在 12/31→1/1 從 52 跳回 1，那個不連續正是這條要擋的。
func TestCorporateActionShardOfDayCrossesYearBoundary(t *testing.T) {
	const shardCount = 10
	var seq []int
	day := time.Date(2026, 12, 21, 6, 30, 0, 0, timeutil.TaipeiTZ) // 星期一
	for i := 0; i < 21; i++ {
		if wd := day.Weekday(); wd != time.Saturday && wd != time.Sunday {
			seq = append(seq, corporateActionShardOfDay(day, shardCount))
		}
		day = day.AddDate(0, 0, 1)
	}
	for i := 1; i < len(seq); i++ {
		want := (seq[i-1] + 1) % shardCount
		if seq[i] != want {
			t.Fatalf("第 %d 個工作日的片號 = %d, 期望 %d（序列 %v）", i+1, seq[i], want, seq)
		}
	}
}

// TestCorporateActionSelectionIsStableAcrossListChanges 驗片別綁 symbol 而不是排序位置。
// 這是 hash 取代 `index % 5` 的迴歸保護：位置分片下，清單前面多一檔新股會讓後面
// 所有標的整批位移一格，被推過當天那片的標的要再等一輪——「每檔每週至少覆蓋一次」
// 在清單變動的那一週就不成立。這裡走真正的選取路徑，比對兩份只差一檔的全集。
func TestCorporateActionSelectionIsStableAcrossListChanges(t *testing.T) {
	s := &Scheduler{
		corporateActionCfg: config.CorporateActionConfig{ShardCount: 5},
		log:                zap.NewNop(),
	}
	universe := make([]string, 0, 200)
	for i := 1000; i < 1200; i++ {
		universe = append(universe, strconv.Itoa(i))
	}
	// 新股上市：排在最前面，位置分片下會讓其餘 200 檔全部位移一格。
	withNewListing := append([]string{"0001"}, universe...)

	selected := func(all []string) map[string]bool {
		out, err := s.corporateActionSymbols(context.Background(), all)
		if err != nil {
			t.Fatalf("組名單失敗: %v", err)
		}
		in := map[string]bool{}
		for _, symbol := range out {
			in[symbol] = true
		}
		return in
	}

	before, after := selected(universe), selected(withNewListing)
	moved := 0
	for _, symbol := range universe {
		if before[symbol] != after[symbol] {
			moved++
		}
	}
	if moved != 0 {
		t.Errorf("清單多一檔新股後有 %d 檔的當日歸屬改變——片別跟著位置走了", moved)
	}
}

// TestCorporateActionShardsPartitionUniverse 驗各片是全集的一個分割：
// 聯集等於全集、兩兩互斥。分片規則寫錯造成某片永遠空，是本筆最需要擋住的失誤。
func TestCorporateActionShardsPartitionUniverse(t *testing.T) {
	const shardCount = 5
	universe := make([]string, 0, 200)
	for i := 1000; i < 1200; i++ {
		universe = append(universe, strconv.Itoa(i))
	}

	seen := map[string]int{}
	for shard := 0; shard < shardCount; shard++ {
		count := 0
		for _, symbol := range universe {
			if corporateActionShardOf(symbol, shardCount) == shard {
				seen[symbol]++
				count++
			}
		}
		if count == 0 {
			t.Errorf("片 %d 是空的", shard)
		}
	}
	for _, symbol := range universe {
		if seen[symbol] != 1 {
			t.Errorf("%s 落在 %d 片, 期望 1", symbol, seen[symbol])
		}
	}
}

// TestCorporateActionSymbolsAlwaysIncludesWatchlist 驗當日名單一定含全部 watchlist 標的
// （不是「每一片都含 watchlist」——watchlist 是分片之外另外加的）。
func TestCorporateActionSymbolsAlwaysIncludesWatchlist(t *testing.T) {
	universe := make([]string, 0, 200)
	for i := 1000; i < 1200; i++ {
		universe = append(universe, strconv.Itoa(i))
	}
	watched := []string{"1000", "1001", "1199"}

	s := &Scheduler{
		watchlist:          &schedulerWatchlistStub{symbols: watched},
		corporateActionCfg: config.CorporateActionConfig{ShardCount: 5},
		log:                zap.NewNop(),
	}

	selected, err := s.corporateActionSymbols(context.Background(), universe)
	if err != nil {
		t.Fatalf("組名單失敗: %v", err)
	}
	in := map[string]bool{}
	for _, symbol := range selected {
		in[symbol] = true
	}
	for _, symbol := range watched {
		if !in[symbol] {
			t.Errorf("watchlist 標的 %s 不在當日名單裡", symbol)
		}
	}
	// 名單要比全集小得多，否則分片等於沒生效。
	if len(selected) >= len(universe)/2 {
		t.Errorf("當日名單 %d 檔 / 全集 %d 檔——分片沒有生效", len(selected), len(universe))
	}
}

// TestCorporateActionSymbolsCoversEveryoneOverOneCycle 驗一個輪替週期內每檔都輪得到。
// 這是分片的核心目標：取代「每天固定只跑排序最前的約 50 檔」的確定性破洞。
func TestCorporateActionSymbolsCoversEveryoneOverOneCycle(t *testing.T) {
	const shardCount = 5
	universe := make([]string, 0, 200)
	for i := 1000; i < 1200; i++ {
		universe = append(universe, strconv.Itoa(i))
	}

	covered := map[string]bool{}
	for shard := 0; shard < shardCount; shard++ {
		for _, symbol := range universe {
			if corporateActionShardOf(symbol, shardCount) == shard {
				covered[symbol] = true
			}
		}
	}
	if len(covered) != len(universe) {
		t.Fatalf("一輪只覆蓋 %d 檔 / 全集 %d 檔", len(covered), len(universe))
	}
}

// TestCorporateActionSymbolsDegradesWhenWatchlistUnavailable 驗 watchlist 拿不到時
// **降級成只跑當日分片**，而不是整輪放棄（2026-08-24 review 後改）。
//
// 分片那一批與 watchlist 無關；讓它們陪葬會多掉一整片，而片號由日期決定、沒有游標，
// 掉的那片要等下一輪才輪得回來。回傳的 error 仍要有——那是給呼叫端記 partial 的訊號。
func TestCorporateActionSymbolsDegradesWhenWatchlistUnavailable(t *testing.T) {
	const shardCount = 5
	universe := make([]string, 0, 200)
	for i := 1000; i < 1200; i++ {
		universe = append(universe, strconv.Itoa(i))
	}
	s := &Scheduler{
		watchlist:          &schedulerWatchlistStub{err: errors.New("db down")},
		corporateActionCfg: config.CorporateActionConfig{ShardCount: shardCount},
		log:                zap.NewNop(),
	}

	selected, err := s.corporateActionSymbols(context.Background(), universe)
	if err == nil {
		t.Fatal("watchlist 讀取失敗時仍要回傳錯誤——呼叫端靠它把該輪記成 partial")
	}

	// 名單必須剛好等於當日分片：一檔都不能少（降級不等於不跑），也不能因為
	// watchlist 讀失敗而混進不屬於今天的標的。
	shard := corporateActionShardOfDay(time.Now(), shardCount)
	want := map[string]bool{}
	for _, symbol := range universe {
		if corporateActionShardOf(symbol, shardCount) == shard {
			want[symbol] = true
		}
	}
	if len(want) == 0 {
		t.Fatal("當日分片是空的，這條測試驗不到東西")
	}
	if len(selected) != len(want) {
		t.Fatalf("降級後名單 %d 檔, 期望當日分片的 %d 檔", len(selected), len(want))
	}
	for _, symbol := range selected {
		if !want[symbol] {
			t.Errorf("名單裡的 %s 不屬於當日分片", symbol)
		}
	}
}

// TestCorporateActionTimeoutAndShardDefaults 驗設定缺漏或填了無效值時退回預設，
// 而不是變成 0（0 秒預算 = 每輪立刻逾時；0 片 = 除以零）。
func TestCorporateActionTimeoutAndShardDefaults(t *testing.T) {
	for _, cfg := range []config.CorporateActionConfig{{}, {TimeoutSec: -1, ShardCount: -1}, {TimeoutSec: 0, ShardCount: 0}} {
		s := &Scheduler{corporateActionCfg: cfg, log: zap.NewNop()}
		if got := s.corporateActionTimeout(); got != defaultCorporateActionTimeout {
			t.Errorf("cfg=%+v 的 timeout = %v, 期望 %v", cfg, got, defaultCorporateActionTimeout)
		}
		if got := s.corporateActionShardCount(); got != defaultCorporateActionShardCount {
			t.Errorf("cfg=%+v 的 shard_count = %d, 期望 %d", cfg, got, defaultCorporateActionShardCount)
		}
	}
}

// TestJobRunRetentionCutoffIsThirtyDaysBack 驗保留期的起點算對。
//
// **原本是 TodayTaipei()（只留當天）**，於是排程健康史每天歸零：2026-08-24 那次
// corporate_action_sync 狀態修正（原記於 issue.md I-084，已收斂）所依據的
// 「failed=808」證據，當天不看、隔天就不存在了
// （見 docs/api-reference.md 的「job_runs 保留 30 天」）。
// 這條把「30 天」與「台北日界」兩件事一起釘住——用 UTC 日界會讓台灣時間的
// 凌晨 08:00 前多刪或少刪一天。
func TestJobRunRetentionCutoffIsThirtyDaysBack(t *testing.T) {
	cutoff := jobRunRetentionCutoff()
	today := timeutil.TodayTaipei()

	if got := today.Sub(cutoff); got != 30*24*time.Hour {
		t.Errorf("cutoff 距今 %v，期望剛好 30 天", got)
	}
	// 必須落在台北時區的日界上（00:00:00），否則同一天的紀錄會被部分刪除。
	inTaipei := cutoff.In(timeutil.TaipeiTZ)
	if h, m, sec := inTaipei.Clock(); h != 0 || m != 0 || sec != 0 {
		t.Errorf("cutoff 台北時間是 %02d:%02d:%02d，期望落在日界 00:00:00", h, m, sec)
	}
}

// ── sr_zone_verify 的覆蓋窗口（見 docs/architecture.md 的排程說明段）──

// TestSRZoneVerifyUsesConfiguredWindow 驗排程真的照設定的天數取分析。
// analyses 回空，所以不會走到 Verify（srZoneVerifier 是具體型別、無法 stub），
// 這條只盯「窗口參數算得對不對」。
func TestSRZoneVerifyUsesConfiguredWindow(t *testing.T) {
	repo := &schedulerSRZoneRepoStub{}
	s := &Scheduler{jobRuns: &schedulerJobRunRepoStub{}, srZoneRepo: repo, log: zap.NewNop()}
	s.SetSRZoneVerify(config.SRZoneVerifyConfig{Days: 7, MaxAnalyses: 123})

	s.runSRZoneVerification(context.Background())

	if repo.sinceArg == nil {
		t.Fatal("ListRefsSince 沒被呼叫——排程可能還走在舊的「最近 N 筆」路徑上")
	}
	gotDays := int(time.Since(*repo.sinceArg).Hours() / 24)
	if gotDays < 6 || gotDays > 8 { // 容忍跨日與執行耗時
		t.Errorf("窗口起點距今約 %d 天，期望 7 天", gotDays)
	}
	if repo.limitArg != 123 {
		t.Errorf("limit = %d, 期望 123（硬上限應照設定）", repo.limitArg)
	}
}

// 設定沒注入或填了非正值時要退回預設，不能變成「往回驗 0 天」而靜默驗不到東西。
func TestSRZoneVerifyFallsBackToDefaults(t *testing.T) {
	for _, cfg := range []config.SRZoneVerifyConfig{
		{},                             // 完全沒注入
		{Days: 0, MaxAnalyses: 0},      // 明確填 0
		{Days: -5, MaxAnalyses: -1},    // 負值
	} {
		repo := &schedulerSRZoneRepoStub{}
		s := &Scheduler{jobRuns: &schedulerJobRunRepoStub{}, srZoneRepo: repo, log: zap.NewNop()}
		s.SetSRZoneVerify(cfg)

		s.runSRZoneVerification(context.Background())

		if repo.sinceArg == nil {
			t.Fatalf("cfg=%+v：ListRefsSince 沒被呼叫", cfg)
		}
		gotDays := int(time.Since(*repo.sinceArg).Hours() / 24)
		if gotDays < defaultSRZoneVerifyDays-1 || gotDays > defaultSRZoneVerifyDays+1 {
			t.Errorf("cfg=%+v：窗口約 %d 天，期望退回預設 %d 天", cfg, gotDays, defaultSRZoneVerifyDays)
		}
		if repo.limitArg != defaultSRZoneVerifyMaxAnalyses {
			t.Errorf("cfg=%+v：limit = %d，期望退回預設 %d", cfg, repo.limitArg, defaultSRZoneVerifyMaxAnalyses)
		}
	}
}

// 設定值超過 maxSRZoneVerifyMaxAnalyses 時要被截到上限，不能原封不動變成 SQL LIMIT。
//
// **這道 clamp 是唯一的底**：runSRZoneVerification 沒有 context.WithTimeout，
// RunDailyClose 尾端也是無條件呼叫它。少了它，SR_ZONE_VERIFY_MAX_ANALYSES 多打
// 一個零就會讓單輪逐筆驗證跑到失控。行為比照 store 的 maxTimelineMaxAnalyses。
func TestSRZoneVerifyClampsMaxAnalysesToHardLimit(t *testing.T) {
	cases := []struct {
		name string
		set  int
		want int
	}{
		{"遠超上限", 10_000_000, maxSRZoneVerifyMaxAnalyses},
		{"略超上限", maxSRZoneVerifyMaxAnalyses + 1, maxSRZoneVerifyMaxAnalyses},
		// 對照組：剛好等於上限、以及上限以下的合法值都必須原封不動通過，
		// 否則這條會在「clamp 寫成一律截到上限」時假綠。
		{"剛好等於上限", maxSRZoneVerifyMaxAnalyses, maxSRZoneVerifyMaxAnalyses},
		{"上限以下照設定", 123, 123},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &schedulerSRZoneRepoStub{}
			s := &Scheduler{jobRuns: &schedulerJobRunRepoStub{}, srZoneRepo: repo, log: zap.NewNop()}
			s.SetSRZoneVerify(config.SRZoneVerifyConfig{Days: 7, MaxAnalyses: tc.set})

			s.runSRZoneVerification(context.Background())

			if repo.sinceArg == nil {
				t.Fatal("ListRefsSince 沒被呼叫")
			}
			if repo.limitArg != tc.want {
				t.Errorf("設定 %d → limit = %d，期望 %d", tc.set, repo.limitArg, tc.want)
			}
		})
	}
}
