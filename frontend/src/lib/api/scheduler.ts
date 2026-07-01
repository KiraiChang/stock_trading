import { apiFetch } from './client'

export type JobName = 'pre_market' | 'intraday' | 'daily_close'

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
