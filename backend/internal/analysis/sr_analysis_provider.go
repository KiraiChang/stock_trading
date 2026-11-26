package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/trading/backend/internal/store"
)

const (
	defaultSRAnalysisTimeframe     = "1d"
	defaultSRAnalysisLimit         = 250
	defaultSRAnalysisSnapshotLimit = 200
	defaultSRAnalysisReuseMaxAge   = 24 * time.Hour
)

type SRZoneScorer interface {
	ScoreZones(ctx context.Context, symbol, timeframe string, limit int) (*ZoneScoreResult, error)
}

type SRAnalysisOptions struct {
	Timeframe    string
	Limit        int
	ForceRefresh bool
}

type SRAnalysisResult struct {
	Analysis *store.SRZoneAnalysis
	Zones    []store.SRZone
}

// LatestCandleReader 取某檔某 timeframe 最新的一根 K 棒（store.CandleRepo 滿足它）。查無資料時回 sql.ErrNoRows。
type LatestCandleReader interface {
	GetLatest(ctx context.Context, symbol, timeframe string) (*store.Candle, error)
}

type SRAnalysisProvider struct {
	scorer        SRZoneScorer
	repo          store.SRZoneRepo
	candles       LatestCandleReader
	reuseMaxAge   time.Duration
	snapshotLimit int
	now           func() time.Time
}

// NewSRAnalysisProvider：candles 用來確認候選快照是不是「目前最新那根 K 棒」算出來的（T-089 D1）；
// 是 nil 時確認不了，一律⛔ 不重用。
func NewSRAnalysisProvider(scorer SRZoneScorer, repo store.SRZoneRepo, candles LatestCandleReader, reuseMaxAge time.Duration) *SRAnalysisProvider {
	if reuseMaxAge <= 0 {
		reuseMaxAge = defaultSRAnalysisReuseMaxAge
	}
	return &SRAnalysisProvider{
		scorer: scorer, repo: repo, candles: candles, reuseMaxAge: reuseMaxAge,
		snapshotLimit: defaultSRAnalysisSnapshotLimit,
		now:           time.Now,
	}
}

func (p *SRAnalysisProvider) Analyze(ctx context.Context, symbol string, opts SRAnalysisOptions) (*SRAnalysisResult, error) {
	if opts.Timeframe == "" {
		opts.Timeframe = defaultSRAnalysisTimeframe
	}
	if opts.Limit == 0 {
		opts.Limit = defaultSRAnalysisLimit
	}

	if !opts.ForceRefresh {
		existing, err := p.loadReusable(ctx, symbol, opts.Timeframe)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	if p.scorer == nil {
		return nil, fmt.Errorf("sr analysis scorer is not configured")
	}
	result, err := p.scorer.ScoreZones(ctx, symbol, opts.Timeframe, opts.Limit)
	if err != nil {
		return nil, err
	}
	sr, zones, projections, err := result.ToStore()
	if err != nil {
		return nil, fmt.Errorf("convert sr zone result to store: %w", err)
	}
	id, err := p.repo.Create(ctx, sr, zones, projections)
	if err != nil {
		return nil, fmt.Errorf("create sr zone analysis: %w", err)
	}
	saved, err := p.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get saved sr zone analysis: %w", err)
	}
	savedZones, err := p.repo.GetZones(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get saved sr zone zones: %w", err)
	}
	return &SRAnalysisResult{Analysis: saved, Zones: savedZones}, nil
}

// loadReusable 找可以重用的快照。T-089 D1：除了 timeframe 相同、年齡在 reuseMaxAge 內，快照的 analyzed_at
// 還要**等於**這一檔、這個 timeframe 目前最新那根 K 棒的時間（Python 把 analyzed_at 設成最後一根 K 棒的時間）。
// 新 K 棒入庫後一定重算，⛔ 不再依賴 reuseMaxAge 的設定值。查不到最新 K 棒（查無資料或查詢失敗）→ ⛔ 不重用、
// 照常呼叫 Python，⛔ 不因為這次查詢而讓分析失敗。只會讓重用變少、⛔ 不會變多。
func (p *SRAnalysisProvider) loadReusable(ctx context.Context, symbol, timeframe string) (*SRAnalysisResult, error) {
	if p.candles == nil {
		return nil, nil
	}
	latest, err := p.candles.GetLatest(ctx, symbol, timeframe)
	if err != nil || latest == nil {
		return nil, nil
	}
	snapshotLimit := p.snapshotLimit
	if snapshotLimit <= 0 {
		snapshotLimit = defaultSRAnalysisSnapshotLimit
	}
	analyses, err := p.repo.List(ctx, symbol, snapshotLimit)
	if err != nil {
		return nil, fmt.Errorf("list existing sr zone analyses: %w", err)
	}
	for i := range analyses {
		if analyses[i].Timeframe != timeframe {
			continue
		}
		if !analyses[i].AnalyzedAt.Equal(latest.Timestamp) {
			continue
		}
		age := p.currentTime().Sub(analyses[i].AnalyzedAt)
		if age < 0 || age > p.reuseMaxAge {
			continue
		}
		zones, err := p.repo.GetZones(ctx, analyses[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get reusable sr zone zones: %w", err)
		}
		return &SRAnalysisResult{Analysis: &analyses[i], Zones: zones}, nil
	}
	return nil, nil
}

func (p *SRAnalysisProvider) currentTime() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}
