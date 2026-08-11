import { apiFetch } from './client'

export type JobName =
  | 'pre_market'
  | 'intraday'
  | 'daily_close'
  | 'chip_daily_sync'
  | 'stock_symbol_sync'
  | 'sr_evaluation'
  | 'corporate_action_sync'

export interface SchedulerJob {
  job_name: JobName
  status: string
  symbols_total: number
  symbols_failed: number
  error?: string
  started_at?: string
  finished_at?: string
  stale: boolean
}

export async function fetchSchedulerStatus(): Promise<SchedulerJob[]> {
  const res = await apiFetch<{ jobs: SchedulerJob[] }>('/scheduler/status')
  return res.jobs ?? []
}

// triggerDailyCloseRun 手動重新觸發「收盤後拉日K + 完整掃描」，用於排程
// 時間點 FinMind 當天日K還沒發布（拉到 0 筆）時的補救，在背景執行、立即回應。
export async function triggerDailyCloseRun(): Promise<{ message: string }> {
  return apiFetch('/scheduler/daily-close/run', { method: 'POST' })
}

export async function triggerStockSymbolSyncRun(): Promise<{ message: string }> {
  return apiFetch('/scheduler/stock-symbol-sync/run', { method: 'POST' })
}

export async function triggerSREvaluationRun(): Promise<{ message: string }> {
  return apiFetch('/scheduler/sr-evaluation/run', { method: 'POST' })
}

// triggerCorporateActionSyncRun 手動重跑公司行動同步（分割 ＋ 除權息）與還原係數重算。
//
// 排程是平日 06:30；部署若發生在那之後，沒有這個入口就得等到隔天才驗得了還原是否正確
// （見 scripts/verify-adjustment.sh）。重算是冪等的，重複觸發不會累積誤差。
export async function triggerCorporateActionSyncRun(): Promise<{ message: string }> {
  return apiFetch('/scheduler/corporate-action-sync/run', { method: 'POST' })
}
