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

export async function analyzeTrade(symbol: string, forceRefresh = false): Promise<TradeAnalysisResult> {
  return apiFetch('/trade-analysis/analyze', {
    method: 'POST',
    body: JSON.stringify({ symbol, timeframe: '1d', limit: 250, force_refresh: forceRefresh }),
  })
}

export async function listTradeAnalyses(symbol: string): Promise<PositionAnalysis[]> {
  const response = await apiFetch<{ analyses: PositionAnalysis[] }>(
    `/trade-analysis/${encodeURIComponent(symbol)}/history?limit=20`
  )
  return response.analyses ?? []
}
