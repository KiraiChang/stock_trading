package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type SRRegressionResultHandler struct {
	client          *analysis.Client
	repo            store.SRRegressionResultRepo
	evalJobs        store.SREvaluationJobRepo
	chipScores      store.ChipScoreRepo
	modelGovernance store.SRModelGovernanceRepo
	log             *zap.Logger
}

func NewSRRegressionResultHandler(
	client *analysis.Client,
	repo store.SRRegressionResultRepo,
	evalJobs store.SREvaluationJobRepo,
	chipScores store.ChipScoreRepo,
	modelGovernance store.SRModelGovernanceRepo,
	log *zap.Logger,
) *SRRegressionResultHandler {
	return &SRRegressionResultHandler{
		client: client, repo: repo, evalJobs: evalJobs,
		chipScores: chipScores, modelGovernance: modelGovernance, log: log,
	}
}

// GET /api/v1/sr-zones/regression-results?limit=20&schema_version=sr_zone_decision_replay_p0
func (h *SRRegressionResultHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	schemaVersion := c.Query("schema_version")
	rows, err := h.repo.ListBySchemaVersion(c.Request.Context(), schemaVersion, limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list regression results")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"results":        rows,
		"total":          len(rows),
		"schema_version": schemaVersion,
	})
}

// POST /api/v1/sr-zones/evaluate
func (h *SRRegressionResultHandler) Evaluate(c *gin.Context) {
	var body analysis.SREvaluationRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	symbols := make([]string, 0, len(body.Symbols))
	for _, symbol := range body.Symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols is required"})
		return
	}
	body.Symbols = symbols
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.Limit <= 0 {
		body.Limit = 1500
	}
	if body.Limit > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be <= 5000"})
		return
	}
	if body.DecisionReplay && body.ReplayMaxRows <= 0 {
		body.ReplayMaxRows = 200
	}
	if !body.DecisionReplay {
		body.ReplayMaxRows = 0
	}
	if body.ReplayMaxRows < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "replay_max_rows must be >= 0"})
		return
	}
	if h.client == nil {
		serverError(c, h.log, errors.New("sr evaluation client is not configured"), "sr-zones: evaluate client")
		return
	}
	if h.evalJobs == nil {
		serverError(c, h.log, errors.New("sr evaluation job repo is not configured"), "sr-zones: evaluate jobs")
		return
	}
	analysis.PopulateSREvaluationReplayContext(c.Request.Context(), &body, h.chipScores, h.modelGovernance, h.log)

	symbolsJSON, err := json.Marshal(body.Symbols)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: marshal evaluation symbols")
		return
	}
	jobID := analysis.NewEvaluationJobID()
	mode := "evaluation"
	if body.DecisionReplay {
		mode = "decision_replay"
	}
	job := &store.SREvaluationJob{
		JobID: jobID, Symbols: string(symbolsJSON), Timeframe: body.Timeframe,
		FetchLimit: body.Limit, Mode: mode, WriteDB: body.WriteDB, ReplayMaxRows: body.ReplayMaxRows,
	}
	if _, err := h.evalJobs.Create(c.Request.Context(), job); err != nil {
		serverError(c, h.log, err, "sr-zones: create evaluation job")
		return
	}

	go h.runEvaluationJob(jobID, body)

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "pending",
		"message": "SR Zone evaluation 已在背景啟動",
		"symbols": len(body.Symbols),
	})
}

// GET /api/v1/sr-zones/evaluation-jobs?limit=20
func (h *SRRegressionResultHandler) ListEvaluationJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	jobs, err := h.evalJobs.List(c.Request.Context(), limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list evaluation jobs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// GET /api/v1/sr-zones/evaluation-jobs/:job_id
func (h *SRRegressionResultHandler) GetEvaluationJob(c *gin.Context) {
	job, err := h.evalJobs.Get(c.Request.Context(), c.Param("job_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "evaluation job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *SRRegressionResultHandler) runEvaluationJob(jobID string, request analysis.SREvaluationRequest) {
	ctx := context.Background()
	if err := h.evalJobs.MarkRunning(ctx, jobID); err != nil {
		h.log.Error("sr evaluation job: mark running failed", zap.String("job_id", jobID), zap.Error(err))
	}

	report, err := h.client.RunSREvaluation(ctx, request)
	if err != nil {
		h.log.Error("sr evaluation failed", zap.String("job_id", jobID), zap.Error(err))
		if markErr := h.evalJobs.MarkFailed(ctx, jobID, err.Error()); markErr != nil {
			h.log.Error("sr evaluation job: mark failed failed", zap.String("job_id", jobID), zap.Error(markErr))
		}
		return
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		h.log.Error("sr evaluation job: marshal report failed", zap.String("job_id", jobID), zap.Error(err))
		reportJSON = []byte("null")
	}
	if err := h.evalJobs.MarkDone(
		ctx,
		jobID,
		store.RawJSON(reportJSON),
		analysis.StringFromReport(report, "run_id"),
		analysis.StringFromReport(report, "schema_version"),
		analysis.StringFromReport(report, "pipeline_version"),
		analysis.IntFromReport(report, "rows"),
		analysis.IntFromReport(report, "sources"),
	); err != nil {
		h.log.Error("sr evaluation job: mark done failed", zap.String("job_id", jobID), zap.Error(err))
	}
}

