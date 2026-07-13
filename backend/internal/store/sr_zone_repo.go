package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type SRZoneRepo interface {
	// Create 寫入一筆 zone 評分快照與其所有 zone，回傳新建的 analysis id
	Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone) (uint64, error)
	Get(ctx context.Context, id uint64) (*SRZoneAnalysis, error)
	List(ctx context.Context, symbol string, limit int) ([]SRZoneAnalysis, error)
	GetZones(ctx context.Context, analysisID uint64) ([]SRZone, error)
	// UpdateZoneStatus 供 SRZoneVerifier 使用（見 internal/analysis/sr_zone_verifier.go）。
	// resolvedRole 只有原本 role=AT_ZONE 的 zone 在這次驗證解析出方向時才會
	// 非空；role 本身不是 AT_ZONE 的 zone 呼叫端應傳空字串，維持 NULL。
	UpdateZoneStatus(ctx context.Context, zoneID uint64, status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) error
	// Delete 刪除一筆 zone 評分快照及其所有 zone
	Delete(ctx context.Context, id uint64) error
}

type srZoneRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRZoneRepo(db *sqlx.DB) SRZoneRepo {
	return &srZoneRepo{db: db, driver: db.DriverName()}
}

func (r *srZoneRepo) Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone) (uint64, error) {
	if a.PeriodSummaries == "" {
		a.PeriodSummaries = RawJSON("[]")
	}
	if a.AnalysisTips == "" {
		a.AnalysisTips = RawJSON("[]")
	}
	if a.ChipSummary == "" {
		a.ChipSummary = RawJSON("null")
	}
	if a.DecisionSummary == "" {
		a.DecisionSummary = RawJSON("null")
	}
	if a.Evidence == "" {
		a.Evidence = RawJSON("null")
	}
	if a.Explanation == "" {
		a.Explanation = RawJSON("null")
	}
	if a.Scenario == "" {
		a.Scenario = RawJSON("null")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const cols = `symbol, timeframe, analyzed_at, current_price,
		global_trend, global_volatility, global_expected_value, global_confidence, global_risk_reward_ratio,
		model_version, model_config_hash, pipeline_version, evidence, explanation, scenario,
		period_summaries, analysis_tips, chip_summary, decision_summary`

	var id uint64
	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
			RETURNING id
		`,
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice,
			a.GlobalTrend, a.GlobalVolatility, a.GlobalExpectedValue, a.GlobalConfidence, a.GlobalRiskRewardRatio,
			a.ModelVersion, a.ModelConfigHash, a.PipelineVersion, a.Evidence, a.Explanation, a.Scenario,
			a.PeriodSummaries, a.AnalysisTips, a.ChipSummary, a.DecisionSummary,
		).Scan(&id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		`),
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice,
			a.GlobalTrend, a.GlobalVolatility, a.GlobalExpectedValue, a.GlobalConfidence, a.GlobalRiskRewardRatio,
			a.ModelVersion, a.ModelConfigHash, a.PipelineVersion, a.Evidence, a.Explanation, a.Scenario,
			a.PeriodSummaries, a.AnalysisTips, a.ChipSummary, a.DecisionSummary,
		)
		if err == nil {
			var lastID int64
			lastID, err = result.LastInsertId()
			id = uint64(lastID)
		}
	}
	if err != nil {
		return 0, err
	}

	for i := range zones {
		zones[i].AnalysisID = id
		if zones[i].Status == "" {
			zones[i].Status = "PENDING"
		}
		if zones[i].Features == "" {
			zones[i].Features = RawJSON("null")
		}
		if zones[i].Evidence == "" {
			zones[i].Evidence = RawJSON("null")
		}
		if zones[i].Explanation == "" {
			zones[i].Explanation = RawJSON("null")
		}
		if zones[i].Scenario == "" {
			zones[i].Scenario = RawJSON("null")
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_sr_zones (
				analysis_id, price_low, price_high, method, role, tier, tier_label,
				support_score, resistance_score, net_score, net_score_label,
				confidence, confidence_level,
				bounce_probability, break_probability,
				expected_gain, expected_loss, expected_value, risk_reward_ratio, reward_risk_percentile,
				relative_volume, volume_confirmation,
				touch_count, support_touch_count, resistance_touch_count, reject_count, break_count,
				zone_momentum, zone_direction,
				recent_validation, trading_score, trading_score_breakdown, trading_recommendation,
				overlap_group, confluence_count, status, features, evidence, explanation, scenario
			) VALUES (
				:analysis_id, :price_low, :price_high, :method, :role, :tier, :tier_label,
				:support_score, :resistance_score, :net_score, :net_score_label,
				:confidence, :confidence_level,
				:bounce_probability, :break_probability,
				:expected_gain, :expected_loss, :expected_value, :risk_reward_ratio, :reward_risk_percentile,
				:relative_volume, :volume_confirmation,
				:touch_count, :support_touch_count, :resistance_touch_count, :reject_count, :break_count,
				:zone_momentum, :zone_direction,
				:recent_validation, :trading_score, :trading_score_breakdown, :trading_recommendation,
				:overlap_group, :confluence_count, :status, :features, :evidence, :explanation, :scenario
			)
		`, zones[i]); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

const srZoneAnalysisColumns = `id, symbol, timeframe, analyzed_at, current_price,
	global_trend, global_volatility, global_expected_value, global_confidence, global_risk_reward_ratio,
	model_version, model_config_hash, pipeline_version, evidence, explanation, scenario,
	period_summaries, analysis_tips, chip_summary, decision_summary, created_at`

func (r *srZoneRepo) Get(ctx context.Context, id uint64) (*SRZoneAnalysis, error) {
	var a SRZoneAnalysis
	err := r.db.GetContext(ctx, &a, r.db.Rebind(`
		SELECT `+srZoneAnalysisColumns+` FROM stock_sr_zone_analyses WHERE id=?
	`), id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *srZoneRepo) List(ctx context.Context, symbol string, limit int) ([]SRZoneAnalysis, error) {
	var rows []SRZoneAnalysis
	var err error
	if symbol != "" {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+srZoneAnalysisColumns+`
			FROM stock_sr_zone_analyses WHERE symbol=? ORDER BY created_at DESC LIMIT ?
		`), symbol, limit)
	} else {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+srZoneAnalysisColumns+`
			FROM stock_sr_zone_analyses ORDER BY created_at DESC LIMIT ?
		`), limit)
	}
	return rows, err
}

func (r *srZoneRepo) GetZones(ctx context.Context, analysisID uint64) ([]SRZone, error) {
	var rows []SRZone
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, price_low, price_high, method, role, tier, tier_label,
			support_score, resistance_score, net_score, net_score_label,
			confidence, confidence_level,
			bounce_probability, break_probability,
			expected_gain, expected_loss, expected_value, risk_reward_ratio, reward_risk_percentile,
			relative_volume, volume_confirmation,
			touch_count, support_touch_count, resistance_touch_count, reject_count, break_count,
			zone_momentum, zone_direction,
			recent_validation, trading_score, trading_score_breakdown, trading_recommendation,
			overlap_group, confluence_count,
		       status, broken_at, broken_price, resolved_role, features, evidence, explanation, scenario
		FROM stock_sr_zones WHERE analysis_id=?
		ORDER BY CASE tier WHEN 'TIER_1_MAIN_STRUCTURE' THEN 1 WHEN 'TIER_2_TRADING_ZONE' THEN 2 ELSE 3 END, trading_score DESC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) UpdateZoneStatus(ctx context.Context, zoneID uint64, status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) error {
	var resolvedRoleArg any
	if resolvedRole != "" {
		resolvedRoleArg = resolvedRole
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE stock_sr_zones
		SET status=?, broken_at=?, broken_price=?,
			resolved_role=CASE WHEN role='AT_ZONE' THEN ? ELSE NULL END
		WHERE id=?
	`), status, brokenAt, brokenPrice, resolvedRoleArg, zoneID)
	return err
}

func (r *srZoneRepo) Delete(ctx context.Context, id uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先刪子表（stock_sr_zones 有 FK 指向 stock_sr_zone_analyses），再刪主紀錄
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zones WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zone_analyses WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
}
