package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type SRZoneRepo interface {
	// Create 寫入一筆 zone 評分快照與其所有 zone，回傳新建的 analysis id
	Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone, projections SRZoneNormalizedProjections) (uint64, error)
	Get(ctx context.Context, id uint64) (*SRZoneAnalysis, error)
	List(ctx context.Context, symbol string, limit int) ([]SRZoneAnalysis, error)
	// ListRefsSince 取 created_at >= since 的分析參照（全 symbol），最新的優先，
	// 最多 limit 筆。
	//
	// **給 sr_zone_verify 用，不能用 List("", N) 代替**：那是「最近 N 筆」，
	// 覆蓋窗口會隨 watchlist 大小與每日分析輪數縮短——11 檔 × 兩輪時 50 筆只剩
	// 約 2.3 個交易日（見 docs/architecture.md 的排程說明段）。
	// 這支讓窗口回到「時間」這個穩定的單位。
	//
	// **用 created_at 而不是 analyzed_at**：後者是日期粒度（同一交易日兩輪都寫成
	// 當日 00:00），取不出「最近 N 天實際跑過的分析」。
	//
	// **回傳 Ref 而不是完整分析**：呼叫端（排程迴圈）只用得到 ID 與 Symbol，
	// 而完整型別平均 28 kB／筆，上限 10000 筆時會 OOM（見 docs/architecture.md）。
	ListRefsSince(ctx context.Context, since time.Time, limit int) ([]SRZoneAnalysisRef, error)
	// GetLatestByTimeframe 取某檔某 timeframe 的最新一筆分析。
	//
	// **不能用 List(symbol, 1) 代替**：List 只按 symbol 過濾，使用者今天手動跑過一次 5m
	// 分析，就會讓 1d 的排程誤判「今天已經分析過」而整批跳過（見 todo.md T-052）。
	// 沒有任何分析時回 (nil, nil)——那不是錯誤，是「這檔還沒被分析過」。
	GetLatestByTimeframe(ctx context.Context, symbol, timeframe string) (*SRZoneAnalysis, error)
	GetZones(ctx context.Context, analysisID uint64) ([]SRZone, error)
	GetDecision(ctx context.Context, analysisID uint64) (*SRDecision, error)
	GetMarketEventDetections(ctx context.Context, analysisID uint64) ([]MarketEventDetection, error)
	GetMarketEventStates(ctx context.Context, analysisID uint64) ([]MarketEventState, error)
	GetLatestMarketEventStates(ctx context.Context, symbol, timeframe string) ([]MarketEventState, error)
	// ListMarketEventStateHistory 取一段歷史的事件狀態快照，供 Event Timeline 摺疊成事件鏈
	// （todo.md T-045 P1）。**不能用上面兩支代替**：GetMarketEventStates 只看單次分析、
	// GetLatestMarketEventStates 只取最新一批，兩者都給不出跨分析的序列。
	ListMarketEventStateHistory(ctx context.Context, opts MarketEventStateHistoryOptions) ([]MarketEventState, error)
	// ListAnalysisSnapshots 取一段期間**所有**分析的時間點（不論有沒有產生事件）。
	// Event Timeline 的 gap 揭露必須依賴它——只看 market_event_states 會漏掉
	// 「跑了分析但沒有任何事件」的那幾次，把它們誤報成沒有觀測。
	ListAnalysisSnapshots(ctx context.Context, opts MarketEventStateHistoryOptions) ([]AnalysisSnapshot, error)
	GetDailyCandidates(ctx context.Context, analysisID uint64) ([]SRDailyCandidate, error)
	GetModelGovernance(ctx context.Context, analysisID uint64) (*SRModelGovernance, error)
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

func (r *srZoneRepo) Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone, projections SRZoneNormalizedProjections) (uint64, error) {
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
	if a.ZoneBuilderRuntimeConfig == "" {
		a.ZoneBuilderRuntimeConfig = RawJSON("null")
	}
	if a.ProbabilityContext == "" {
		a.ProbabilityContext = RawJSON("null")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const cols = `symbol, timeframe, analyzed_at, current_price,
		global_trend, global_volatility, global_expected_value, global_confidence, global_risk_reward_ratio,
		model_version, model_config_hash, pipeline_version, evidence, explanation, scenario, probability_context,
		period_summaries, analysis_tips, chip_summary, decision_summary, zone_builder_runtime_config`

	var id uint64
	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
			RETURNING id
		`,
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice,
			a.GlobalTrend, a.GlobalVolatility, a.GlobalExpectedValue, a.GlobalConfidence, a.GlobalRiskRewardRatio,
			a.ModelVersion, a.ModelConfigHash, a.PipelineVersion, a.Evidence, a.Explanation, a.Scenario, a.ProbabilityContext,
			a.PeriodSummaries, a.AnalysisTips, a.ChipSummary, a.DecisionSummary, a.ZoneBuilderRuntimeConfig,
		).Scan(&id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		`),
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice,
			a.GlobalTrend, a.GlobalVolatility, a.GlobalExpectedValue, a.GlobalConfidence, a.GlobalRiskRewardRatio,
			a.ModelVersion, a.ModelConfigHash, a.PipelineVersion, a.Evidence, a.Explanation, a.Scenario, a.ProbabilityContext,
			a.PeriodSummaries, a.AnalysisTips, a.ChipSummary, a.DecisionSummary, a.ZoneBuilderRuntimeConfig,
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
		if zones[i].ProbabilityContext == "" {
			zones[i].ProbabilityContext = RawJSON("null")
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
				overlap_group, confluence_count, status, features, evidence, explanation, scenario, probability_context,
				zone_uid
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
				:overlap_group, :confluence_count, :status, :features, :evidence, :explanation, :scenario, :probability_context,
				:zone_uid
			)
		`, zones[i]); err != nil {
			return 0, err
		}
	}

	if projections.Decision != nil {
		projections.Decision.AnalysisID = id
		projections.Decision.Symbol = a.Symbol
		projections.Decision.Timeframe = a.Timeframe
		projections.Decision.AnalyzedAt = a.AnalyzedAt
		if projections.Decision.ReasonCodes == "" {
			projections.Decision.ReasonCodes = RawJSON("[]")
		}
		if projections.Decision.DecisionSummary == "" {
			projections.Decision.DecisionSummary = RawJSON("null")
		}
		defaultSRDecisionDetailJSON(projections.Decision)
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_sr_decisions (
				analysis_id, symbol, timeframe, analyzed_at,
				market_bias, entry_permission_state, position_action, price_path_state,
				model_health_state, event_market_state, reason_codes,
				market_regime_json, data_quality_json, decision_derived_view_json,
				event_sequence_json, daily_price_action_json,
				price_path_json, daily_confirmation_json, defense_lines_json,
				entry_executability_json, entry_blocking_zone_json, rr_context_json,
				rr_gate_json, position_action_condition_json, market_context_json,
				confidence_explanation_json, risk_notes_json, zone_summaries_json, decision_summary
			) VALUES (
				:analysis_id, :symbol, :timeframe, :analyzed_at,
				:market_bias, :entry_permission_state, :position_action, :price_path_state,
				:model_health_state, :event_market_state, :reason_codes,
				:market_regime_json, :data_quality_json, :decision_derived_view_json,
				:event_sequence_json, :daily_price_action_json,
				:price_path_json, :daily_confirmation_json, :defense_lines_json,
				:entry_executability_json, :entry_blocking_zone_json, :rr_context_json,
				:rr_gate_json, :position_action_condition_json, :market_context_json,
				:confidence_explanation_json, :risk_notes_json, :zone_summaries_json, :decision_summary
			)
		`, projections.Decision); err != nil {
			return 0, err
		}
	}

	for i := range projections.EventDetections {
		projections.EventDetections[i].AnalysisID = id
		projections.EventDetections[i].Symbol = a.Symbol
		projections.EventDetections[i].Timeframe = a.Timeframe
		projections.EventDetections[i].AnalyzedAt = a.AnalyzedAt
		if projections.EventDetections[i].ReasonCodes == "" {
			projections.EventDetections[i].ReasonCodes = RawJSON("[]")
		}
		if projections.EventDetections[i].EventJSON == "" {
			projections.EventDetections[i].EventJSON = RawJSON("null")
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO market_event_detections (
				analysis_id, symbol, timeframe, analyzed_at,
				event_key, event_type, event_family, event_scope, zone_key,
				direction, state, active, confidence, price_level, reason_codes, event_json
			) VALUES (
				:analysis_id, :symbol, :timeframe, :analyzed_at,
				:event_key, :event_type, :event_family, :event_scope, :zone_key,
				:direction, :state, :active, :confidence, :price_level, :reason_codes, :event_json
			)
		`, projections.EventDetections[i]); err != nil {
			return 0, err
		}
	}

	for i := range projections.EventStates {
		projections.EventStates[i].AnalysisID = id
		projections.EventStates[i].Symbol = a.Symbol
		projections.EventStates[i].Timeframe = a.Timeframe
		projections.EventStates[i].AnalyzedAt = a.AnalyzedAt
		if projections.EventStates[i].ReasonCodes == "" {
			projections.EventStates[i].ReasonCodes = RawJSON("[]")
		}
		if projections.EventStates[i].StateJSON == "" {
			projections.EventStates[i].StateJSON = RawJSON("null")
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO market_event_states (
				analysis_id, symbol, timeframe, analyzed_at,
				event_key, event_type, event_family, event_scope, zone_key,
				root_event_type, latest_event_type, direction, state, active,
				resolved_by, confidence, price_level, reason_codes, state_json
			) VALUES (
				:analysis_id, :symbol, :timeframe, :analyzed_at,
				:event_key, :event_type, :event_family, :event_scope, :zone_key,
				:root_event_type, :latest_event_type, :direction, :state, :active,
				:resolved_by, :confidence, :price_level, :reason_codes, :state_json
			)
		`, projections.EventStates[i]); err != nil {
			return 0, err
		}
	}

	for i := range projections.DailyCandidates {
		projections.DailyCandidates[i].AnalysisID = id
		projections.DailyCandidates[i].Symbol = a.Symbol
		projections.DailyCandidates[i].Timeframe = a.Timeframe
		projections.DailyCandidates[i].AnalyzedAt = a.AnalyzedAt
		if projections.DailyCandidates[i].EventRefs == "" {
			projections.DailyCandidates[i].EventRefs = RawJSON("[]")
		}
		if projections.DailyCandidates[i].CandidateJSON == "" {
			projections.DailyCandidates[i].CandidateJSON = RawJSON("null")
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_sr_daily_candidates (
				analysis_id, symbol, timeframe, analyzed_at,
				price_low, price_high, label, role, source, lifecycle, decision_role,
				distance_pct, distance_label, reason, event_refs, candidate_json
			) VALUES (
				:analysis_id, :symbol, :timeframe, :analyzed_at,
				:price_low, :price_high, :label, :role, :source, :lifecycle, :decision_role,
				:distance_pct, :distance_label, :reason, :event_refs, :candidate_json
			)
		`, projections.DailyCandidates[i]); err != nil {
			return 0, err
		}
	}

	if projections.ModelGovernance != nil {
		projections.ModelGovernance.AnalysisID = id
		projections.ModelGovernance.Symbol = a.Symbol
		projections.ModelGovernance.Timeframe = a.Timeframe
		projections.ModelGovernance.AnalyzedAt = a.AnalyzedAt
		projections.ModelGovernance.ModelVersion = a.ModelVersion
		projections.ModelGovernance.ModelConfigHash = a.ModelConfigHash
		if projections.ModelGovernance.QualityFlags == "" {
			projections.ModelGovernance.QualityFlags = RawJSON("[]")
		}
		if projections.ModelGovernance.WarningFlags == "" {
			projections.ModelGovernance.WarningFlags = RawJSON("[]")
		}
		if projections.ModelGovernance.BlockingFlags == "" {
			projections.ModelGovernance.BlockingFlags = RawJSON("[]")
		}
		if projections.ModelGovernance.ConfidenceGateJSON == "" {
			projections.ModelGovernance.ConfidenceGateJSON = RawJSON("null")
		}
		if projections.ModelGovernance.CalibrationReportJSON == "" {
			projections.ModelGovernance.CalibrationReportJSON = RawJSON("null")
		}
		if projections.ModelGovernance.WalkForwardReportJSON == "" {
			projections.ModelGovernance.WalkForwardReportJSON = RawJSON("null")
		}
		if projections.ModelGovernance.DatasetDiagnosticsJSON == "" {
			projections.ModelGovernance.DatasetDiagnosticsJSON = RawJSON("null")
		}
		if projections.ModelGovernance.GovernanceJSON == "" {
			projections.ModelGovernance.GovernanceJSON = RawJSON("null")
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_sr_model_governance (
				analysis_id, symbol, timeframe, analyzed_at, model_version, model_config_hash,
				health_state, average_edge_pp, directional_zone_count, zone_count,
				allow_entry, max_entry_state, quality_flags, warning_flags, blocking_flags,
				confidence_gate_json, calibration_report_json, walk_forward_report_json,
				dataset_diagnostics_json, governance_json
			) VALUES (
				:analysis_id, :symbol, :timeframe, :analyzed_at, :model_version, :model_config_hash,
				:health_state, :average_edge_pp, :directional_zone_count, :zone_count,
				:allow_entry, :max_entry_state, :quality_flags, :warning_flags, :blocking_flags,
				:confidence_gate_json, :calibration_report_json, :walk_forward_report_json,
				:dataset_diagnostics_json, :governance_json
			)
		`, projections.ModelGovernance); err != nil {
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
	model_version, model_config_hash, pipeline_version, evidence, explanation, scenario, probability_context,
	period_summaries, analysis_tips, chip_summary, decision_summary, zone_builder_runtime_config, created_at`

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

func (r *srZoneRepo) GetLatestByTimeframe(ctx context.Context, symbol, timeframe string) (*SRZoneAnalysis, error) {
	var a SRZoneAnalysis
	err := r.db.GetContext(ctx, &a, r.db.Rebind(`
		SELECT `+srZoneAnalysisColumns+`
		FROM stock_sr_zone_analyses WHERE symbol=? AND timeframe=?
		ORDER BY created_at DESC LIMIT 1
	`), symbol, timeframe)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
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

// ListRefsSince 由 idx_stock_sr_zone_analyses_created_at 支撐（migration 073）——
// 既有的 (symbol, created_at DESC) 索引 leading column 是 symbol，這條查詢不帶 symbol，
// 走不到它。
//
// **ORDER BY 帶 id DESC 是必要的**，理由與 job_run_repo 的 GetLatestPerJob 相同：
// created_at 只有秒級精度（mysql DATETIME(0)、sqlite CURRENT_TIMESTAMP），
// 同一輪分析的多檔很容易落在同一秒。沒有 id 決勝時，同秒的那批由資料庫任意排序，
// **撞到 limit 時邊界那幾筆會在不同引擎、不同執行計畫之間漂移**——某筆分析可能就
// 一直輪不到驗證，正是「覆蓋窗口會隨資料成長縮短」那個問題的變形。
//
// **只 SELECT id, symbol，不要改回 srZoneAnalysisColumns**
// （理由與實測見 docs/architecture.md 的排程說明段）：
// 那份欄位清單含九個 RawJSON 欄位、平均 28 kB／筆，上限 10000 筆時一次載入約
// 276 MB，實測 256MB 與 512MB 的 container 都會被 OOM kill。呼叫端只用得到這兩欄，
// 其餘由 Verify 自己重查。symbol 不在索引裡所以要回表，但一萬次回表遠比 276 MB 便宜。
func (r *srZoneRepo) ListRefsSince(ctx context.Context, since time.Time, limit int) ([]SRZoneAnalysisRef, error) {
	var rows []SRZoneAnalysisRef
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, symbol
		FROM stock_sr_zone_analyses WHERE created_at >= ?
		ORDER BY created_at DESC, id DESC LIMIT ?
	`), since, limit)
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
		       status, broken_at, broken_price, resolved_role, features, evidence, explanation, scenario, probability_context,
		       zone_uid
		FROM stock_sr_zones WHERE analysis_id=?
		ORDER BY CASE tier WHEN 'TIER_1_MAIN_STRUCTURE' THEN 1 WHEN 'TIER_2_TRADING_ZONE' THEN 2 ELSE 3 END, trading_score DESC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) GetDecision(ctx context.Context, analysisID uint64) (*SRDecision, error) {
	var row SRDecision
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			market_bias, entry_permission_state, position_action, price_path_state,
			model_health_state, event_market_state, reason_codes,
			market_regime_json, data_quality_json, decision_derived_view_json,
			event_sequence_json, daily_price_action_json,
			price_path_json, daily_confirmation_json, defense_lines_json,
			entry_executability_json, entry_blocking_zone_json, rr_context_json,
			rr_gate_json, position_action_condition_json, market_context_json,
			confidence_explanation_json, risk_notes_json, zone_summaries_json, decision_summary, created_at
		FROM stock_sr_decisions WHERE analysis_id=?
	`), analysisID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func defaultSRDecisionDetailJSON(decision *SRDecision) {
	if decision.MarketRegimeJSON == "" {
		decision.MarketRegimeJSON = RawJSON("null")
	}
	if decision.DataQualityJSON == "" {
		decision.DataQualityJSON = RawJSON("null")
	}
	if decision.DecisionDerivedViewJSON == "" {
		decision.DecisionDerivedViewJSON = RawJSON("null")
	}
	if decision.EventSequenceJSON == "" {
		decision.EventSequenceJSON = RawJSON("[]")
	}
	if decision.DailyPriceActionJSON == "" {
		decision.DailyPriceActionJSON = RawJSON("null")
	}
	if decision.PricePathJSON == "" {
		decision.PricePathJSON = RawJSON("null")
	}
	if decision.DailyConfirmationJSON == "" {
		decision.DailyConfirmationJSON = RawJSON("null")
	}
	if decision.DefenseLinesJSON == "" {
		decision.DefenseLinesJSON = RawJSON("null")
	}
	if decision.EntryExecutabilityJSON == "" {
		decision.EntryExecutabilityJSON = RawJSON("null")
	}
	if decision.EntryBlockingZoneJSON == "" {
		decision.EntryBlockingZoneJSON = RawJSON("null")
	}
	if decision.RRContextJSON == "" {
		decision.RRContextJSON = RawJSON("null")
	}
	if decision.RRGateJSON == "" {
		decision.RRGateJSON = RawJSON("null")
	}
	if decision.PositionActionConditionJSON == "" {
		decision.PositionActionConditionJSON = RawJSON("null")
	}
	if decision.MarketContextJSON == "" {
		decision.MarketContextJSON = RawJSON("[]")
	}
	if decision.ConfidenceExplanationJSON == "" {
		decision.ConfidenceExplanationJSON = RawJSON("null")
	}
	if decision.RiskNotesJSON == "" {
		decision.RiskNotesJSON = RawJSON("[]")
	}
	if decision.ZoneSummariesJSON == "" {
		decision.ZoneSummariesJSON = RawJSON(`{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}`)
	}
}

func (r *srZoneRepo) GetMarketEventDetections(ctx context.Context, analysisID uint64) ([]MarketEventDetection, error) {
	var rows []MarketEventDetection
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			event_key, event_type, event_family, event_scope, zone_key,
			direction, state, active, confidence, price_level, reason_codes, event_json, created_at
		FROM market_event_detections WHERE analysis_id=?
		ORDER BY id ASC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) GetMarketEventStates(ctx context.Context, analysisID uint64) ([]MarketEventState, error) {
	var rows []MarketEventState
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			event_key, event_type, event_family, event_scope, zone_key,
			root_event_type, latest_event_type, direction, state, active,
			resolved_by, confidence, price_level, reason_codes, state_json, created_at
		FROM market_event_states WHERE analysis_id=?
		ORDER BY id ASC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) GetLatestMarketEventStates(ctx context.Context, symbol, timeframe string) ([]MarketEventState, error) {
	var rows []MarketEventState
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			event_key, event_type, event_family, event_scope, zone_key,
			root_event_type, latest_event_type, direction, state, active,
			resolved_by, confidence, price_level, reason_codes, state_json, created_at
		FROM market_event_states
		WHERE symbol=? AND timeframe=?
			AND analysis_id = (
				SELECT analysis_id FROM market_event_states
				WHERE symbol=? AND timeframe=?
				ORDER BY analyzed_at DESC, analysis_id DESC
				LIMIT 1
			)
		ORDER BY id ASC
	`), symbol, timeframe, symbol, timeframe)
	return rows, err
}

// AnalysisSnapshot 只帶 timeline 需要的兩個欄位，避免為了畫 gap 就拉整份分析。
type AnalysisSnapshot struct {
	ID         uint64    `db:"id"          json:"analysis_id"`
	AnalyzedAt time.Time `db:"analyzed_at" json:"analyzed_at"`
}

// MarketEventStateHistoryOptions 的 From／To 為零值代表不限。
type MarketEventStateHistoryOptions struct {
	Symbol    string
	Timeframe string
	From      time.Time
	To        time.Time
	// MaxAnalyses 限制回溯幾次分析（不是幾列）。0 代表用預設值。
	// 以分析次數而非列數為單位，是因為截斷在列的中間會切出半份快照，
	// 摺疊時會產生不存在的 transition。
	MaxAnalyses int
}

const (
	defaultTimelineMaxAnalyses = 60
	maxTimelineMaxAnalyses     = 500
)

func (r *srZoneRepo) ListMarketEventStateHistory(ctx context.Context, opts MarketEventStateHistoryOptions) ([]MarketEventState, error) {
	limit := opts.MaxAnalyses
	if limit <= 0 {
		limit = defaultTimelineMaxAnalyses
	}
	if limit > maxTimelineMaxAnalyses {
		limit = maxTimelineMaxAnalyses
	}

	where := []string{"symbol=?", "timeframe=?"}
	args := []any{opts.Symbol, opts.Timeframe}
	if !opts.From.IsZero() {
		where = append(where, "analyzed_at >= ?")
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		where = append(where, "analyzed_at <= ?")
		args = append(args, opts.To)
	}
	filter := strings.Join(where, " AND ")

	// 先挑出要回溯的那幾次分析，再撈它們的全部狀態列。
	// **不能直接對狀態列 LIMIT**：那會在快照中間截斷，摺疊時把「被切掉的事件」
	// 誤判成消失，產生不存在的 transition。
	args = append(args, limit)
	query := `
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			event_key, event_type, event_family, event_scope, zone_key,
			root_event_type, latest_event_type, direction, state, active,
			resolved_by, confidence, price_level, reason_codes, state_json, created_at
		FROM market_event_states
		WHERE ` + filter + `
			AND analysis_id IN (
				-- 多包一層 derived table 是**必要的**，不是多餘：MySQL 至今仍拒絕
				-- IN/ALL/ANY/SOME 子查詢裡直接用 LIMIT
				-- （ERROR 1235: This version of MySQL doesn't yet support
				-- 'LIMIT & IN/ALL/ANY/SOME subquery'），包成 derived table 才會過。
				-- 同檔的 GetLatestMarketEventStates 用的是純量形式 = (SELECT … LIMIT 1)，
				-- 那個 MySQL 允許，所以它不需要這層。
				SELECT * FROM (
					SELECT analysis_id FROM market_event_states
					WHERE ` + filter + `
					GROUP BY analysis_id
					ORDER BY MAX(analyzed_at) DESC, analysis_id DESC
					LIMIT ?
				) AS recent_analyses
			)
		ORDER BY analyzed_at ASC, analysis_id ASC, zone_key ASC, event_family ASC, id ASC`

	// filter 出現兩次，參數也要帶兩份；LIMIT 的參數排在最後。
	full := make([]any, 0, len(args)*2)
	full = append(full, args[:len(args)-1]...)
	full = append(full, args[:len(args)-1]...)
	full = append(full, args[len(args)-1])

	var rows []MarketEventState
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), full...); err != nil {
		return nil, fmt.Errorf("list market event state history: %w", err)
	}
	return rows, nil
}

func (r *srZoneRepo) ListAnalysisSnapshots(ctx context.Context, opts MarketEventStateHistoryOptions) ([]AnalysisSnapshot, error) {
	limit := opts.MaxAnalyses
	if limit <= 0 {
		limit = defaultTimelineMaxAnalyses
	}
	if limit > maxTimelineMaxAnalyses {
		limit = maxTimelineMaxAnalyses
	}

	where := []string{"symbol=?", "timeframe=?"}
	args := []any{opts.Symbol, opts.Timeframe}
	if !opts.From.IsZero() {
		where = append(where, "analyzed_at >= ?")
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		where = append(where, "analyzed_at <= ?")
		args = append(args, opts.To)
	}
	args = append(args, limit)

	// 先取最近 N 筆再依時間遞增排回來——直接 ORDER BY ASC + LIMIT 會拿到最舊的 N 筆。
	var rows []AnalysisSnapshot
	query := `
		SELECT id, analyzed_at FROM (
			SELECT id, analyzed_at FROM stock_sr_zone_analyses
			WHERE ` + strings.Join(where, " AND ") + `
			ORDER BY analyzed_at DESC, id DESC
			LIMIT ?
		) AS recent
		ORDER BY analyzed_at ASC, id ASC`
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list analysis snapshots: %w", err)
	}
	return rows, nil
}

func (r *srZoneRepo) GetDailyCandidates(ctx context.Context, analysisID uint64) ([]SRDailyCandidate, error) {
	var rows []SRDailyCandidate
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at,
			price_low, price_high, label, role, source, lifecycle, decision_role,
			distance_pct, distance_label, reason, event_refs, candidate_json, created_at
		FROM stock_sr_daily_candidates WHERE analysis_id=?
		ORDER BY id ASC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) GetModelGovernance(ctx context.Context, analysisID uint64) (*SRModelGovernance, error) {
	var row SRModelGovernance
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at, model_version, model_config_hash,
			health_state, average_edge_pp, directional_zone_count, zone_count,
			allow_entry, max_entry_state, quality_flags, warning_flags, blocking_flags,
			confidence_gate_json, calibration_report_json, walk_forward_report_json,
			dataset_diagnostics_json, governance_json, created_at
		FROM stock_sr_model_governance WHERE analysis_id=?
	`), analysisID)
	if err != nil {
		return nil, err
	}
	return &row, nil
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

	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM market_event_states WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM market_event_detections WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_decisions WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_daily_candidates WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_model_governance WHERE analysis_id=?`), id); err != nil {
		return err
	}
	// 先刪子表（stock_sr_zones 有 FK 指向 stock_sr_zone_analyses），再刪主紀錄
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zones WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zone_analyses WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
}
