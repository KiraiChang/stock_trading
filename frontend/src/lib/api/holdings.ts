import { apiFetch } from './client'
import type { SRZone, SRZoneAnalysis } from './srZones'

export interface Holding {
  id: number
  symbol: string
  shares: number
  cost_price: number
  note: string
  created_at: string
  updated_at: string
}

export type HoldingAction = 'HOLD' | 'STOP_LOSS' | 'TAKE_PROFIT' | 'ADD_ON_BREAKOUT' | 'REDUCE'

export interface HoldingAnalysis {
  id: number
  holding_id: number
  symbol: string
  shares: number
  cost_price: number
  analyzed_at: string
  current_price: number
  sr_zone_analysis_id: number | null
  action: HoldingAction
  action_label: string
  stop_loss_price: number | null
  stop_loss_amount: number | null
  take_profit_price: number | null
  take_profit_amount: number | null
  add_on_trigger_price: number | null
  add_on_amount: number | null
  unrealized_pnl: number
  unrealized_pnl_pct: number
  reason: string[]
  detail_json: Record<string, unknown>
  created_at: string
}

export interface HoldingInput {
  symbol: string
  shares: number
  cost_price: number
  note?: string
}

export async function listHoldings(): Promise<Holding[]> {
  const res = await apiFetch<{ holdings: Holding[]; total: number }>('/holdings')
  return res.holdings ?? []
}

export async function createHolding(input: HoldingInput): Promise<Holding> {
  const res = await apiFetch<{ holding: Holding }>('/holdings', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return res.holding
}

export async function updateHolding(id: number, input: HoldingInput): Promise<Holding> {
  const res = await apiFetch<{ holding: Holding }>(`/holdings/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
  return res.holding
}

export async function deleteHolding(id: number): Promise<void> {
  await apiFetch(`/holdings/${id}`, { method: 'DELETE' })
}

export async function analyzeHolding(
  id: number,
  opts: { timeframe?: string; limit?: number } = {}
): Promise<{ analysis: HoldingAnalysis; sr_zone_analysis: SRZoneAnalysis; zones: SRZone[] }> {
  return apiFetch(`/holdings/${id}/analyze`, {
    method: 'POST',
    body: JSON.stringify({ timeframe: opts.timeframe ?? '1d', limit: opts.limit ?? 250 }),
  })
}

export async function listHoldingAnalyses(id: number, limit = 20): Promise<HoldingAnalysis[]> {
  const res = await apiFetch<{ analyses: HoldingAnalysis[]; total: number }>(`/holdings/${id}/analyses?limit=${limit}`)
  return res.analyses ?? []
}

export async function getHoldingAnalysis(id: number): Promise<HoldingAnalysis> {
  const res = await apiFetch<{ analysis: HoldingAnalysis }>(`/holding-analyses/${id}`)
  return res.analysis
}
