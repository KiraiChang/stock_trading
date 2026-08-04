package analysis

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// PopulateSREvaluationReplayContext 補齊 decision replay 所需的歷史 DB context。
// 手動 API 與 scheduler 共用同一套規則，避免 chip / model governance 的 replay
// 資料來源在不同入口產生語意差異。
func PopulateSREvaluationReplayContext(
	ctx context.Context,
	request *SREvaluationRequest,
	chipScores store.ChipScoreRepo,
	modelGovernance store.SRModelGovernanceRepo,
	log *zap.Logger,
) {
	if request == nil || !request.DecisionReplay {
		return
	}
	from, to := evaluationContextRange(request.Limit)
	if len(request.ChipScoresBySymbol) == 0 && chipScores != nil {
		request.ChipScoresBySymbol = chipScoresContext(ctx, chipScores, request.Symbols, from, to, log)
	}
	if len(request.ModelGovernanceBySymbol) == 0 && modelGovernance != nil {
		request.ModelGovernanceBySymbol = modelGovernanceContext(ctx, modelGovernance, request.Symbols, request.Timeframe, from, to, log)
	}
}

func evaluationContextRange(limit int) (time.Time, time.Time) {
	if limit <= 0 {
		limit = 1500
	}
	days := limit * 3
	if days < 365 {
		days = 365
	}
	to := time.Now().UTC().AddDate(0, 0, 1)
	return to.AddDate(0, 0, -days), to
}

func chipScoresContext(
	ctx context.Context,
	repo store.ChipScoreRepo,
	symbols []string,
	from time.Time,
	to time.Time,
	log *zap.Logger,
) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, symbol := range symbols {
		rows, err := repo.GetRange(ctx, symbol, from, to)
		if err != nil {
			if log != nil {
				log.Warn("sr evaluation: chip context unavailable", zap.String("symbol", symbol), zap.Error(err))
			}
			continue
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, map[string]any{
				"symbol":              row.Symbol,
				"trade_date":          row.TradeDate,
				"institutional_score": row.InstitutionalScore,
				"margin_score":        row.MarginScore,
				"broker_score":        row.BrokerScore,
				"concentration_score": row.ConcentrationScore,
				"total_score":         row.TotalScore,
				"signal":              row.Signal,
			})
		}
		if len(items) > 0 {
			out[symbol] = items
		}
	}
	return out
}

func modelGovernanceContext(
	ctx context.Context,
	repo store.SRModelGovernanceRepo,
	symbols []string,
	timeframe string,
	from time.Time,
	to time.Time,
	log *zap.Logger,
) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, symbol := range symbols {
		rows, err := repo.ListRange(ctx, symbol, timeframe, from, to)
		if err != nil {
			if log != nil {
				log.Warn("sr evaluation: model governance context unavailable", zap.String("symbol", symbol), zap.Error(err))
			}
			continue
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, modelGovernanceReplayRow(row))
		}
		if len(items) > 0 {
			out[symbol] = items
		}
	}
	return out
}

func modelGovernanceReplayRow(row store.SRModelGovernance) map[string]any {
	return map[string]any{
		"as_of":                  row.AnalyzedAt,
		"created_at":             row.CreatedAt,
		"symbol":                 row.Symbol,
		"timeframe":              row.Timeframe,
		"model_version":          row.ModelVersion,
		"model_config_hash":      row.ModelConfigHash,
		"health_state":           row.HealthState,
		"average_edge_pp":        nullableFloat(row.AverageEdgePP),
		"directional_zone_count": nullableInt(row.DirectionalZoneCount),
		"zone_count":             nullableInt(row.ZoneCount),
		"allow_entry":            nullableBool(row.AllowEntry),
		"max_entry_state":        row.MaxEntryState,
		"quality_flags":          jsonValue(row.QualityFlags, []any{}),
		"warning_flags":          jsonValue(row.WarningFlags, []any{}),
		"blocking_flags":         jsonValue(row.BlockingFlags, []any{}),
		"confidence_gate":        jsonValue(row.ConfidenceGateJSON, map[string]any{}),
		"calibration_report":     jsonValue(row.CalibrationReportJSON, map[string]any{}),
		"walk_forward_report":    jsonValue(row.WalkForwardReportJSON, map[string]any{}),
		"dataset_diagnostics":    jsonValue(row.DatasetDiagnosticsJSON, map[string]any{}),
		"governance":             jsonValue(row.GovernanceJSON, map[string]any{}),
	}
}

func nullableFloat(value store.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func nullableInt(value store.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableBool(value store.NullBool) any {
	if !value.Valid {
		return nil
	}
	return value.Bool
}

func jsonValue(raw store.RawJSON, fallback any) any {
	if raw == "" {
		return fallback
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}
