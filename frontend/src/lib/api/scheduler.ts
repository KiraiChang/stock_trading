import { apiFetch } from './client'

export type JobName =
  | 'pre_market'
  | 'intraday'
  | 'daily_close'
  // 沒有自己的 cron（跟著 daily_close 跑），但有獨立的 job_runs 紀錄
  | 'sr_zone_verify'
  | 'chip_daily_sync'
  | 'stock_symbol_sync'
  | 'sr_evaluation'
  | 'corporate_action_sync'
  | 'evaluation_universe_sync'
  // 沒有自己的 cron（跟著 evaluation_universe_sync 那輪跑），但有獨立的 job_runs 紀錄
  | 'candle_gap_detection'
  // 平日兩輪：17:00 那輪拿前一日籌碼，22:00 那輪（_chip）才有當日的
  | 'sr_analysis'
  | 'sr_analysis_chip'
  // 只補官方的終止上市日期並對帳，⚠️ **不驅動 is_listed**——那是 stock_symbol_sync 的權責
  | 'delisting_reconcile'

// ⚠️ **這個 union 必須與後端的 `knownSchedulerJobs` 一致**
// （`backend/internal/api/handler/scheduler.go`）。少了項目不會有 runtime 影響
// （label 表是 Record<string, string>），但型別會騙人，而以它為 key 的
// `Partial<Record<JobName, …>>` 也跟著不完整。
// ℹ️ 落後（或多出後端已移除的舊 job）時由 `scripts/check-job-names.sh` 擋下，
// 它由 `frontend/scripts/test.sh` 在最後呼叫，對 union 與 `jobLabel` 各做一次
// **雙向**集合比較（原記於 issue.md I-110，已收斂；現況見 docs/development-workflow.md
// 「新增排程還要同步前端的 job 清單」）。

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

// triggerDelistingReconcileRun 手動觸發終止上市名單對帳。
//
// cron 預設關閉，dev 驗收與排程漏跑時都只有這個入口。整輪是冪等的
// （事件以 (source, symbol, delisted_date) upsert），重複觸發不會累積副作用。
//
// acceptShrink 是**人工核可一次來源縮水**的 override，只在確認 TWSE 名單真的變少時才帶。
// ⛔ 它只放行「比上次 accepted 少、但大於 0」；來源回 0 筆是硬失敗，與這個參數無關
// （放行 0 會把所有事件標記成消失並讓防護基準永久降成 0）。⚠️ 後端只認字面值 `1`。
//
// **併發時後端同步回 409**（cron 或另一次手動觸發正在跑），呼叫端要顯示出來——
// 不是「先回 202 再靜默跳過」，那會讓使用者以為觸發成功。
export async function triggerDelistingReconcileRun(
  acceptShrink = false
): Promise<{ message: string; accept_shrink: boolean }> {
  const query = acceptShrink ? '?accept_shrink=1' : ''
  return apiFetch(`/scheduler/delisting-reconcile/run${query}`, { method: 'POST' })
}
