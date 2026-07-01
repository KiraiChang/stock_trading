import { apiFetch } from './client'

export interface IndicatorSnapshot {
  id: number
  symbol: string
  timeframe: string
  ts: string
  ma5: number
  ma10: number
  ma20: number
  ma60: number
  rsi14: number
  macd: number
  macd_signal: number
  macd_hist: number
  bb_upper: number
  bb_middle: number
  bb_lower: number
  atr14: number
  vwap: number
  vol_ma20: number
  vol_ratio: number
}

// 尚未計算過指標的股票（剛加入 watchlist、還沒跑過 pre-market/intraday job）
// 後端會回 404，這裡吃掉錯誤回傳 null，讓呼叫端可以用預設值顯示
export async function fetchIndicators(symbol: string, timeframe = '1d'): Promise<IndicatorSnapshot | null> {
  try {
    return await apiFetch<IndicatorSnapshot>(`/indicators/${symbol}?timeframe=${timeframe}`)
  } catch {
    return null
  }
}
