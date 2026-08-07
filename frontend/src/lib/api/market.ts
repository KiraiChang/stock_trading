import { apiFetch } from './client'

export interface BackfillOptions {
  days?: number
  symbols: string[]
}

export type MarketBackfillStatus = 'pending' | 'running' | 'done' | 'partial' | 'failed'

export interface MarketBackfillFailure {
  symbol: string
  error: string
}

// 形狀比照 ChipSyncJob（同一頁上兩塊 UI 走同樣的輪詢流程）。
// 差別只在回補範圍：籌碼用 from_date/to_date，股價用 days。
export interface MarketBackfillJob {
  id: number
  job_id: string
  symbols: string // JSON array string
  days: number
  status: MarketBackfillStatus
  symbols_total: number
  symbols_done: number
  symbols_failed: number
  failures: MarketBackfillFailure[]
  error: string | null
  started_at: string | null
  finished_at: string | null
  created_at: string
}

// triggerBackfill 立即回傳 job（status=pending），實際回補在後端背景執行，
// 用 getBackfillJob(job_id) 輪詢進度。
//
// **symbols 是必填**：API 層不認識 watchlist（與 /chips/sync 一致），
// 空陣列會被後端回 400。「留空 ＝ 整個監控清單」是 UI 的語法糖，
// 由呼叫端在送出前自行填入。
export async function triggerBackfill(opts: BackfillOptions): Promise<MarketBackfillJob> {
  const res = await apiFetch<{ job: MarketBackfillJob }>('/market/backfill', {
    method: 'POST',
    body: JSON.stringify({ days: opts.days ?? 120, symbols: opts.symbols }),
  })
  return res.job
}

export async function getBackfillJob(jobId: string): Promise<MarketBackfillJob> {
  const res = await apiFetch<{ job: MarketBackfillJob }>(`/market/backfill/${jobId}`)
  return res.job
}
