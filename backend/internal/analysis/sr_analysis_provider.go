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

type SRAnalysisProvider struct {
	scorer        SRZoneScorer
	repo          store.SRZoneRepo
	reuseMaxAge   time.Duration
	snapshotLimit int
	now           func() time.Time
}

func NewSRAnalysisProvider(scorer SRZoneScorer, repo store.SRZoneRepo, reuseMaxAge time.Duration) *SRAnalysisProvider {
	if reuseMaxAge <= 0 {
		reuseMaxAge = defaultSRAnalysisReuseMaxAge
	}
	return &SRAnalysisProvider{
		scorer: scorer, repo: repo, reuseMaxAge: reuseMaxAge,
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

func (p *SRAnalysisProvider) loadReusable(ctx context.Context, symbol, timeframe string) (*SRAnalysisResult, error) {
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
