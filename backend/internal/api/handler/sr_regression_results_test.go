package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type srRegressionResultRepoStub struct {
	rows          []store.SRRegressionResult
	limit         int
	schemaVersion string
}

type srEvaluationJobRepoStub struct {
	jobs    []store.SREvaluationJob
	created *store.SREvaluationJob
	done    chan struct{}
	limit   int
}

type chipScoreRepoStub struct {
	rows []store.ChipScore
}

func (s *chipScoreRepoStub) Upsert(ctx context.Context, score *store.ChipScore) error { return nil }
func (s *chipScoreRepoStub) BulkUpsert(ctx context.Context, scores []store.ChipScore) error {
	return nil
}
func (s *chipScoreRepoStub) GetByDate(ctx context.Context, symbol string, date time.Time) (*store.ChipScore, error) {
	return nil, context.Canceled
}
func (s *chipScoreRepoStub) GetLatest(ctx context.Context, symbol string) (*store.ChipScore, error) {
	return nil, context.Canceled
}
func (s *chipScoreRepoStub) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]store.ChipScore, error) {
	return s.rows, nil
}

type modelGovernanceRepoStub struct {
	rows []store.SRModelGovernance
}

func (s *modelGovernanceRepoStub) ListRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]store.SRModelGovernance, error) {
	return s.rows, nil
}

func (s *srEvaluationJobRepoStub) Create(ctx context.Context, job *store.SREvaluationJob) (uint64, error) {
	job.ID = 1
	s.created = job
	s.jobs = append([]store.SREvaluationJob{*job}, s.jobs...)
	return 1, nil
}

func (s *srEvaluationJobRepoStub) MarkRunning(ctx context.Context, jobID string) error {
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			s.jobs[i].Status = "running"
		}
	}
	return nil
}

func (s *srEvaluationJobRepoStub) MarkDone(ctx context.Context, jobID string, report store.RawJSON, runID, schemaVersion, pipelineVersion string, rows, sources int) error {
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			s.jobs[i].Status = "done"
			s.jobs[i].Report = report
			s.jobs[i].RunID.Valid = runID != ""
			s.jobs[i].RunID.String = runID
			s.jobs[i].SchemaVersion.Valid = schemaVersion != ""
			s.jobs[i].SchemaVersion.String = schemaVersion
			s.jobs[i].PipelineVersion.Valid = pipelineVersion != ""
			s.jobs[i].PipelineVersion.String = pipelineVersion
			s.jobs[i].Rows.Valid = rows > 0
			s.jobs[i].Rows.Int64 = int64(rows)
			s.jobs[i].Sources.Valid = sources > 0
			s.jobs[i].Sources.Int64 = int64(sources)
		}
	}
	if s.done != nil {
		close(s.done)
	}
	return nil
}

func (s *srEvaluationJobRepoStub) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			s.jobs[i].Status = "failed"
			s.jobs[i].Error.Valid = true
			s.jobs[i].Error.String = errMsg
		}
	}
	if s.done != nil {
		close(s.done)
	}
	return nil
}

func (s *srEvaluationJobRepoStub) Get(ctx context.Context, jobID string) (*store.SREvaluationJob, error) {
	for i := range s.jobs {
		if s.jobs[i].JobID == jobID {
			return &s.jobs[i], nil
		}
	}
	return nil, context.Canceled
}

func (s *srEvaluationJobRepoStub) List(ctx context.Context, limit int) ([]store.SREvaluationJob, error) {
	s.limit = limit
	if len(s.jobs) < limit {
		return s.jobs, nil
	}
	return s.jobs[:limit], nil
}

func (s *srRegressionResultRepoStub) Create(ctx context.Context, result *store.SRRegressionResult) (uint64, error) {
	return 0, nil
}

func (s *srRegressionResultRepoStub) Get(ctx context.Context, runID string) (*store.SRRegressionResult, error) {
	return nil, nil
}

func (s *srRegressionResultRepoStub) List(ctx context.Context, limit int) ([]store.SRRegressionResult, error) {
	s.limit = limit
	return s.rows, nil
}

func (s *srRegressionResultRepoStub) ListBySchemaVersion(ctx context.Context, schemaVersion string, limit int) ([]store.SRRegressionResult, error) {
	s.schemaVersion = schemaVersion
	s.limit = limit
	return s.rows, nil
}

func TestSRRegressionResultHandlerList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &srRegressionResultRepoStub{
		rows: []store.SRRegressionResult{
			{
				ID:              1,
				RunID:           "sr_replay_001",
				ModelConfigHash: "hash-v3",
				PipelineVersion: "sr_zone_pipeline_v3",
				MetricsJSON:     store.RawJSON(`{"schema_version":"sr_zone_decision_replay_p0","outcome_summary":{"rows":3}}`),
			},
		},
	}
	handler := NewSRRegressionResultHandler(nil, repo, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/regression-results", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/sr-zones/regression-results?limit=50&schema_version=sr_zone_decision_replay_p0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status=200, got %d body=%s", w.Code, w.Body.String())
	}
	if repo.limit != 50 {
		t.Fatalf("expected limit=50, got %d", repo.limit)
	}
	if repo.schemaVersion != "sr_zone_decision_replay_p0" {
		t.Fatalf("unexpected schema_version: %s", repo.schemaVersion)
	}
	body := w.Body.String()
	for _, want := range []string{`"total":1`, `"run_id":"sr_replay_001"`, `"metrics_json":{"schema_version":"sr_zone_decision_replay_p0"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %s, got %s", want, body)
		}
	}
}

func TestSRRegressionResultHandlerEvaluateForwardsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-scoring/evaluate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var body analysis.SREvaluationRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if len(body.Symbols) != 1 || body.Symbols[0] != "2330" {
			t.Fatalf("unexpected symbols: %+v", body.Symbols)
		}
		if !body.DecisionReplay || !body.WriteDB || body.ReplayMaxRows != 25 {
			t.Fatalf("unexpected evaluation request: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"schema_version":            "sr_zone_decision_replay_p0",
			"run_id":                    "sr_replay_api_001",
			"pipeline_version":          "sr_zone_decision_replay_p0",
			"rows":                      12,
			"sources":                   1,
			"decision_replay_available": true,
		})
	}))
	defer upstream.Close()

	evalJobs := &srEvaluationJobRepoStub{done: make(chan struct{})}
	handler := NewSRRegressionResultHandler(analysis.NewClient(upstream.URL), &srRegressionResultRepoStub{}, evalJobs, nil, nil, zap.NewNop())
	router := gin.New()
	router.POST("/sr-zones/evaluate", handler.Evaluate)

	req := httptest.NewRequest(
		http.MethodPost,
		"/sr-zones/evaluate",
		strings.NewReader(`{"symbols":[" 2330 "],"decision_replay":true,"write_db":true,"replay_max_rows":25}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status=202, got %d body=%s", w.Code, w.Body.String())
	}
	if evalJobs.created == nil || evalJobs.created.Mode != "decision_replay" || !evalJobs.created.WriteDB {
		t.Fatalf("expected evaluation job to be created, got %+v", evalJobs.created)
	}

	select {
	case <-evalJobs.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for evaluation job")
	}
	got, err := evalJobs.Get(context.Background(), evalJobs.created.JobID)
	if err != nil {
		t.Fatalf("Get job failed: %v", err)
	}
	if got.Status != "done" || !got.RunID.Valid || got.RunID.String != "sr_replay_api_001" {
		t.Fatalf("unexpected completed job: %+v", got)
	}
}

func TestSRRegressionResultHandlerEvaluationJobListAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	evalJobs := &srEvaluationJobRepoStub{
		jobs: []store.SREvaluationJob{{ID: 1, JobID: "sr_eval_job_001", Status: "done", Report: store.RawJSON(`{"rows":3}`)}},
	}
	handler := NewSRRegressionResultHandler(nil, &srRegressionResultRepoStub{}, evalJobs, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/evaluation-jobs", handler.ListEvaluationJobs)
	router.GET("/sr-zones/evaluation-jobs/:job_id", handler.GetEvaluationJob)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sr-zones/evaluation-jobs?limit=5", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected list status=200, got %d body=%s", w.Code, w.Body.String())
	}
	if evalJobs.limit != 5 || !strings.Contains(w.Body.String(), `"job_id":"sr_eval_job_001"`) {
		t.Fatalf("unexpected list response: limit=%d body=%s", evalJobs.limit, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sr-zones/evaluation-jobs/sr_eval_job_001", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected get status=200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"report":{"rows":3}`) {
		t.Fatalf("unexpected get response body=%s", w.Body.String())
	}
}

func TestSRRegressionResultHandlerEvaluateInjectsReplayDBContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chipRows := []store.ChipScore{{
		Symbol: "2330", TradeDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		InstitutionalScore: 1, MarginScore: 2, BrokerScore: 3, ConcentrationScore: 4,
		TotalScore: 10, Signal: "BULLISH",
	}}
	governanceRows := []store.SRModelGovernance{{
		Symbol: "2330", Timeframe: "1d", AnalyzedAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		ModelVersion: "v1", ModelConfigHash: "hash-v1", HealthState: "HEALTHY",
		QualityFlags: store.RawJSON(`["OK"]`), WarningFlags: store.RawJSON(`[]`), BlockingFlags: store.RawJSON(`[]`),
		ConfidenceGateJSON: store.RawJSON(`{"allow_entry":true,"max_entry_state":"ENTRY_ALLOWED"}`),
	}}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body analysis.SREvaluationRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if len(body.ChipScoresBySymbol["2330"]) != 1 {
			t.Fatalf("expected chip context, got %+v", body.ChipScoresBySymbol)
		}
		if len(body.ModelGovernanceBySymbol["2330"]) != 1 {
			t.Fatalf("expected governance context, got %+v", body.ModelGovernanceBySymbol)
		}
		if body.ModelGovernanceBySymbol["2330"][0]["health_state"] != "HEALTHY" {
			t.Fatalf("unexpected governance payload: %+v", body.ModelGovernanceBySymbol["2330"][0])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"schema_version": "sr_zone_decision_replay_p0", "run_id": "sr_replay_ctx_001"})
	}))
	defer upstream.Close()

	evalJobs := &srEvaluationJobRepoStub{done: make(chan struct{})}
	handler := NewSRRegressionResultHandler(
		analysis.NewClient(upstream.URL), &srRegressionResultRepoStub{}, evalJobs,
		&chipScoreRepoStub{rows: chipRows}, &modelGovernanceRepoStub{rows: governanceRows}, zap.NewNop(),
	)
	router := gin.New()
	router.POST("/sr-zones/evaluate", handler.Evaluate)

	req := httptest.NewRequest(
		http.MethodPost,
		"/sr-zones/evaluate",
		strings.NewReader(`{"symbols":["2330"],"decision_replay":true,"write_db":false,"replay_max_rows":10}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status=202, got %d body=%s", w.Code, w.Body.String())
	}
	select {
	case <-evalJobs.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for evaluation job")
	}
}
