import { apiFetch } from './client'

export interface Position {
  portfolio_id: number
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
  portfolio_id: number
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

export interface PositionActionCondition {
  state?: string
  invalidation_price?: number | null
  recovery_price?: number | null
  reason_codes?: string[]
}

export interface PositionAnalysisEvidence {
  sr_decision_action?: string
  take_profit_source?: string
  decision_context?: {
    mode?: 'FLAT_ENTRY' | 'LONG_POSITION' | string
    has_position?: boolean
    position_state?: PositionState | string
    shares?: number | null
    avg_cost?: number | null
    current_price?: number | null
    sr_zone_analysis_id?: number | null
  }
  entry_decision?: {
    applicable?: boolean
    state?: PositionAction | 'NOT_APPLICABLE' | string
    label?: string
    target_shares?: number | null
    entry_price?: number | null
    stop_loss_price?: number | null
    take_profit_price?: number | null
    market_rr?: number | null
    reason_codes?: string[]
  }
  position_decision?: {
    applicable?: boolean
    state?: PositionAction | 'CONDITIONAL_HOLD' | 'NOT_APPLICABLE' | string
    label?: string
    target_shares?: number | null
    adjustment_shares?: number | null
    defense_price?: number | null
    structural_stop?: number | null
    position_rr?: number | null
    position_rr_source?: 'POSITION_AVG_COST' | 'UNAVAILABLE' | string
    reason_codes?: string[]
  }
  position_action_condition?: PositionActionCondition
  risk_sizing?: {
    risk_budget?: number | null
    per_share_risk?: number | null
    max_shares?: number | null
    excess_shares?: number | null
  }
  stops?: {
    defense_price?: number | null
    structural_stop?: number | null
  }
  rr?: {
    market_rr?: number | null
    position_rr?: number | null
  }
  pnl_impact?: {
    realized_delta?: number | null
    unrealized_before?: number | null
    unrealized_after?: number | null
  }
  [key: string]: unknown
}

export interface PositionAnalysis {
  id: number
  portfolio_id: number
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
  evidence: PositionAnalysisEvidence
  trigger_conditions: string[]
  invalidation_conditions: string[]
  rule_version: string
  created_at: string
}

function portfolioQuery(portfolioID: number): string {
  if (!Number.isInteger(portfolioID) || portfolioID <= 0) {
    throw new Error('portfolio_id is required')
  }
  return `portfolio_id=${encodeURIComponent(String(portfolioID))}`
}

export async function listPositions(portfolioID: number): Promise<Position[]> {
  const response = await apiFetch<{ positions: Position[] }>(`/positions?${portfolioQuery(portfolioID)}`)
  return response.positions ?? []
}

export async function getPosition(symbol: string, portfolioID: number): Promise<Position> {
  const response = await apiFetch<{ position: Position }>(
    `/positions/${encodeURIComponent(symbol)}?${portfolioQuery(portfolioID)}`
  )
  return response.position
}

export async function listPositionTransactions(symbol: string, portfolioID: number): Promise<PositionTransaction[]> {
  const response = await apiFetch<{ transactions: PositionTransaction[] }>(
    `/positions/${encodeURIComponent(symbol)}/transactions?${portfolioQuery(portfolioID)}`
  )
  return response.transactions ?? []
}

export async function addPositionTransaction(symbol: string, input: {
  portfolio_id: number
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
  portfolio_id: number
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
