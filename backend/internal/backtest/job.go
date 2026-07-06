package backtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// CreateRequest 來自 API 或 Scheduler 的建立請求
type CreateRequest struct {
	Strategy  string   `json:"strategy"`
	Symbols   []string `json:"symbols"`
	Timeframe string   `json:"timeframe"`
	StartDate string   `json:"start_date"` // YYYY-MM-DD
	EndDate   string   `json:"end_date"`
	Trigger   string   `json:"trigger"` // "manual" / "scheduler"
	// UseChipFilter/ChipMinScore：【籌碼分析整合】選填，套用後只對 modular
	// 策略生效（legacy backtrader 策略會忽略並記一筆警告 log，見 Python
	// backtest/engine.py::run_backtest）。ChipMinScore 為 chip_scores.total_score
	// 的門檻（-100~100），未達門檻的進場訊號會被濾掉。
	UseChipFilter bool    `json:"use_chip_filter"`
	ChipMinScore  float64 `json:"chip_min_score"`
}

type Manager struct {
	repo          store.BacktestRepo
	pythonHTTPURL string // Method B：Python FastAPI 端點（空白 = 僅用 Method A）
	httpTimeout   time.Duration
	log           *zap.Logger
}

func NewManager(repo store.BacktestRepo, pythonHTTPURL string, log *zap.Logger) *Manager {
	return &Manager{
		repo:          repo,
		pythonHTTPURL: pythonHTTPURL,
		httpTimeout:   30 * time.Second,
		log:           log,
	}
}

// Submit 建立 Job 並嘗試觸發 Python
func (m *Manager) Submit(ctx context.Context, req CreateRequest) (*store.BacktestJob, error) {
	if req.Trigger == "" {
		req.Trigger = "manual"
	}
	if req.Timeframe == "" {
		req.Timeframe = "1d"
	}

	symbolsJSON, err := json.Marshal(req.Symbols)
	if err != nil {
		return nil, fmt.Errorf("symbols marshal: %w", err)
	}

	job := &store.BacktestJob{
		JobID:         newJobID(),
		Type:          "backtest",
		Strategy:      req.Strategy,
		Symbols:       string(symbolsJSON),
		Timeframe:     req.Timeframe,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Status:        "pending",
		Trigger:       req.Trigger,
		UseChipFilter: req.UseChipFilter,
		ChipMinScore:  req.ChipMinScore,
	}

	if err := m.repo.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	// Method B：若 Python HTTP 服務已設定，主動推送觸發
	if m.pythonHTTPURL != "" {
		go m.triggerHTTP(job)
	}
	// Method A：Python worker 會輪詢 pending jobs，無需額外操作

	m.log.Info("backtest job submitted",
		zap.String("job_id", job.JobID),
		zap.String("strategy", job.Strategy),
		zap.String("trigger", job.Trigger),
	)
	return job, nil
}

// triggerHTTP 非同步 POST 通知 Python HTTP service（Method B）
func (m *Manager) triggerHTTP(job *store.BacktestJob) {
	ctx, cancel := context.WithTimeout(context.Background(), m.httpTimeout)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"job_id":          job.JobID,
		"strategy":        job.Strategy,
		"symbols":         job.Symbols,
		"timeframe":       job.Timeframe,
		"start_date":      job.StartDate,
		"end_date":        job.EndDate,
		"use_chip_filter": job.UseChipFilter,
		"chip_min_score":  job.ChipMinScore,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.pythonHTTPURL+"/backtest", bytes.NewReader(body))
	if err != nil {
		m.log.Warn("python trigger request build failed", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		m.log.Warn("python trigger failed, job will be picked up by worker",
			zap.String("job_id", job.JobID),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()
	m.log.Info("python http trigger ok",
		zap.String("job_id", job.JobID),
		zap.Int("status", resp.StatusCode),
	)
}

func newJobID() string {
	return fmt.Sprintf("bt_%s", time.Now().UTC().Format("20060102_150405_000"))
}
