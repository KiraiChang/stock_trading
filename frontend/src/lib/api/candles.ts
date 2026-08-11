import { apiFetch } from './client'

// 這支端點回傳的是**還原價**（見 docs/todo.md T-042）：跨越分割的歷史若用原始價，
// K 線圖會出現一根從未發生的暴跌（0050 在 2025-06-18 的 1:4 分割就是 −75%）。
//
// 調整在 Go handler 完成，**前端不做任何換算**——前端是第三個語言，
// 讓它自己乘係數等於把同一段邏輯散到三處，而這裡算錯不會有任何東西告訴你。
export interface Candle {
  symbol: string
  timeframe: string
  open: number
  high: number
  low: number
  close: number
  // 還原後的成交量是除法的結果，可能不是整數。
  volume: number
  // amount（成交金額）不調整：錢不隨股數重新定義。
  amount: number
  ts: string
  // 這根 K 棒套用的累積還原係數；1 代表未調整（該區間沒有公司行動）。
  adj_factor: number
  // 成交量用的係數。與 adj_factor 不同：現金股利讓價格下修但股數沒變，
  // 所以量不調整。只有分割與配股會讓兩者不相等。
  vol_factor: number
}

export async function fetchCandles(symbol: string, timeframe = '1d', limit = 120): Promise<Candle[]> {
  const res = await apiFetch<{ candles: Candle[] }>(
    `/candles/${symbol}?timeframe=${timeframe}&limit=${limit}`
  )
  return res.candles ?? []
}
