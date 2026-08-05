import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Backtest from './Backtest.svelte'
import { listBacktestJobs, type BacktestJob } from '../lib/api/backtest'
import { fetchWatchlist } from '../lib/api/watchlist'

vi.mock('../lib/api/watchlist', () => ({
  fetchWatchlist: vi.fn(),
}))

vi.mock('../lib/api/backtest', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api/backtest')>()
  return {
    ...actual,
    submitBacktest: vi.fn(),
    listBacktestJobs: vi.fn(),
    getBacktestJob: vi.fn(),
    getBacktestTrades: vi.fn(),
    cancelBacktestJob: vi.fn(),
  }
})

function pendingJob(): BacktestJob {
  return {
    id: 1,
    job_id: 'bt_job_001',
    type: 'backtest',
    strategy: 'ma_cross',
    symbols: '["2330"]',
    timeframe: '1d',
    start_date: '2026-01-01',
    end_date: '2026-06-30',
    status: 'pending',
    trigger: 'manual',
    created_at: '2026-08-01T00:00:00Z',
  }
}

beforeEach(() => {
  vi.mocked(fetchWatchlist).mockResolvedValue([])
  vi.mocked(listBacktestJobs).mockReset()
})

// 破壞性動作按鈕先前用 text-fall（依色票是綠色），把「取消」這個危險操作標成安全色。
describe('Backtest 頁面的取消按鈕顏色', () => {
  it('取消 job 用紅色，不能用行情色 text-fall', async () => {
    vi.mocked(listBacktestJobs).mockResolvedValue([pendingJob()])
    render(Backtest)

    const cancel = await screen.findByRole('button', { name: '取消' })

    expect(cancel).toHaveClass('text-red-400')
    expect(cancel).not.toHaveClass('text-fall')
  })
})
