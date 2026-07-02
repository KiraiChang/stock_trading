import { apiFetch } from './client'

export interface SRZone {
  id: number
  analysis_id: number
  price_low: number
  price_high: number
  method: 'atr' | 'volume_profile'
  role: 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE'
  support_score: number
  resistance_score: number
  confidence: number
  bounce_probability: number | null
  break_probability: number | null
  expected_value: number | null
  risk_reward_ratio: number | null
  touch_count: number
  rejection_count: number
  breakout_count: number
  avg_return_after_touch: number
  relative_volume: number
  volatility: number
  trend_strength: number
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
