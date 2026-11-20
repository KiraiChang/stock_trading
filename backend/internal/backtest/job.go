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
	maxSymbols    int    // 每個任務的標的數上限（BACKTEST_MAX_SYMBOLS；I-137）
	httpTimeout   time.Duration
	log           *zap.Logger
}

func NewManager(repo store.BacktestRepo, pythonHTTPURL string, maxSymbols int, log *zap.Logger) *Manager {
	return &Manager{
		repo:          repo,
		pythonHTTPURL: pythonHTTPURL,
		maxSymbols:    maxSymbols,
		httpTimeout:   30 * time.Second,
		log:           log,
	}
}

// MaxSymbols 回傳每個任務的標的數上限，給 POST /backtest 的檢查用。
func (m *Manager) MaxSymbols() int { return m.maxSymbols }

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

	// 不論哪一種結果都⛔ 不改任務狀態：狀態只由 python 的條件式認領決定（I-134）。
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// 連線失敗或逾時：python-server 可能已經在回應之前認領了（認領先提交、再回 202），
		// 所以⛔ 不能當成「沒被認領」；worker 只會接手仍是 pending 的任務。
		m.log.Warn("python trigger result unknown: python-server may have claimed the job; "+
			"worker only picks it up if it is still pending",
			zap.String("job_id", job.JobID),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		m.log.Info("python http trigger ok",
			zap.String("job_id", job.JobID),
			zap.Int("status", resp.StatusCode),
		)
	case resp.StatusCode == http.StatusConflict:
		m.log.Info("python http trigger: job already claimed or no longer pending",
			zap.String("job_id", job.JobID),
			zap.Int("status", resp.StatusCode),
		)
	default:
		m.log.Warn("python http trigger rejected",
			zap.String("job_id", job.JobID),
			zap.Int("status", resp.StatusCode),
		)
	}
}

func newJobID() string {
	return fmt.Sprintf("bt_%s", time.Now().UTC().Format("20060102_150405_000"))
}
