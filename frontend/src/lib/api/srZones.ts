import { apiFetch } from './client'

export type NetScoreLabel = 'STRONG_SUPPORT' | 'NEUTRAL' | 'STRONG_RESISTANCE'
export type ConfidenceLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'VERY_HIGH'
export type RecentValidation = 'VALIDATED_RECENTLY' | 'PENDING_VALIDATION' | 'NOT_TESTED_RECENTLY' | 'EXPIRED'
export type VolumeConfirmation = 'CONFIRMED' | 'WEAK' | 'NEUTRAL' | 'FAILED'
export type ZoneDirection = 'UP' | 'DOWN' | 'FLAT'
export type TradingRecommendation = 'STRONG_BUY' | 'BUY' | 'WATCH' | 'NEUTRAL' | 'AVOID' | 'STRONG_SELL'
export type ZoneTier = 'TIER_1_MAIN_STRUCTURE' | 'TIER_2_TRADING_ZONE' | 'TIER_3_SHORT_TERM'

// trading_score 的可拆解分量（加權貢獻值，五個分量加總 = trading_score）：
// Score = EV(40%) + RR(20%) + Trend(15%) + Volume(15%) + Confidence(10%)
export interface TradingScoreBreakdown {
  expected_value: number
  risk_reward: number
  trend: number
  volume: number
  confidence: number
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
  reject_count: number
  break_count: number

  zone_momentum: number
  zone_direction: ZoneDirection

  recent_validation: RecentValidation

  // trading_score = trading_score_breakdown 五個分量加總，可拆解檢視每個
  // 分量各佔多少分（見十三、Score 必須可拆解）。
  trading_score: number
  trading_score_breakdown: TradingScoreBreakdown
  trading_recommendation: TradingRecommendation

  status: 'PENDING' | 'HELD_SO_FAR' | 'BROKEN'
  broken_at?: string | null
  broken_price?: number | null
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
  // zone 依 confidence 加權平均後「唯一收斂」的結果；global_confidence 是
  // 所有 zone confidence 的簡單平均。zones 為空或都沒有明確方向時，
  // global_expected_value/global_confidence/global_risk_reward_ratio 可能是 null。
  global_trend: number
  global_volatility: number
  global_expected_value: number | null
  global_confidence: number | null
  global_risk_reward_ratio: number | null
  model_version: string
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

export interface TrainOptions {
  symbols?: string[]
  timeframe?: string
  limit?: number
  modelType?: 'gradient_boosting' | 'logistic_regression'
}

// triggerSRScoringTrain 手動觸發 bounce/break 機率模型重新訓練。symbols 省略時
// 後端會自動使用整個 watchlist；在背景執行、立即回應（訓練視資料量可能耗時
// 數十秒到數分鐘，不會卡住這個請求）。
export async function triggerSRScoringTrain(opts: TrainOptions = {}): Promise<{ message: string; symbols: number }> {
  return apiFetch('/sr-zones/train', {
    method: 'POST',
    body: JSON.stringify({
      symbols: opts.symbols,
      timeframe: opts.timeframe ?? '1d',
      limit: opts.limit ?? 1500,
      model_type: opts.modelType ?? 'gradient_boosting',
    }),
  })
}
