import { apiFetch } from './client'

export type ChipSignal = 'BULLISH' | 'BEARISH' | 'NEUTRAL' | 'RISK'

// GET /chips/:symbol/summary?date= 的回應形狀。institutional/margin/broker
// 為選填區塊：後端查無對應原始資料時會省略該欄位，不是回錯誤（呼應設計
// 文件「無資料不應顯示錯誤堆疊」原則）。
export interface ChipSummary {
  symbol: string
  date: string
  signal: ChipSignal
  totalScore: number
  reason: string[]
  institutional?: {
    foreignNetBuy: number
    investmentTrustNetBuy: number
    dealerNetBuy: number
    consecutiveDays: number
  }
  margin?: {
    marginBalance: number
    marginChange: number
    shortBalance: number
    shortChange: number
  }
  broker?: {
    topNetBuy: number
    concentration: number
  }
}

export async function fetchChipSummary(symbol: string, date?: string): Promise<ChipSummary> {
  const query = date ? `?date=${date}` : ''
  return apiFetch(`/chips/${symbol}/summary${query}`)
}

// 對應後端 store.ChipScore。trade_date/created_at/updated_at 皆為 ISO 字串。
export interface ChipScore {
  id: number
  symbol: string
  trade_date: string
  institutional_score: number
  margin_score: number
  broker_score: number
  concentration_score: number
  total_score: number
  signal: ChipSignal
  reason: string[]
  created_at: string
  updated_at: string
}

export async function fetchChipScores(symbol: string, from: string, to: string): Promise<ChipScore[]> {
  const res = await apiFetch<{ symbol: string; scores: ChipScore[] }>(
    `/chips/${symbol}/scores?from=${from}&to=${to}`
  )
  return res.scores ?? []
}

// 對應後端 store.BrokerTrade。buy_volume/sell_volume/net_buy 內部單位為股，
// 前端顯示時需除以 1000 換算成張（見設計文件單位規則）。
export interface BrokerTrade {
  id: number
  symbol: string
  trade_date: string
  broker_name: string
  branch_name: string
  buy_volume: number
  sell_volume: number
  net_buy: number
  created_at: string
}

export interface ChipBrokersResponse {
  symbol: string
  date: string
  topBuy: BrokerTrade[]
  topSell: BrokerTrade[]
}

export async function fetchChipBrokers(symbol: string, date: string, limit = 20): Promise<ChipBrokersResponse> {
  return apiFetch(`/chips/${symbol}/brokers?date=${date}&limit=${limit}`)
}

export type ChipSyncMode = 'manual' | 'backfill'
export type ChipSyncStatus = 'pending' | 'running' | 'done' | 'partial' | 'failed'
export type ChipDataType = 'institutional' | 'margin' | 'broker' | 'scores'

export interface ChipSyncFailure {
  symbol: string
  error: string
}

// 對應後端 store.ChipSyncJob（manual/backfill 同步任務追蹤，daily 模式沿用
// 既有 job_runs 表，不會出現在這裡）。
export interface ChipSyncJob {
  id: number
  job_id: string
  mode: ChipSyncMode
  symbols: string // JSON array string
  data_types: string // JSON array string
  from_date: string
  to_date: string
  force: boolean
  status: ChipSyncStatus
  symbols_total: number
  symbols_done: number
  symbols_failed: number
  failures: ChipSyncFailure[]
  error: string | null
  started_at: string | null
  finished_at: string | null
  created_at: string
}

export interface ChipSyncRequest {
  mode?: ChipSyncMode
  symbols: string[]
  from?: string
  to?: string
  dataTypes?: ChipDataType[]
  force?: boolean
}

// triggerChipSync 立即回傳 job（status=pending），實際同步在後端背景執行，
// 用 getChipSyncJob(job_id) 輪詢進度。
export async function triggerChipSync(req: ChipSyncRequest): Promise<ChipSyncJob> {
  const res = await apiFetch<{ job: ChipSyncJob }>('/chips/sync', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  return res.job
}

export async function getChipSyncJob(jobId: string): Promise<ChipSyncJob> {
  const res = await apiFetch<{ job: ChipSyncJob }>(`/chips/sync/${jobId}`)
  return res.job
}
