import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import {
  fetchSchedulerStatus,
  triggerDailyCloseRun,
  triggerSREvaluationRun,
  triggerCorporateActionSyncRun,
  triggerStockSymbolSyncRun,
} from './scheduler'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('scheduler api', () => {
  it('includes sr_evaluation status rows', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      jobs: [{ job_name: 'sr_evaluation', status: 'success', symbols_total: 2, symbols_failed: 0, stale: false }],
    })

    await expect(fetchSchedulerStatus()).resolves.toEqual([
      { job_name: 'sr_evaluation', status: 'success', symbols_total: 2, symbols_failed: 0, stale: false },
    ])
    expect(apiFetch).toHaveBeenCalledWith('/scheduler/status')
  })

  it('triggers manual scheduler jobs through POST endpoints', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ message: 'ok' })

    await triggerDailyCloseRun()
    await triggerStockSymbolSyncRun()
    await triggerSREvaluationRun()
    await triggerCorporateActionSyncRun()

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/scheduler/daily-close/run', { method: 'POST' })
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/scheduler/stock-symbol-sync/run', { method: 'POST' })
    expect(apiFetch).toHaveBeenNthCalledWith(3, '/scheduler/sr-evaluation/run', { method: 'POST' })
    expect(apiFetch).toHaveBeenNthCalledWith(4, '/scheduler/corporate-action-sync/run', { method: 'POST' })
  })
})
