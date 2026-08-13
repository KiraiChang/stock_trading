import { apiFetch } from './client'

export type NetScoreLabel = 'STRONG_SUPPORT' | 'NEUTRAL' | 'STRONG_RESISTANCE'
export type ConfidenceLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'VERY_HIGH'
export type RecentValidation = 'VALIDATED_RECENTLY' | 'PENDING_VALIDATION' | 'NOT_TESTED_RECENTLY' | 'EXPIRED'
export type VolumeConfirmation = 'CONFIRMED' | 'WEAK' | 'NEUTRAL' | 'FAILED'
export type ZoneDirection = 'UP' | 'DOWN' | 'FLAT'
export type TradingRecommendation = 'STRONG_BUY' | 'BUY' | 'WATCH' | 'NEUTRAL' | 'AVOID' | 'STRONG_SELL'
export type ZoneTier = 'TIER_1_MAIN_STRUCTURE' | 'TIER_2_TRADING_ZONE' | 'TIER_3_SHORT_TERM'

// trading_score 的可拆解分量（加權貢獻值，六個分量加總 = trading_score）：
// Score = EV(34%) + RR(17%) + Trend(12.75%) + Volume(12.75%) + Confidence(8.5%) + Chip(15%)
// 【2026-07 籌碼分析整合】新增 chip 分量後，其餘五個分量依原比例縮小
// （40/20/15/15/10 → 34/17/12.75/12.75/8.5），見後端
// sr_scoring/scoring.py::TRADING_SCORE_WEIGHTS。
export interface TradingScoreBreakdown {
  expected_value: number
  risk_reward: number
  trend: number
  volume: number
  confidence: number
  chip: number
}

export interface SRShapContribution {
  feature: string
  value: number
  contribution: number
  direction: 'supportive' | 'opposing' | 'neutral'
}

export interface SRShapTargetEvidence {
  baseline_probability: number
  final_probability: number
  additivity_error: number
  contributions: SRShapContribution[]
}

export interface SRDirectionEvidence {
  role: 'SUPPORT' | 'RESISTANCE'
  targets: {
    hold: SRShapTargetEvidence
    break: SRShapTargetEvidence
  }
}

export interface SRZoneEvidence {
  price_low: number
  price_high: number
  support: SRDirectionEvidence
  resistance: SRDirectionEvidence
  risk_flags: string[]
}

export interface SRZoneExplanation {
  schema_version: 'sr_explain_v1'
  role_summary: string
  score_reason: string
  probability_reason: string
  confidence_reason: string
  positive_factors: string[]
  negative_factors: string[]
  watch_conditions: string[]
}

export interface SRAnalysisExplanation {
  schema_version: 'sr_explain_v1'
  summary: string
  action_reason: string
  market_drivers: string[]
  risk_notes: string[]
  model_context: {
    version: string
    config_hash: string
    uses_shap_evidence: boolean
  }
}

export interface SRScenario {
  schema_version: 'sr_scenario_v1'
  state: string
  title: string
  summary: string
  trigger_conditions: string[]
  invalidation_conditions: string[]
  // market_regime / primary_zone 不再由 scenario 提供（改讀 decision_summary）
  global_confidence?: number | null
}

export interface SRProbabilityModelMetrics {
  auc: number | null
  brier_score: number | null
  log_loss: number | null
  calibrated: number | null
  test_rows: number | null
}

export interface SRProbabilityContext {
  schema_version: 'sr_probability_context_v1'
  bounce_probability?: number | null
  break_probability?: number | null
  neutral_probability?: number | null
  dominant_outcome?: 'BOUNCE' | 'BREAK' | 'NEUTRAL' | 'NO_DIRECTION' | string
  edge_pp?: number | null
  quality_flags?: string[]
  model_metrics?: {
    hold: SRProbabilityModelMetrics
    break: SRProbabilityModelMetrics
  }
  health?: {
    quality_flags: string[]
    warning_flags?: string[]
    blocking_flags?: string[]
    health_state?: 'HEALTHY' | 'DEGRADED' | 'UNRELIABLE' | string
    average_edge_pp: number | null
    directional_zone_count: number
    zone_count: number
    confidence_gate?: {
      state?: string
      allow_entry?: boolean
      max_entry_state?: string
      reason_codes?: string[]
    }
    reports?: Record<string, unknown>
  }
  model_reports?: Record<string, unknown>
}

export interface SRGlobalEvidence {
  trend: number
  volatility: number
  metrics: {
    expected_value: number | null
    confidence: number | null
    risk_reward_ratio: number | null
  }
  chip: SRChipSummary
  model: {
    version: string
    config_hash: string
    explainer: 'permutation_shap'
    explained_output: 'calibrated_normalized_probability'
  }
}

export interface SRValidationDebug {
  analysis_date?: string | null
  zone_generation_end_date?: string | null
  validation_start_date?: string | null
  validation_end_date?: string | null
  latest_touch_bar_date?: string | null
  latest_validation_bar_date?: string | null
  latest_touch_index?: number | null
  latest_validation_index?: number | null
}

// 機構級版本（2026-07 重新設計，見後端 sr_scoring/scoring.py 開頭的完整說明）
export interface SRZone {
  id: number
  analysis_id: number
  price_low: number
  price_high: number
  method: 'atr' | 'volume_profile' | 'recent_pivot' | 'breakdown_reclaim' | 'vwap_reclaim'
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'

  // tier/tier_label：zone 依寬度在同一次分析裡的相對排名分三層，讓 zone
  // 清單「可排序」（主結構 → 交易區 → 短期）。display_label 才包含支撐/壓力角色。
  tier: ZoneTier
  tier_label: string
  role_label?: string
  display_label?: string

  support_score: number
  resistance_score: number
  net_score: number
  net_score_label: NetScoreLabel

  confidence: number
  confidence_level: ConfidenceLevel

  bounce_probability: number | null
  break_probability: number | null
  // expected_gain/expected_loss 是角色解析後的平均反彈/跌破報酬；
  // expected_value = 反彈機率×expected_gain + 跌破機率×expected_loss
  expected_gain: number | null
  expected_loss: number | null
  expected_value: number | null
  risk_reward_ratio: number | null
  reward_risk_percentile: number | null

  relative_volume: number | null
  volume_confirmation: VolumeConfirmation | null

  touch_count: number
  support_touch_count: number
  resistance_touch_count: number
  reject_count: number
  break_count: number

  zone_momentum: number
  zone_direction: ZoneDirection

  recent_validation: RecentValidation

  // trading_score = trading_score_breakdown 六個分量加總，可拆解檢視每個
  // 分量各佔多少分（見十三、Score 必須可拆解；chip 分量見籌碼分析整合）。
  trading_score: number
  trading_score_breakdown: TradingScoreBreakdown
  trading_recommendation: TradingRecommendation
  zone_quality_score?: number | null
  entry_relevance_score?: number | null
  entry_relevance_breakdown?: Record<string, number> | null
  validation_debug?: SRValidationDebug | null

  // 跨方法（ATR/volume_profile）重疊分群：overlap_group 相同的 zone 代表
  // 不同方法都指向同一個價位帶（「多方法共振」），不會合併或刪除任何
  // zone。confluence_count 恆 >= 1；overlap_group 只有 confluence_count > 1
  // 時才有值。
  overlap_group: number | null
  confluence_count: number
  confluence_family_count?: number | null
  confluence_families?: string[] | null

  status: 'PENDING' | 'HELD_SO_FAR' | 'BROKEN'
  broken_at?: string | null
  broken_price?: number | null

  // 只有 role='AT_ZONE' 的 zone 在後續驗證（POST /sr-zones/:id/verify）時，
  // 價格真正收盤離開區間後才會被解析並寫入 SUPPORT 或 RESISTANCE；
  // role != 'AT_ZONE' 的 zone 永遠是 null（角色從分析當下就已明確）。role
  // 本身分析後不會再變動，判斷「這個 zone 現在算支撐還是壓力」應優先看
  // resolved_role，沒有值再退回 role。
  resolved_role?: 'SUPPORT' | 'RESISTANCE' | null
  features?: {
    support: Record<string, number>
    resistance: Record<string, number>
  } | null
  evidence?: SRZoneEvidence | null
  explanation?: SRZoneExplanation | null
  scenario?: SRScenario | null
  probability_context?: SRProbabilityContext | null
}

export type SRPeriodKey = 'short' | 'mid' | 'long'

export type ChipDirection = 'bullish' | 'bearish' | 'neutral' | 'none'

// 每張支撐/壓力摘要卡的角色化籌碼一行（見後端 _zone_summary 的 chip 欄位）。
// direction：整檔原始方向（未翻號）。contribution：籌碼對這個角色 trading_score
// 的直接加權貢獻（0~15，已依支撐/壓力翻號）。bounce/break_delta_pp：籌碼相對
// 中性籌碼對本 zone 反彈/跌破機率的邊際貢獻（百分點，模型路徑）；查無籌碼
// 資料時為 null。contribution 與 delta 分屬「直接權重」與「模型」兩條路徑。
export interface SRZoneChip {
  direction: ChipDirection
  contribution: number | null
  bounce_delta_pp: number | null
  break_delta_pp: number | null
}

export interface SRZoneSummaryItem {
  price_low: number
  price_high: number
  label: string
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  side: 'support' | 'resistance'
  tier: ZoneTier
  tier_label: string
  role_label?: string
  display_label?: string
  confidence: number
  confidence_level: ConfidenceLevel
  trading_score: number
  recent_validation: RecentValidation
  volume_confirmation: VolumeConfirmation | null
  confluence_count: number
  confluence_family_count?: number | null
  confluence_families?: string[] | null
  chip?: SRZoneChip
  reasons: string[]
}

// 整檔層級籌碼拆解（見後端 _build_chip_summary），供「共用籌碼面板」一次顯示。
// missing=true 代表查無籌碼資料（與 score 接近 0 的「中性」不同），此時各分數為
// null。分數範圍：score/institutional/margin/broker 為 -100~100，concentration 為
// 0~100。這是分析快照當下對齊的籌碼，不是即時最新值。
export interface SRChipSummary {
  missing: boolean
  score: number | null
  raw_score?: number | null
  effective_score?: number | null
  coverage?: number
  confidence?: number
  confidence_level?: string | null
  signal: 'BULLISH' | 'WEAK_BULLISH' | 'BEARISH' | 'WEAK_BEARISH' | 'NEUTRAL' | 'RISK' | null
  source_signal?: string | null
  institutional_score: number | null
  margin_score: number | null
  broker_score: number | null
  concentration_score: number | null
}

export interface SRPeriodSummary {
  key: SRPeriodKey
  label: string
  tier: ZoneTier
  support: SRZoneSummaryItem | null
  resistance: SRZoneSummaryItem | null
  support_note?: string
  resistance_note?: string
}

export type SRDecisionAction = 'Buy' | 'BuySmall' | 'Hold' | 'Avoid'
export type SREntryActionState = 'BLOCKED' | 'WAIT_CONFIRMATION' | 'PROBE_ENTRY' | 'SMALL_ENTRY' | 'ACCUMULATE' | 'BUY' | string
export type SRMarketBias = 'BULLISH_BIAS' | 'NEUTRAL_BIAS' | 'BEARISH_BIAS' | 'REVERSAL_BIAS' | string
export type SRMarketAction = 'WATCH' | 'BUY_SMALL' | 'BUY' | 'AVOID'
export type SRPositionAction = 'HOLD' | 'REDUCE_ON_BREAKDOWN' | 'REDUCE' | 'EXIT'
export type SRMarketRegimePrimary = 'TREND_UP' | 'TREND_DOWN' | 'RANGE_BOUND'
export type SRShortTermRegime = 'NORMAL' | 'BREAKDOWN_RISK' | 'RECLAIM_ATTEMPT' | 'REVERSAL_CANDIDATE' | string
export type SRStructureState =
  | 'NORMAL'
  | 'SUPPORT_RECLAIM_CANDIDATE'
  | 'SUPPORT_RECLAIM_CONFIRMED'
  | 'SUPPORT_RECLAIM_INVALIDATED'
  | 'BREAKDOWN'
export type SRVolatilityState = 'NORMAL' | 'HIGH_VOLATILITY'
export type SRMarketRegimeFlag = 'LOW_CONFIDENCE' | 'HIGH_VOLATILITY' | 'MODEL_UNRELIABLE' | 'MODEL_DEGRADED' | string

export interface SRMarketRegime {
  primary: SRMarketRegimePrimary
  trend_regime?: SRMarketRegimePrimary
  structural_trend?: SRMarketRegimePrimary
  short_term_regime?: SRShortTermRegime
  market_state?: string
  tactical_regime?: SRShortTermRegime
  structure_state?: SRStructureState
  recovery_state?: SRStructureState | string
  volatility_state?: SRVolatilityState
  flags: SRMarketRegimeFlag[]
  label: string
  reasons: string[]
}

export interface SRDecisionContextItem {
  key: string
  label: string
  value: string
  effect?: string | null
}

export interface SRZoneInteraction {
  distance_pct: number
  distance_label: string
  touched: boolean
  penetration_pct: number
  closed_inside: boolean
  closed_above: boolean
  closed_below: boolean
  state_label: string
  price_action_evidence?: {
    reclaim_type: string
    rejection_type: string
    penetration_ratio: number
    close_relative_to_zone: string
    follow_through: string
    touched: boolean
    closed_above: boolean
    closed_below: boolean
  }
}

export interface SRDecisionZoneSummary {
  price_low: number
  price_high: number
  label: string
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  tier: ZoneTier
  tier_label: string
  role_label?: string
  display_label?: string
  trading_score: number
  zone_quality_score?: number | null
  structural_score?: number | null
  entry_relevance_score?: number | null
  decision_relevance_score?: number | null
  tradability_score?: number | null
  entry_relevance_breakdown?: Record<string, number> | null
  confidence: number
  confidence_level: ConfidenceLevel
  expected_value: number | null
  risk_reward_ratio: number | null
  distance_pct: number
  distance_label: string
  zone_width_pct?: number
  zone_width_penalty?: number
  zone_interaction?: SRZoneInteraction
  recent_validation: RecentValidation
  volume_confirmation: VolumeConfirmation | null
  confluence_count: number
  confluence_family_count?: number | null
  confluence_families?: string[] | null
  source?: string
  /**
   * zone **本身**的健康度（`CANDIDATE` / `VALIDATED` / `CONFIRMED` / `WEAKENING` /
   * `BROKEN` / `INVALIDATED`）。與 `semantic_pipeline.lifecycle_phase`（整體事件演進）
   * 是不同的軸——後者的 CONFIRMED 指「收復事件已確認」，這裡指「zone 被收復確認」。
   */
  zone_health_state?: string
  /** @deprecated 改用 `zone_health_state`；後端仍同時輸出兩者且值相同。 */
  lifecycle?: string
  decision_role?: string
  reason: string
}

export interface SRMarketEvent {
  type: 'EXTREME_VOLUME' | 'HIGH_VOLUME_BREAKDOWN' | 'INTRADAY_RECLAIM' | 'REVERSAL_CANDIDATE' | string
  direction: 'BULLISH' | 'BEARISH' | 'NEUTRAL' | string
  confidence: number
  zone_ref?: {
    price_low: number
    price_high: number
    label: string
    role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
    tier?: ZoneTier
    tier_label?: string
    distance_pct?: number
    entry_relevance_score?: number
  } | null
  price_level: number | null
  reason: string
  detected_at: string
}

export interface SRDefenseLine {
  price_low: number
  price_high: number
  label: string
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  source: string
  invalidation_price: number | null
  recovery_price: number | null
}

export interface SRDefenseLines {
  tactical: SRDefenseLine | null
  swing: SRDefenseLine | null
  strategic: SRDefenseLine | null
}

export interface SRPositionActionCondition {
  state: SRStructureState | string
  structure_state?: SRStructureState | string
  invalidation_price: number | null
  recovery_price: number | null
  reason_codes: string[]
}

export interface SRConfidenceFactor {
  key: string
  value: number | null
  label: string
  description?: string
  effect?: string
}

export interface SRConfidenceExplanation {
  value: number | null
  level: ConfidenceLevel | null
  label: string
  formula_factors: SRConfidenceFactor[]
  context_factors: SRConfidenceFactor[]
}

export interface SRRRGate {
  minimum_rr: number | null
  actual_rr: number | null
  qualified: boolean
  reason_code: string
  gate_basis?: string
  zone_actual_rr?: number | null
  target_known?: boolean
}

export interface SRDataQuality {
  data_mode?: string
  overall_completeness: number
  market_data_completeness?: number
  rr_completeness?: number
  trade_qualification_completeness?: number
  price_data_complete: boolean
  chip_coverage: number
  missing_features: string[]
  unavailable_features?: string[]
  neutral_features?: string[]
  negative_features?: string[]
  positive_features?: string[]
  invalid_features?: string[]
  features?: Record<string, {
    status: string
    confidence: number
    source: string
    interpretation: string
    value: number | null
    updated_at?: string | null
    reason_codes?: string[]
  }>
  stale_features: string[]
  notes: string[]
}

export interface SREventSequenceItem {
  type: string
  label: string
  direction?: string | null
  confidence?: number | null
  price_level?: number | null
}

export interface SRDailyPriceAction {
  available: boolean
  close_location: number | null
  body_proxy_ratio?: number | null
  body_ratio?: number | null
  body_ratio_source?: string | null
  lower_wick_ratio?: number | null
  upper_wick_ratio?: number | null
  close_location_state: string
  range_pct: number | null
  range_state: string
  gap_state: string
  follow_through_state: string
  price_follow_through_state?: string
  momentum_confirmation_state?: string
  reclaim_rejection_state: string
  signals: string[]
  reference_prices?: Record<string, number | null>
}

export interface SRDailyCandidateZone {
  price_low: number
  price_high: number
  label: string
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  source: string
  lifecycle: string
  decision_role: string
  zone_kind?: 'DAILY_ZONE' | 'BREAKOUT_TRIGGER' | 'BREAKDOWN_TRIGGER' | string
  trigger_price?: number | null
  distance_pct: number
  distance_label: string
  reason: string
  event_refs: string[]
}

export interface SRPricePath {
  path_state: string
  event_state?: string
  active_event_types?: string[]
  blocked_by_event?: Record<string, unknown> | null
  reason_codes?: string[]
  invalidation_price: number | null
  recovery_price: number | null
  next_decision_price: number | null
  next_decision_source: string | null
  blocking_zone: {
    price_low: number
    price_high: number
    label: string
    role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
    source: string
    source_scope?: string | null
    zone_id?: number | string | null
    method?: string | null
    timeframe?: string | null
    tier?: string | null
    tier_label?: string | null
    confidence?: number | null
    confidence_level?: string | null
    distance_pct?: number | null
    selected_summary_zone?: boolean
  } | null
  transitions: Array<{
    if: string
    then: string
    price: number | null
  }>
}

export interface SRDailyConfirmation {
  state: string
  label: string
  reason_codes: string[]
  requires_next_daily_close: boolean
  source: string
}

export interface SRFinalEntryPermission {
  state: 'BLOCKED' | 'WAIT_CONFIRMATION' | 'PROBE_ALLOWED' | 'ENTRY_ALLOWED' | string
  label: string
  entry_action_state: string
  daily_confirmation_state: string
  reason_codes: string[]
}

export interface SRRRContext {
  entry_rr: number | null
  entry_rr_source: string
  execution_rr?: number | null
  execution_rr_source?: string
  position_rr: number | null
  position_rr_source: string
  entry_price?: number | null
  entry_zone_lower?: number | null
  entry_zone_upper?: number | null
  stop_price?: number | null
  target_price?: number | null
  price_basis?: string
  stop_basis?: string
  target_basis?: string
  structural_stop_price?: number | null
  risk_price?: number | null
  reward_price?: number | null
  stop_distance_pct?: number | null
  executable_now?: boolean
  entry_executability_reason_code?: string | null
  rr_formula_available?: boolean
}

export interface SREntryExecutability {
  entry_price: number | null
  entry_zone_lower: number | null
  entry_zone_upper: number | null
  tolerance: number | null
  executable_now: boolean
  reason_code: string | null
  price_basis: string
}

export interface SREntryBlockingZone {
  blocked: boolean
  reason_code?: string | null
  distance_price?: number | null
  threshold_price?: number | null
  distance_pct?: number | null
  threshold_pct?: number | null
  /** @deprecated use distance_pct. */
  distance_to_nearest_resistance?: number | null
  /** @deprecated use threshold_pct. */
  threshold?: number | null
  threshold_basis?: string
  blocking_zone?: {
    price_low: number
    price_high: number
    label: string
    role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
    tier?: string | null
    tier_label?: string | null
    source_scope?: string | null
    method?: string | null
    confidence?: number | null
  } | null
}

export interface SRSemanticPipeline {
  version?: string
  event_signal?: string
  lifecycle_phase?: string
  market_state?: string
  bias_state?: SRMarketBias
  action_state?: string
  entry_permission_state?: string
  reason_codes?: string[]
  source_order?: string[]
}

export interface SRDecisionDerivedView {
  version?: string
  semantic_pipeline?: SRSemanticPipeline
  bias_state?: SRMarketBias
  bias_label?: string
  bias_reason_codes?: string[]
  active_event_types?: string[]
  candidate_event_types?: string[]
  final_entry_reason_codes?: string[]
  path_gate_state?: string
  path_reason_codes?: string[]
  /** @deprecated use semantic_pipeline.action_state */
  position_gate_state?: string
  position_reason_codes?: string[]
  daily_reason_codes?: string[]
  authority_reason_codes?: string[]
}

// decision_derived_view 的 reason code → 中文說明；查無對照時回退顯示原始 code。
// SRZones 與 Positions 共用此表，避免兩處各自維護而漂移。
export const derivedReasonText: Record<string, string> = {
  MARKET_ACTION_AVOID: '盤勢避開（hard block）',
  SHORT_TERM_RECOVERY: '短線收復確認',
  SHORT_TERM_RECLAIM_ATTEMPT: '短線收復嘗試',
  SHORT_TERM_EARLY_TREND: '早期趨勢',
  REVERSAL_CANDIDATE: '反轉候選（待確認）',
  MARKET_ACTION_BUY: '盤勢偏多',
  MARKET_ACTION_BUY_SMALL: '盤勢偏多（小量）',
  STRUCTURAL_BEARISH_CONTEXT: '結構偏空',
  STRUCTURAL_BULLISH_CONTEXT: '結構偏多',
  NEUTRAL_CONTEXT: '中性',
  WAIT_PRICE_FOLLOW_THROUGH: '等待價格延續',
  REVERSAL_AWAIT_NEXT_DAILY_CONFIRM: '反轉待隔日確認',
  NO_MOMENTUM_CONFIRMATION: '動能未確認',
  MOMENTUM_UNCONFIRMED: '動能未確認',
  ENTRY_GATE_BLOCKED: '進場閘門阻擋',
  ENTRY_GATE_WAIT_CONFIRMATION: '進場閘門等待確認',
  ACTIVE_BEARISH_EVENT: '有效偏空事件',
  SUPPORT_BREAKDOWN_RISK: '支撐跌破風險',
  BLOCKING_ZONE_AHEAD: '前方壓力擋道',
  DAILY_CANDIDATE_ONLY: '僅日 K 候選',
  RR_NOT_QUALIFIED: 'RR 未達門檻',
  POSITION_DEFENSE_REQUIRED: '持倉需防守',
  POSITION_RECLAIM_DEFENSE: '收復後條件式防守',
  POSITION_SUPPORT_DEFENSE: '支撐防守',
  POSITION_RESISTANCE_OVERHEAD: '上方壓力需突破',
}

export const derivedReasonLabel = (code: string): string => derivedReasonText[code] ?? code

export interface SRDecisionSummary {
  data_mode?: string
  data_quality?: SRDataQuality
  decision_derived_view?: SRDecisionDerivedView
  market_regime?: SRMarketRegime
  model_governance?: {
    health_state?: string
    quality_flags?: string[]
    warning_flags?: string[]
    blocking_flags?: string[]
    confidence_gate?: {
      state?: string
      allow_entry?: boolean
      max_entry_state?: string
      reason_codes?: string[]
    }
    reports?: Record<string, unknown>
  }
  market_events?: SRMarketEvent[]
  event_state_summary?: {
    version?: string
    states?: Array<Record<string, unknown>>
    active?: Array<Record<string, unknown>>
    resolved?: Array<Record<string, unknown>>
    active_bearish_events?: Array<Record<string, unknown>>
    active_bullish_events?: Array<Record<string, unknown>>
    latest_event_type?: string | null
    market_state?: string
  }
  event_sequence?: SREventSequenceItem[]
  daily_price_action?: SRDailyPriceAction
  daily_candidate_zones?: SRDailyCandidateZone[]
  price_path?: SRPricePath
  daily_confirmation?: SRDailyConfirmation
  entry_executability?: SREntryExecutability
  entry_blocking_zone?: SREntryBlockingZone
  defense_lines?: SRDefenseLines
  rr_context?: SRRRContext
  market_bias?: SRMarketBias
  market_bias_label?: string
  decision_contract?: {
    version?: string
    authoritative_fields?: string[]
    deprecated_fields?: string[]
  }
  market_action?: SRMarketAction
  position_action?: SRPositionAction
  position_action_condition?: SRPositionActionCondition
  action?: SRDecisionAction
  action_label?: string
  entry_action_state?: SREntryActionState
  entry_action_label?: string
  final_entry_permission?: SRFinalEntryPermission
  daily_entry_state?: string
  daily_entry_label?: string
  rr_gate?: SRRRGate
  nearest_decision_zone?: SRDecisionZoneSummary | null
  nearest_support_zone?: SRDecisionZoneSummary | null
  nearest_resistance_zone?: SRDecisionZoneSummary | null
  primary_structural_zone?: SRDecisionZoneSummary | null
  best_trade_zone?: SRDecisionZoneSummary | null
  primary_zone?: SRDecisionZoneSummary | null
  market_context?: SRDecisionContextItem[]
  confidence_explanation?: SRConfidenceExplanation
  risk_notes?: string[]
  secondary_zones?: SRDecisionZoneSummary[]
}

export interface SRNormalizedStatus {
  decision?: string
  events?: string
  daily_candidates?: string
  model_governance?: string
}

export interface SRZoneAnalysis {
  id: number
  symbol: string
  timeframe: string
  analyzed_at: string
  current_price: number
  // 只有一個 Global Model：這次分析唯一、權威的整體評估區塊，同一次分析裡
  // 所有 zone 共用，不在每個 zone 重複顯示。global_trend/global_volatility
  // 是股票層級的量；global_expected_value/global_risk_reward_ratio 是所有
  // zone 依 confidence 加權平均後「唯一收斂」的結果，只在「完全沒有 zone
  // 解析出明確方向」（全部是 AT_ZONE 或都沒有 expected_value/risk_reward_ratio）
  // 時才是 null。global_confidence 是所有 zone confidence 的簡單平均
  // （不論 zone 有沒有明確方向都會計入），只有 zones 陣列本身是空的時候
  // 才會是 null——這跟 global_expected_value/global_risk_reward_ratio 的
  // null 條件不同，不要混為一談。
  global_trend: number
  global_volatility: number
  global_expected_value: number | null
  global_confidence: number | null
  global_risk_reward_ratio: number | null
  model_version: string
  // 訓練這個模型時的 DatasetConfig/zone builder 參數/model_type/
  // calibration_method 快照的短 hash——比 model_version 更細，同一個
  // model_version 底下換過幾次訓練參數都可能有不同的值，重訓改參數後舊
  // 分析可以靠這個值被辨識出來。
  model_config_hash: string
  pipeline_version: string
  evidence: SRGlobalEvidence | null
  explanation?: SRAnalysisExplanation | null
  scenario?: SRScenario | null
  probability_context?: SRProbabilityContext | null
  // Python 端已收斂好的短/中/長期支撐壓力摘要；完整明細仍由 zones 提供。
  period_summaries: SRPeriodSummary[]
  // 跑馬燈輪播提示，用白話補充籌碼、均線、量能與驗證狀態。
  analysis_tips: string[]
  // 整檔層級籌碼拆解，供共用籌碼面板顯示（每張支撐/壓力卡的角色化籌碼一行在
  // period_summaries[].support/resistance.chip）。舊分析沒有這欄時為 null。
  chip_summary?: SRChipSummary | null
  decision_summary?: SRDecisionSummary | null
  // 這次分析實際採用的 zone builder 設定。057 migration 之前的舊分析為 null，
  // 代表「沒有這項紀錄」——不等於 adaptive 未啟用（那是 enabled:false）。
  zone_builder_runtime_config?: SRZoneBuilderRuntimeConfig | null
  normalized_status?: SRNormalizedStatus
  created_at: string
}

interface SRZonePipelineItem {
  data: Pick<SRZone, 'id' | 'analysis_id' | 'price_low' | 'price_high' | 'method' | 'role'>
  features: SRZone['features']
  score: SRZone
  evidence: SRZoneEvidence | null
  explanation: SRZoneExplanation | null
  scenario: SRScenario | null
  probability_context: SRProbabilityContext | null
  lifecycle: Pick<SRZone, 'status' | 'broken_at' | 'broken_price' | 'resolved_role'>
}

interface SRZonePipelineResponse {
  pipeline_version: string
  analysis: Pick<SRZoneAnalysis,
    'id' | 'symbol' | 'timeframe' | 'analyzed_at' | 'current_price' |
    'model_version' | 'model_config_hash' | 'period_summaries' |
    'analysis_tips' | 'chip_summary' | 'zone_builder_runtime_config' | 'created_at'>
  features: Pick<SRZoneAnalysis, 'global_trend' | 'global_volatility'>
  score: Pick<SRZoneAnalysis,
    'global_expected_value' | 'global_confidence' | 'global_risk_reward_ratio'>
  evidence: SRGlobalEvidence | null
  decision: SRDecisionSummary | null
  explanation: SRAnalysisExplanation | null
  scenario: SRScenario | null
  probability_context: SRProbabilityContext | null
  normalized_status?: SRNormalizedStatus
  zones: SRZonePipelineItem[]
}

function normalizePipelineResponse(response: SRZonePipelineResponse): {
  analysis: SRZoneAnalysis
  zones: SRZone[]
} {
  return {
    analysis: {
      ...response.analysis,
      ...response.features,
      ...response.score,
      pipeline_version: response.pipeline_version,
      evidence: response.evidence,
      explanation: response.explanation ?? null,
      scenario: response.scenario ?? null,
      probability_context: response.probability_context ?? null,
      decision_summary: response.decision,
      period_summaries: response.analysis.period_summaries ?? [],
      analysis_tips: response.analysis.analysis_tips ?? [],
      // v2 evidence is preferred for new analyses; pre-migration snapshots
      // retain their dedicated chip_summary and have evidence=null.
      chip_summary: response.evidence?.chip ?? response.analysis.chip_summary ?? null,
      normalized_status: response.normalized_status,
    },
    zones: response.zones.map((item) => ({
      ...item.score,
      ...item.data,
      ...item.lifecycle,
      features: item.features,
      evidence: item.evidence,
      explanation: item.explanation ?? null,
      scenario: item.scenario ?? null,
      probability_context: item.probability_context ?? null,
    })),
  }
}

// limit 為抓取的歷史K棒根數（不是天數），省略或傳 0 時由 Python 端套用預設值（250）
export async function createSRZoneAnalysis(
  symbol: string,
  timeframe = '1d',
  limit?: number,
  reuseExisting = false
): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  const response = await apiFetch<SRZonePipelineResponse>('/sr-zones', {
    method: 'POST',
    body: JSON.stringify({ symbol, timeframe, limit: limit || undefined, reuse_existing: reuseExisting }),
  })
  return normalizePipelineResponse(response)
}

export async function listSRZoneAnalyses(symbol?: string, limit = 20): Promise<SRZoneAnalysis[]> {
  const query = symbol ? `?symbol=${symbol}&limit=${limit}` : `?limit=${limit}`
  const res = await apiFetch<{ analyses: SRZoneAnalysis[]; total: number }>(`/sr-zones${query}`)
  return res.analyses ?? []
}

export async function getSRZoneAnalysis(id: number): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  const response = await apiFetch<SRZonePipelineResponse>(`/sr-zones/${id}`)
  return normalizePipelineResponse(response)
}

export async function deleteSRZoneAnalysis(id: number): Promise<void> {
  await apiFetch(`/sr-zones/${id}`, { method: 'DELETE' })
}

// verifySRZoneAnalysis 手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個
// zone 的 status（PENDING/HELD_SO_FAR/BROKEN）。可重複呼叫，每次都用目前
// 為止最新的資料重新計算（見 sr-zone-scoring.md「Zone 生命週期驗證」）。
export async function verifySRZoneAnalysis(id: number): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  const response = await apiFetch<SRZonePipelineResponse>(`/sr-zones/${id}/verify`, { method: 'POST' })
  return normalizePipelineResponse(response)
}

export interface TrainOptions {
  symbols?: string[]
  timeframe?: string
  limit?: number
  modelType?: 'gradient_boosting' | 'hist_gradient_boosting' | 'lightgbm' | 'logistic_regression'
  splitMethod?: 'time' | 'random'
  calibrationMethod?: 'sigmoid' | 'isotonic' | 'none'
}

export type TrainJobStatus = 'pending' | 'running' | 'done' | 'failed'

// SRScoringTrainJob 對應後端 store.SRScoringTrainJob。rows/sources/metrics/
// model_path/model_version 只有 status=done 才有值；error 只有 status=failed
// 才有值（見 sr-zone-scoring.md「訓練任務可觀測化」）。
export interface SRScoringTrainJob {
  id: number
  job_id: string
  status: TrainJobStatus
  symbols: string // JSON array string
  timeframe: string
  fetch_limit: number
  model_type: string
  rows: number | null
  sources: number | null
  // metrics 是 {"hold": {...}, "break": {...}} 形狀（見 model.py::train_model
  // 回傳的 metrics dict：accuracy/precision/recall/auc/brier_score/log_loss/
  // train_rows/test_rows/positive_rate_train/positive_rate_test/calibrated
  // (1=有校準/0=降級為不校準)）。
  metrics: Record<string, Record<string, number>> | null
  model_path: string | null
  model_version: string | null
  // split_method: "time"（依 touch_time 逐股票切分，預設）或 "random"（舊行為）
  split_method: string | null
  // dataset_summary 見 dataset.py::summarize_training_dataset()：rows_by_symbol
  // /role_counts/hold_positive_rate/break_positive_rate/feature_zero_rate/
  // rr_reference_count，用來判斷這次訓練出來的模型可不可信，不影響任何計算。
  dataset_summary: {
    rows: number
    rows_by_symbol: Record<string, number>
    role_counts: Record<string, number>
    hold_positive_rate: number | null
    break_positive_rate: number | null
    feature_zero_rate: Record<string, number>
    rr_reference_count: number
  } | null
  error: string | null
  started_at: string | null
  finished_at: string | null
  created_at: string
}

// triggerSRScoringTrain 手動觸發 bounce/break 機率模型重新訓練。symbols 省略時
// 後端會自動使用整個 watchlist；立即回傳 job_id（status=pending），實際訓練
// 在背景執行（可能耗時數十秒到數分鐘），用 getTrainJob(job_id) 輪詢進度。
export async function triggerSRScoringTrain(
  opts: TrainOptions = {}
): Promise<{ job_id: string; status: TrainJobStatus; message: string; symbols: number }> {
  return apiFetch('/sr-zones/train', {
    method: 'POST',
    body: JSON.stringify({
      symbols: opts.symbols,
      timeframe: opts.timeframe ?? '1d',
      limit: opts.limit ?? 1500,
      model_type: opts.modelType ?? 'gradient_boosting',
      split_method: opts.splitMethod ?? 'time',
      calibration_method: opts.calibrationMethod ?? 'sigmoid',
    }),
  })
}

export async function getTrainJob(jobId: string): Promise<SRScoringTrainJob> {
  const res = await apiFetch<{ job: SRScoringTrainJob }>(`/sr-zones/train-jobs/${jobId}`)
  return res.job
}

export async function listTrainJobs(limit = 20): Promise<SRScoringTrainJob[]> {
  const res = await apiFetch<{ jobs: SRScoringTrainJob[]; total: number }>(`/sr-zones/train-jobs?limit=${limit}`)
  return res.jobs ?? []
}

export async function pruneTrainJobs(keep = 20): Promise<{ deleted: number; keep: number }> {
  return apiFetch(`/sr-zones/train-jobs?keep=${keep}`, { method: 'DELETE' })
}

// ModelStatus 對應後端 analysis.ModelStatus。exists=false 時其餘欄位皆為
// null——用這支端點在觸發分析前先確認模型準備好了沒，不用等分析失敗才知道。
export interface ModelStatus {
  exists: boolean
  version: string | null
  trained_at: string | null
  model_path: string | null
  split_method: string | null
  metrics: Record<string, Record<string, number>> | null
  feature_names: string[] | null
  // config_hash：訓練設定（DatasetConfig/zone builder 參數/model_type/
  // calibration_method）快照的短 hash，跟分析快照存的 model_config_hash
  // 是同一個值，重訓改參數後舊分析可以靠這個值被辨識出來。
  config_hash: string | null
  training_config: Record<string, unknown> | null
}

export async function getModelStatus(): Promise<ModelStatus> {
  return apiFetch('/sr-zones/model-status')
}

export interface SREvaluationOptions {
  symbols: string[]
  timeframe?: string
  limit?: number
  writeDb?: boolean
  decisionReplay?: boolean
  replayMaxRows?: number
  // zone builder 的四個 ATR 參數。evaluation 與 decision replay 兩種模式都會生效
  // （見 sr-zone-scoring.md「Decision Replay 的 zone builder 參數」）。
  // 只有正數會被送出；undefined / <= 0 一律不送該鍵，由 Python 預設值接手
  // （原因見 optionalBuilderParams：Go 的 omitempty 讓 0 根本傳不到 Python）。
  atrWidthMultiplier?: number
  maxMergeWidthMultiple?: number
  atrLookback?: number
  atrPeriod?: number
}

// decision replay 的治理判定（Python `_decision_replay_governance_evaluation`）。
// confidence_gate 的 allow_entry / max_entry_state 才是真正會限制 production 進場的值，
// health_state 只是它們的摘要。
export interface SRDecisionReplayGovernance {
  schema_version?: string
  scope?: string
  health_state?: string
  passed?: boolean
  strict_passed?: boolean
  blocking_flags?: string[]
  warning_flags?: string[]
  confidence_gate?: {
    state?: string
    allow_entry?: boolean
    max_entry_state?: string
    reason_codes?: string[]
  }
  coverage?: Record<string, number | null>
}

// replay 實際涵蓋到的股票範圍。report 的 symbols/sources 描述的是「要求驗證的範圍」，
// 這裡才是「實際驗證到的範圍」，預算不足時兩者會不一致。
export interface SRReplayCoverage {
  symbols_requested?: number
  symbols_covered?: number
  symbols_skipped?: string[]
  coverage_ratio?: number | null
  window_mode?: string
}

// 實測校準（`sr_evaluation_calibration_v1`）。與 train job 的 calibration_method 是不同東西：
// 那支描述「訓練時有沒有做校準」，這裡是拿 holdout 實際量出來的 reliability。
// 空 bin 會保留（rows=0、其餘欄位為 null），schema 才能跨 candidate 對齊比較。
export interface SRCalibrationBin {
  lower?: number | null
  upper?: number | null
  rows?: number
  mean_predicted?: number | null
  observed_rate?: number | null
  gap?: number | null
}

export interface SRCalibration {
  schema_version?: string
  bin_count?: number
  rows?: number
  binned_rows?: number
  bins?: SRCalibrationBin[]
  expected_calibration_error?: number | null
  max_calibration_error?: number | null
  // 樣本 < MIN_CALIBRATION_ROWS(50) 時 bin 內 observed_rate 抖動極大，ECE 不可拿來調參。
  insufficient_sample?: boolean
}

export interface SRBinaryMetrics {
  rows?: number
  positive_rows?: number
  auc?: number | null
  brier_score?: number | null
  log_loss?: number | null
  calibration?: SRCalibration | null
}

// 注意：模型載不到時 Python 回的是 {model_available: false, hold: null, break: null}
// ——hold / break 是 null 而不是缺鍵，取值前必須判空。
export interface SRModelMetrics {
  model_available?: boolean
  model_version?: string
  model_trained_at?: string
  model_config_hash?: string
  hold?: SRBinaryMetrics | null
  break?: SRBinaryMetrics | null
}

// 頂層 zone_outcomes 與三種分層（by_method / by_role / by_volatility_bucket）共用這個形狀，
// 因為 Python 的 `_zone_outcome_group` 與 `_zone_outcomes` 刻意用同名同算法的欄位。
// `hold_rate` 只有分層才有：它是整組不分角色的 zone 守住率，與拆開角色的
// support_hold_rate / resistance_rejection_rate 是不同指標，不要互相取代。
// 在 by_role 分層裡，support_hold_rate 與 resistance_rejection_rate 必有一個是 null。
export interface SRZoneOutcomeGroup {
  rows?: number
  hold_rate?: number | null
  support_hold_rate?: number | null
  resistance_rejection_rate?: number | null
  break_positive_rate?: number | null
  average_forward_return?: number | null
}

export interface SRZoneOutcomes extends SRZoneOutcomeGroup {
  by_method?: Record<string, SRZoneOutcomeGroup>
  by_role?: Record<string, SRZoneOutcomeGroup>
  by_volatility_bucket?: Record<string, SRZoneOutcomeGroup>
}

export interface SRDecisionOutcomeGroup {
  rows?: number
  rows_with_forward_return?: number
  average_forward_return?: number | null
  positive_forward_return_rate?: number | null
  negative_forward_return_rate?: number | null
}

// 保守版 RR 統計：只抽 rr_context 的穩定欄位，bucket / 完整 distribution 尚未納入。
// 數值分布摘要（Python `_metric_distribution`）。
//
// 只看平均會誤導：2026-08-07 的真實 report 裡 entry RR 平均 6.45、中位數 2.34、
// 最大值 1032——平均是中位數的 2.75 倍。UI 要同時顯示中位數與 p10/p90 才看得出尾巴。
// count=0 時其餘欄位是 null 而不是 0（沒有樣本 ≠ 樣本值為 0）。
export interface SRMetricDistribution {
  count?: number
  average?: number | null
  stddev?: number | null
  min?: number | null
  p10?: number | null
  p25?: number | null
  median?: number | null
  p75?: number | null
  p90?: number | null
  max?: number | null
}

export interface SRRRSummary {
  rows_with_entry_rr?: number
  average_entry_rr?: number | null
  median_entry_rr?: number | null
  rows_with_position_rr?: number
  average_position_rr?: number | null
  median_position_rr?: number | null
  entry_rr_source_counts?: Record<string, number>
  position_rr_source_counts?: Record<string, number>
  // 2026-08-07 新增：execution RR（先前完全沒有統計，但它參與 rr_gate 判斷）
  rows_with_execution_rr?: number
  average_execution_rr?: number | null
  median_execution_rr?: number | null
  execution_rr_source_counts?: Record<string, number>
  entry_rr_distribution?: SRMetricDistribution
  execution_rr_distribution?: SRMetricDistribution
  position_rr_distribution?: SRMetricDistribution
}

// daily confirmation 的分層單位（Python `_daily_confirmation_groups`）。
//
// **不要拿 SRDecisionOutcomeGroup 來套**：這兩者除了 `rows` 之外完全沒有共同欄位。
// 2026-08-06 之前 by_state / by_primary_role 就是被錯誤宣告成 SRDecisionOutcomeGroup，
// 因為從沒有任何地方消費它，svelte-check 與 build 都抓不到（見 development-workflow.md §3）。
//
// 這裡只有原始 counts，沒有 hold rate 之類的現成比率——那是刻意的。Python 的
// `_outcome_rate` 帶 primary_role 過濾語意，在前端自行相除必然會悄悄跟 Python 分岔，
// 所以 UI 一律照 counts 顯示，不重算比率。
export interface SRDailyConfirmationGroup {
  rows?: number
  next_zone_result_counts?: Record<string, number>
  two_bar_result_counts?: Record<string, number>
  average_next_close_return?: number | null
  average_two_bar_close_return?: number | null
  positive_two_bar_return_rate?: number | null
  negative_two_bar_return_rate?: number | null
  failure_distribution?: Record<string, number>
}

export interface SRDailyConfirmationSummary {
  rows?: number
  support_next_hold_rate?: number | null
  support_two_bar_confirm_rate?: number | null
  resistance_next_rejection_rate?: number | null
  resistance_next_breakout_rate?: number | null
  resistance_two_bar_breakout_continuation_rate?: number | null
  average_next_close_return?: number | null
  average_two_bar_close_return?: number | null
  positive_two_bar_return_rate?: number | null
  negative_two_bar_return_rate?: number | null
  failure_distribution?: Record<string, number>
  // 十五個分層，UI 依語意分三群顯示：結果面（state / primary_role）、
  // 條件面（volume / event 系列）、RR 面（rr_gate 系列）。
  by_state?: Record<string, SRDailyConfirmationGroup>
  by_primary_role?: Record<string, SRDailyConfirmationGroup>
  by_volume_context?: Record<string, SRDailyConfirmationGroup>
  by_event_sequence?: Record<string, SRDailyConfirmationGroup>
  by_market_event_types?: Record<string, SRDailyConfirmationGroup>
  by_event_market_state?: Record<string, SRDailyConfirmationGroup>
  by_rr_gate?: Record<string, SRDailyConfirmationGroup>
  by_rr_gate_reason_code?: Record<string, SRDailyConfirmationGroup>
  by_rr_bucket?: Record<string, SRDailyConfirmationGroup>
  // 2026-08-07 新增的細分層。量能與停損距離是「數值分桶」（邊界沿用 Python 既有常數
  // 與真實分布，見 evaluation._volume_strength_bucket / _stop_distance_bucket）；
  // primary_market_event 是依固定優先序取的代表事件，**不是時間上最早發生的**——
  // 同一列的事件全來自同一根 K 棒，沒有時間順序可言。它是 market_event_types 的低基數粗化。
  by_volume_strength?: Record<string, SRDailyConfirmationGroup>
  by_stop_distance_bucket?: Record<string, SRDailyConfirmationGroup>
  by_entry_executability?: Record<string, SRDailyConfirmationGroup>
  // risk / reward 齊備性：RR_UNAVAILABLE 是最大的一組，靠這個維度才拆得開
  // 「缺目標價」與「缺停損」兩種完全不同的原因。
  by_rr_formula_state?: Record<string, SRDailyConfirmationGroup>
  by_primary_market_event?: Record<string, SRDailyConfirmationGroup>
  by_market_event_count?: Record<string, SRDailyConfirmationGroup>
}

// 這次分析用了哪組 zone builder 設定（Python `_resolve_runtime_builders`）。
// reason_code 有五種：VOLATILITY_BUCKET_CONFIG（adaptive 生效）、EXPLICIT_BUILDERS
// （呼叫端自帶 builder）、ADAPTIVE_ZONE_BUILDERS_DISABLED（開關關閉）、
// UNKNOWN_VOLATILITY_BUCKET（分不出 bucket）、ADAPTIVE_ZONE_BUILDERS_ERROR（例外，含 error）。
// config 是三個 builder 的參數快照，形狀依 builder 而異，故以 unknown 承接。
export interface SRZoneBuilderRuntimeConfig {
  enabled?: boolean
  bucket?: string
  reason_code?: string
  atr_pct?: number | null
  average_range_pct?: number | null
  error?: string
  config?: Record<string, Record<string, unknown>>
}

// 每檔標的的波動側寫（Python `_volatility_profiles`）。`bucket` 就是 zone_outcomes
// `by_volatility_bucket` 的分組鍵，兩者要一起看：這裡是母體（各檔落在哪個 bucket、
// 有幾次觸價），那裡是該 bucket 的成效。atr_pct / average_range_pct 在資料不足時是 null。
export interface SRVolatilityProfile {
  symbol?: string
  timeframe?: string
  bucket?: string
  atr_pct?: number | null
  average_range_pct?: number | null
  touch_count?: number
  candle_count?: number
  touch_density_per_100_bars?: number | null
  lookback_bars?: number
  thresholds?: {
    low_volatility_max?: number
    high_volatility_min?: number
  }
}

export interface SROutcomeSummary {
  at_zone_rate?: number | null
  rows_with_primary_zone?: number
  primary_zone_role_counts?: Record<string, number>
  rr_summary?: SRRRSummary
  daily_confirmation_summary?: SRDailyConfirmationSummary
  by_final_entry_state?: Record<string, SRDecisionOutcomeGroup>
  by_daily_confirmation_state?: Record<string, SRDecisionOutcomeGroup>
  by_market_bias?: Record<string, SRDecisionOutcomeGroup>
  [key: string]: unknown
}

// 兩種 schema 的欄位幾乎互斥：`sr_zone_evaluation_p0` 有 model_metrics / zone_outcomes，
// `sr_zone_decision_replay_p0` 有 outcome_summary / governance_evaluation / replay_coverage，
// 只有 warnings 等少數欄位共用。所以 UI 一律以「欄位在不在」決定顯示，不用模式旗標判斷。
export interface SREvaluationReport {
  schema_version?: string
  run_id?: string
  pipeline_version?: string
  rows?: number
  sources?: number
  symbols?: string[]
  timeframe?: string
  model_available?: boolean
  decision_replay_available?: boolean
  event_lifecycle_replay_available?: boolean
  zone_score_fields_available?: boolean
  decision_fields_available?: boolean
  outcome_summary?: SROutcomeSummary
  zone_outcomes?: SRZoneOutcomes
  model_metrics?: SRModelMetrics
  governance_evaluation?: SRDecisionReplayGovernance
  replay_coverage?: SRReplayCoverage
  volatility_profiles?: Record<string, SRVolatilityProfile>
  warnings?: string[]
  [key: string]: unknown
}

export interface SRRegressionResult {
  id: number
  run_id: string
  model_config_hash: string
  pipeline_version: string
  dataset_from: string | null
  dataset_to: string | null
  split_method: string
  hold_auc: number | null
  hold_brier_score: number | null
  break_auc: number | null
  break_brier_score: number | null
  passed: boolean | null
  schema_version: string
  rows: number | null
  sources: number | null
  governance_health_state: string
  governance_strict_passed: boolean | null
  metrics_json: SREvaluationReport
  created_at: string
}

export type SREvaluationJobStatus = 'pending' | 'running' | 'done' | 'failed'

export interface SREvaluationJob {
  id: number
  job_id: string
  status: SREvaluationJobStatus
  symbols: string
  timeframe: string
  fetch_limit: number
  mode: 'evaluation' | 'decision_replay' | string
  write_db: boolean
  replay_max_rows: number
  run_id: string | null
  schema_version: string | null
  pipeline_version: string | null
  rows: number | null
  sources: number | null
  report: SREvaluationReport | null
  error: string | null
  started_at: string | null
  finished_at: string | null
  created_at: string
}

export async function runSREvaluation(
  opts: SREvaluationOptions
): Promise<{ job_id: string; status: SREvaluationJobStatus; message: string; symbols: number }> {
  return apiFetch('/sr-zones/evaluate', {
    method: 'POST',
    body: JSON.stringify({
      symbols: opts.symbols,
      timeframe: opts.timeframe ?? '1d',
      limit: opts.limit ?? 1500,
      write_db: opts.writeDb ?? false,
      decision_replay: opts.decisionReplay ?? false,
      replay_max_rows: opts.replayMaxRows ?? 200,
      ...optionalBuilderParams(opts),
    }),
  })
}

// 四個 builder 參數只在使用者填了「正數」時才放進 body，其餘（留白 / NaN / <= 0）
// 整個鍵不送，由後端沿用預設值。
//
// 為什麼連 0 都要擋：Go 的 `SREvaluationRequest` 對這四個欄位用 `omitempty`，
// 0 在轉發給 Python 前就會被丟掉。若前端照送 0，使用者會看到「參數收下了」卻毫無效果——
// 正是 T-037 C 想解掉的那種靜默 wiring 失效。這四個參數本來也沒有 0 的合理語意
// （zone 寬度 0、ATR 期數 0）。要支援 0 就得先拿掉 Go 那邊的 omitempty，不是前端硬送。
function optionalBuilderParams(opts: SREvaluationOptions): Record<string, number> {
  const pairs: [string, number | undefined][] = [
    ['atr_width_multiplier', opts.atrWidthMultiplier],
    ['max_merge_width_multiple', opts.maxMergeWidthMultiple],
    ['atr_lookback', opts.atrLookback],
    ['atr_period', opts.atrPeriod],
  ]
  const body: Record<string, number> = {}
  for (const [key, value] of pairs) {
    if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
      body[key] = value
    }
  }
  return body
}

export async function getSREvaluationJob(jobId: string): Promise<SREvaluationJob> {
  const res = await apiFetch<{ job: SREvaluationJob }>(`/sr-zones/evaluation-jobs/${jobId}`)
  return res.job
}

export async function listSREvaluationJobs(limit = 5): Promise<SREvaluationJob[]> {
  const res = await apiFetch<{ jobs: SREvaluationJob[]; total: number }>(`/sr-zones/evaluation-jobs?limit=${limit}`)
  return res.jobs ?? []
}

export async function listSRRegressionResults(
  schemaVersion?: string,
  limit = 10
): Promise<SRRegressionResult[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (schemaVersion) params.set('schema_version', schemaVersion)
  const res = await apiFetch<{ results: SRRegressionResult[]; total: number }>(
    `/sr-zones/regression-results?${params.toString()}`
  )
  return res.results ?? []
}
