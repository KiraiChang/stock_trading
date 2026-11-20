import { apiFetch } from './client'

// 盤中逐檔行情與評估狀態（T-088＋I-103）。規格見 docs/api-reference.md 的 GET /intraday/status。

export type IntradayDataStatus = 'written' | 'no_data' | 'store_failed' | 'fetch_failed'
export type IntradayEvalStatus =
  | 'no_signal'
  | 'signal_sent'
  | 'signal_suppressed'
  | 'withheld_stale'
  | 'degraded'
  | 'evaluate_failed'
  | 'not_evaluated'

export interface IntradaySymbolRun {
  run_id: number
  symbol: string
  source: 'scheduled' | 'refill'
  data_at: string
  data_status: IntradayDataStatus
  data_cause: string | null
  last_bar_ts: string | null
  fetch_attempts: number
  http_recovered: boolean
  eval_at: string | null
  eval_status: IntradayEvalStatus | null
  eval_cause: string | null
}

export interface IntradayRound {
  run_id: number
  source: 'scheduled' | 'refill'
  scheduled_at: string
  lock_acquired_at: string | null
  exec_finished_at: string | null
  lock_wait_ms: number
  outcome: 'executed' | 'skipped_lock_timeout' | 'failed'
  evidence_failures: number
  running: boolean
}

export interface IntradaySymbolStatus {
  symbol: string
  latest: IntradaySymbolRun | null
  refill: IntradaySymbolRun | null
  last_success_eval_at: string | null
  last_success_run_id: number | null
  last_bar_ts: string | null
  lag_minutes: number | null
  stale: boolean
  // 最近一輪排程比這一檔最近的逐檔列新時那一輪的狀態（畫面以輪次紀錄為準）；沒有時省略。
  round_state?: 'skipped' | 'failed' | 'running' | 'incomplete'
  eval_pending?: 'evaluating' | 'interrupted'
}

export interface IntradayStatusResponse {
  now: string
  market_open: boolean
  stale_after_minutes: number
  latest_round: IntradayRound | null
  symbols: IntradaySymbolStatus[]
}

export async function fetchIntradayStatus(): Promise<IntradayStatusResponse> {
  return apiFetch<IntradayStatusResponse>('/intraday/status')
}

export interface IntradayRefillSymbol {
  symbol: string
  status: IntradayDataStatus
  cause?: string
  inserted: number
  updated: number
  unchanged: number
  last_bar_ts: string | null
  bars_today: number
  gap_minutes: number
  gaps: string[]
  gaps_truncated: boolean
}

export interface IntradayRefillReport {
  run_id: number
  window_from: string
  window_to: string
  deadline_exceeded: boolean
  symbols: IntradayRefillSymbol[]
}

// 手動補抓當日分 K：只寫 K 棒、⛔ 不評估訊號。symbols 省略＝整份自選清單。
export async function runIntradayRefill(symbols?: string[]): Promise<IntradayRefillReport> {
  return apiFetch<IntradayRefillReport>('/scheduler/intraday-refill/run', {
    method: 'POST',
    body: JSON.stringify(symbols && symbols.length > 0 ? { symbols } : {}),
  })
}
