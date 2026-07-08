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

// 機構級版本（2026-07 重新設計，見後端 sr_scoring/scoring.py 開頭的完整說明）
export interface SRZone {
  id: number
  analysis_id: number
  price_low: number
  price_high: number
  method: 'atr' | 'volume_profile'
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'

  // tier/tier_label：zone 依寬度在同一次分析裡的相對排名分三層，讓 zone
  // 清單「可排序」（主結構 → 交易區 → 短期支撐）。
  tier: ZoneTier
  tier_label: string

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

  // 跨方法（ATR/volume_profile）重疊分群：overlap_group 相同的 zone 代表
  // 不同方法都指向同一個價位帶（「多方法共振」），不會合併或刪除任何
  // zone。confluence_count 恆 >= 1；overlap_group 只有 confluence_count > 1
  // 時才有值。
  overlap_group: number | null
  confluence_count: number

  status: 'PENDING' | 'HELD_SO_FAR' | 'BROKEN'
  broken_at?: string | null
  broken_price?: number | null

  // 只有 role='AT_ZONE' 的 zone 在後續驗證（POST /sr-zones/:id/verify）時，
  // 價格真正收盤離開區間後才會被解析並寫入 SUPPORT 或 RESISTANCE；
  // role != 'AT_ZONE' 的 zone 永遠是 null（角色從分析當下就已明確）。role
  // 本身分析後不會再變動，判斷「這個 zone 現在算支撐還是壓力」應優先看
  // resolved_role，沒有值再退回 role。
  resolved_role?: 'SUPPORT' | 'RESISTANCE' | null
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
  confidence: number
  confidence_level: ConfidenceLevel
  trading_score: number
  recent_validation: RecentValidation
  volume_confirmation: VolumeConfirmation | null
  confluence_count: number
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
  signal: 'BULLISH' | 'BEARISH' | 'NEUTRAL' | 'RISK' | null
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
export type SRMarketRegimePrimary = 'TREND_UP' | 'TREND_DOWN' | 'RANGE_BOUND'
export type SRMarketRegimeFlag = 'LOW_CONFIDENCE' | 'HIGH_VOLATILITY'

export interface SRMarketRegime {
  primary: SRMarketRegimePrimary
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

export interface SRDecisionZoneSummary {
  price_low: number
  price_high: number
  label: string
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  tier: ZoneTier
  tier_label: string
  trading_score: number
  confidence: number
  confidence_level: ConfidenceLevel
  expected_value: number | null
  risk_reward_ratio: number | null
  distance_pct: number
  distance_label: string
  recent_validation: RecentValidation
  volume_confirmation: VolumeConfirmation | null
  confluence_count: number
  reason: string
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

export interface SRDecisionSummary {
  market_regime: SRMarketRegime
  action: SRDecisionAction
  action_label: string
  primary_zone: SRDecisionZoneSummary | null
  market_context: SRDecisionContextItem[]
  confidence_explanation: SRConfidenceExplanation
  risk_notes: string[]
  secondary_zones: SRDecisionZoneSummary[]
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
  // Python 端已收斂好的短/中/長期支撐壓力摘要；完整明細仍由 zones 提供。
  period_summaries: SRPeriodSummary[]
  // 跑馬燈輪播提示，用白話補充籌碼、均線、量能與驗證狀態。
  analysis_tips: string[]
  // 整檔層級籌碼拆解，供共用籌碼面板顯示（每張支撐/壓力卡的角色化籌碼一行在
  // period_summaries[].support/resistance.chip）。舊分析沒有這欄時為 null。
  chip_summary?: SRChipSummary | null
  decision_summary?: SRDecisionSummary | null
  created_at: string
}

// limit 為抓取的歷史K棒根數（不是天數），省略或傳 0 時由 Python 端套用預設值（250）
export async function createSRZoneAnalysis(
  symbol: string,
  timeframe = '1d',
  limit?: number
): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  return apiFetch('/sr-zones', {
    method: 'POST',
    body: JSON.stringify({ symbol, timeframe, limit: limit || undefined }),
  })
}

export async function listSRZoneAnalyses(symbol?: string, limit = 20): Promise<SRZoneAnalysis[]> {
  const query = symbol ? `?symbol=${symbol}&limit=${limit}` : `?limit=${limit}`
  const res = await apiFetch<{ analyses: SRZoneAnalysis[]; total: number }>(`/sr-zones${query}`)
  return res.analyses ?? []
}

export async function getSRZoneAnalysis(id: number): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  return apiFetch(`/sr-zones/${id}`)
}

export async function deleteSRZoneAnalysis(id: number): Promise<void> {
  await apiFetch(`/sr-zones/${id}`, { method: 'DELETE' })
}

// verifySRZoneAnalysis 手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個
// zone 的 status（PENDING/HELD_SO_FAR/BROKEN）。可重複呼叫，每次都用目前
// 為止最新的資料重新計算（見 sr-zone-scoring.md「Zone 生命週期驗證」）。
export async function verifySRZoneAnalysis(id: number): Promise<{ analysis: SRZoneAnalysis; zones: SRZone[] }> {
  return apiFetch(`/sr-zones/${id}/verify`, { method: 'POST' })
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

