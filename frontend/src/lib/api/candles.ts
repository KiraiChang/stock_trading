import { apiFetch } from './client'

export interface Candle {
  symbol: string
  timeframe: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  amount: number
  ts: string
}

export async function fetchCandles(symbol: string, timeframe = '1d', limit = 120): Promise<Candle[]> {
  const res = await apiFetch<{ candles: Candle[] }>(
    `/candles/${symbol}?timeframe=${timeframe}&limit=${limit}`
  )
  return res.candles ?? []
}
