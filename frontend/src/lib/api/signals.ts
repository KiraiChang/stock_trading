import { apiFetch } from './client'
import type { Signal } from '../stores/signals'

export async function fetchSignals(limit = 50, symbol?: string): Promise<Signal[]> {
  const query = symbol ? `?limit=${limit}&symbol=${symbol}` : `?limit=${limit}`
  const res = await apiFetch<{ signals: Signal[] }>(`/signals${query}`)
  return res.signals ?? []
}

export interface EvaluateResult {
  signal: Signal | null
  message?: string
}

// 手動觸發訊號評估：完全基於 candles 計算，不要求該股票在監控清單裡；
// candles 不足 35 根時後端回 422，讓呼叫端自己 catch 顯示錯誤
export async function evaluateSignal(symbol: string, timeframe = '1d'): Promise<EvaluateResult> {
  return apiFetch(`/signals/${symbol}/evaluate?timeframe=${timeframe}`, { method: 'POST' })
}
