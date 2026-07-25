import { apiFetch } from './client'
import type { PositionAnalysis } from './positions'
import type { SRZone, SRZoneAnalysis } from './srZones'

export interface TradeAnalysisContext {
  symbol: string
  position_state: 'FLAT' | 'LONG'
  has_position: boolean
}

export interface TradeAnalysisResult {
  context: TradeAnalysisContext
  analysis: PositionAnalysis
  sr_zone_analysis: SRZoneAnalysis
  zones: SRZone[]
}

export async function analyzeTrade(symbol: string, portfolioID: number, forceRefresh = false): Promise<TradeAnalysisResult> {
  if (!Number.isInteger(portfolioID) || portfolioID <= 0) {
    throw new Error('portfolio_id is required')
  }
  return apiFetch('/trade-analysis/analyze', {
    method: 'POST',
    body: JSON.stringify({ symbol, portfolio_id: portfolioID, timeframe: '1d', limit: 250, force_refresh: forceRefresh }),
  })
}

export async function listTradeAnalyses(symbol: string, portfolioID: number): Promise<PositionAnalysis[]> {
  if (!Number.isInteger(portfolioID) || portfolioID <= 0) {
    throw new Error('portfolio_id is required')
  }
  const response = await apiFetch<{ analyses: PositionAnalysis[] }>(
    `/trade-analysis/${encodeURIComponent(symbol)}/history?limit=20&portfolio_id=${encodeURIComponent(String(portfolioID))}`
  )
  return response.analyses ?? []
}
