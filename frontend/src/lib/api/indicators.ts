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

// 手動計算指標：不要求該股票在監控清單裡，只要求 candles 至少 35 根，
// 不足時後端回 422，這裡讓呼叫端自己 catch 顯示錯誤（不吃掉，因為這是
// 使用者主動觸發的動作，需要看到失敗原因）
export async function computeIndicators(symbol: string, timeframe = '1d'): Promise<IndicatorSnapshot> {
  return apiFetch<IndicatorSnapshot>(`/indicators/${symbol}/compute?timeframe=${timeframe}`, {
    method: 'POST',
  })
}
