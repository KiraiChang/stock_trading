import { apiFetch } from './client'
import type { SRZone, SRZoneAnalysis } from './srZones'

export interface Position {
  symbol: string
  shares: number
  avg_cost: number
  realized_pnl: number
  version: number
  last_event_id?: number
  updated_at?: string
}

export type PositionEventType = 'OPENING_BALANCE' | 'BUY' | 'SELL' | 'ADJUSTMENT'

export interface PositionTransaction {
  id: number
  symbol: string
  event_type: PositionEventType
  occurred_at: string
  shares: number | null
  price: number | null
  fee: number
  tax: number
  target_shares: number | null
  target_avg_cost: number | null
  note: string
  created_at: string
}

export type PositionState = 'FLAT' | 'LONG'
export type PositionAction =
  | 'ENTER' | 'ENTER_SMALL' | 'WAIT' | 'AVOID'
  | 'HOLD' | 'ADD' | 'REDUCE' | 'TAKE_PROFIT' | 'EXIT_STOP'

export interface PositionAnalysis {
  id: number
  symbol: string
  position_state: PositionState
  position_version: number
  shares: number
  avg_cost: number
  realized_pnl: number
  analyzed_at: string
  current_price: number
  sr_zone_analysis_id: number | null
  action: PositionAction
  action_label: string
  target_shares: number
  adjustment_shares: number
  adjustment_side: 'BUY' | 'SELL' | 'NONE'
  adjustment_amount: number
  entry_price: number | null
  stop_loss_price: number | null
  take_profit_price: number | null
  risk_amount: number | null
  expected_reward_amount: number | null
  risk_reward_ratio: number | null
  unrealized_pnl: number
  unrealized_pnl_pct: number
  config: Record<string, number>
  reason: string[]
  evidence: Record<string, unknown>
  trigger_conditions: string[]
  invalidation_conditions: string[]
  rule_version: string
  created_at: string
}

export async function listPositions(): Promise<Position[]> {
  const response = await apiFetch<{ positions: Position[] }>('/positions')
  return response.positions ?? []
}

export async function getPosition(symbol: string): Promise<Position> {
  const response = await apiFetch<{ position: Position }>(`/positions/${encodeURIComponent(symbol)}`)
  return response.position
}

export async function listPositionTransactions(symbol: string): Promise<PositionTransaction[]> {
  const response = await apiFetch<{ transactions: PositionTransaction[] }>(`/positions/${encodeURIComponent(symbol)}/transactions`)
  return response.transactions ?? []
}

export async function addPositionTransaction(symbol: string, input: {
  event_type: 'BUY' | 'SELL'
  shares: number
  price: number
  fee?: number
  tax?: number
  occurred_at?: string
  expected_version: number
  note?: string
}): Promise<Position> {
  const response = await apiFetch<{ position: Position }>(`/positions/${encodeURIComponent(symbol)}/transactions`, {
    method: 'POST', body: JSON.stringify(input),
  })
  return response.position
}

export async function adjustPosition(symbol: string, input: {
  target_shares: number
  target_avg_cost: number
  expected_version: number
  reason: string
  occurred_at?: string
}): Promise<Position> {
  const response = await apiFetch<{ position: Position }>(`/positions/${encodeURIComponent(symbol)}/adjustments`, {
    method: 'POST', body: JSON.stringify(input),
  })
  return response.position
}

export async function analyzePosition(symbol: string, forceRefresh = false): Promise<{
  analysis: PositionAnalysis
  sr_zone_analysis: SRZoneAnalysis
  zones: SRZone[]
}> {
  return apiFetch('/position-analyses', {
    method: 'POST',
    body: JSON.stringify({ symbol, timeframe: '1d', limit: 250, force_refresh: forceRefresh }),
  })
}

export async function listPositionAnalyses(symbol?: string): Promise<PositionAnalysis[]> {
  const query = symbol ? `?symbol=${encodeURIComponent(symbol)}&limit=20` : '?limit=20'
  const response = await apiFetch<{ analyses: PositionAnalysis[] }>(`/position-analyses${query}`)
  return response.analyses ?? []
}
