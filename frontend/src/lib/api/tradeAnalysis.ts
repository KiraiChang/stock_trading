import { apiFetch } from './client'
import type { PositionAnalysis } from './positions'
import type { SRZone, SRZoneAnalysis } from './srZones'

export interface TradeAnalysisContext {
  symbol: string
  position_state: 'FLAT' | 'LONG'
  has_position: boolean
}

// T-089 D2：每一筆計畫的衍生欄位（後端算、⛔ 不存 DB）。
export interface TradePlan extends PositionAnalysis {
  // 這次分析實際用到的最後一根日 K 的台北日期（YYYY-MM-DD）；來源無法確認時是 null
  market_date: string | null
  // 確認是現行分析器用日 K 產生的計畫；false 的一律當歷史（舊紀錄或非日 K）
  source_confirmed_1d: boolean
}

// T-089 D2：這一個投組、這一檔的資料狀態（回應頂層）。規格見 docs/api-reference.md 的交易分析兩支端點。
export interface TradeDataStatus {
  latest_candle: 'ok' | 'no_data' | 'error'
  latest_candle_date: string | null
  // 當下應有的行情日：交易日 13:30 之後是今天，其他是 settled_date
  expected_date: string | null
  // 今天之前最近一個交易日
  settled_date: string | null
  // 今天是交易日、已過 13:30、最新日 K 早於今天
  awaiting_today: boolean
  calendar: 'confirmed' | 'unconfirmed'
  // 目前持倉的版本（沒有持倉列是 0；讀取失敗是 null）
  position_version: number | null
  source_lookup: 'ok' | 'error'
  as_of: string
}

export interface TradeAnalysisResult {
  context: TradeAnalysisContext
  analysis: TradePlan
  sr_zone_analysis: SRZoneAnalysis
  zones: SRZone[]
  data_status: TradeDataStatus
}

export interface TradePlanHistory {
  analyses: TradePlan[]
  data_status: TradeDataStatus
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

export async function listTradeAnalyses(symbol: string, portfolioID: number): Promise<TradePlanHistory> {
  if (!Number.isInteger(portfolioID) || portfolioID <= 0) {
    throw new Error('portfolio_id is required')
  }
  const response = await apiFetch<{ analyses?: TradePlan[]; data_status: TradeDataStatus }>(
    `/trade-analysis/${encodeURIComponent(symbol)}/history?limit=20&portfolio_id=${encodeURIComponent(String(portfolioID))}`
  )
  return { analyses: response.analyses ?? [], data_status: response.data_status }
}
