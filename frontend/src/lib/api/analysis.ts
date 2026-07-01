import { apiFetch } from './client'

export interface AnalysisLevel {
  id: number
  analysis_id: number
  price: number
  type: 'SUPPORT' | 'RESISTANCE'
  strength: number
  method: string
  status: 'PENDING' | 'HELD_SO_FAR' | 'BROKEN'
  broken_at?: string
  broken_price?: number
}

export interface StockAnalysis {
  id: number
  symbol: string
  timeframe: string
  analyzed_at: string
  current_price: number
  trend: 'BULLISH' | 'BEARISH' | 'SIDEWAYS'
  entry_status: 'ACTIVE' | 'WATCHING'
  entry_direction: 'LONG' | 'SHORT' | 'NONE'
  entry_price: number
  entry_reason?: string
  stop_loss_atr?: number
  stop_loss_structural?: number
  stop_loss_composite?: number
  take_profit_next_level?: number
  take_profit_risk_reward?: number
  take_profit_atr?: number
  trade_verification?: string // JSON string，見 TradeVerification
  verified_at?: string
  created_at: string
}

export interface TouchResult {
  hit: boolean
  hit_at?: string
  hit_price?: number
}

export interface TradeVerification {
  applicable: boolean
  stop_loss?: Record<string, TouchResult>
  take_profit?: Record<string, TouchResult>
}

export function parseTradeVerification(raw?: string): TradeVerification | null {
  if (!raw) return null
  try {
    return JSON.parse(raw) as TradeVerification
  } catch {
    return null
  }
}

export async function createAnalysis(symbol: string, timeframe = '1d'): Promise<{ analysis: StockAnalysis; levels: AnalysisLevel[] }> {
  return apiFetch('/analysis', {
    method: 'POST',
    body: JSON.stringify({ symbol, timeframe }),
  })
}

export async function listAnalyses(symbol?: string, limit = 20): Promise<StockAnalysis[]> {
  const query = symbol ? `?symbol=${symbol}&limit=${limit}` : `?limit=${limit}`
  const res = await apiFetch<{ analyses: StockAnalysis[]; total: number }>(`/analysis${query}`)
  return res.analyses ?? []
}

export async function getAnalysis(id: number): Promise<{ analysis: StockAnalysis; levels: AnalysisLevel[] }> {
  return apiFetch(`/analysis/${id}`)
}

export async function verifyAnalysis(id: number): Promise<{ analysis: StockAnalysis; levels: AnalysisLevel[] }> {
  return apiFetch(`/analysis/${id}/verify`, { method: 'POST' })
}

export async function deleteAnalysis(id: number): Promise<void> {
  await apiFetch(`/analysis/${id}`, { method: 'DELETE' })
}
