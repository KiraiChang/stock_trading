import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import Backtest from './Backtest.svelte'
import {
  getBacktestJob,
  getBacktestTrades,
  listBacktestJobs,
  submitBacktest,
  type BacktestJob,
  type BacktestResult,
} from '../lib/api/backtest'
import { ApiError } from '../lib/api/client'
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

// ── I-137：選檔規則、400 的原因、失敗任務的原因、覆蓋範圍 ─────────────────

const WATCHLIST = [
  { symbol: '2330', name: '台積電' },
  { symbol: '2317', name: '鴻海' },
] as never

function boxes(): HTMLInputElement[] {
  return ['2330', '2317'].map(
    (s) => screen.getByText(s).closest('label')!.querySelector('input') as HTMLInputElement,
  )
}

function strategySelect(): HTMLSelectElement {
  return screen.getByLabelText('策略') as HTMLSelectElement
}

function submitButton(): HTMLButtonElement {
  return screen.getByRole('button', { name: /送出回測/ }) as HTMLButtonElement
}

function job(over: Partial<BacktestJob>): BacktestJob {
  return { ...pendingJob(), job_id: 'bt_x', strategy: 'breakout_v1', ...over }
}

function result(over: Partial<BacktestResult> = {}): BacktestResult {
  return {
    id: 1, job_id: 'bt_x', strategy: 'breakout_v1', total_return: 0, annual_return: 0, win_rate: 0,
    max_drawdown: 0, sharpe_ratio: 0, total_trades: 0, win_trades: 0, loss_trades: 0, avg_pnl: 0,
    created_at: '2026-11-19T00:00:00Z', ...over,
  }
}

describe('Backtest 頁面的選檔（backtrader 一次只能一檔）', () => {
  beforeEach(() => {
    vi.mocked(fetchWatchlist).mockResolvedValue(WATCHLIST)
    vi.mocked(listBacktestJobs).mockResolvedValue([])
  })

  it('預設策略是 breakout_v1 時，首次載入⛔ 不預選、顯示提示', async () => {
    render(Backtest)
    await screen.findByText('2330')
    expect(strategySelect().value).toBe('breakout_v1')
    expect(boxes().map((b) => b.checked)).toEqual([false, false])
    expect(boxes()[0].type).toBe('radio')
    expect(screen.getByTestId('select-hint')).toHaveTextContent('一次只能回測一檔')
    expect(submitButton()).toHaveTextContent('已選 0 檔')
  })

  it('backtrader 下選新的一檔會取代舊的', async () => {
    render(Backtest)
    await screen.findByText('2330')
    await fireEvent.change(boxes()[0])
    await fireEvent.change(boxes()[1])
    expect(boxes().map((b) => b.checked)).toEqual([false, true])
    expect(submitButton()).toHaveTextContent('已選 1 檔')
  })

  it('modular 多選後切到 breakout_v1 會清空並提示；切回 modular 保留選擇', async () => {
    render(Backtest)
    await screen.findByText('2330')
    await fireEvent.change(strategySelect(), { target: { value: 'breakout_swing_atr_v1' } })
    await fireEvent.click(screen.getByRole('button', { name: '全選' }))
    expect(submitButton()).toHaveTextContent('已選 2 檔')

    await fireEvent.change(strategySelect(), { target: { value: 'breakout_v1' } })
    expect(submitButton()).toHaveTextContent('已選 0 檔')
    expect(screen.getByTestId('select-hint')).toBeInTheDocument()

    await fireEvent.change(boxes()[0])
    await fireEvent.change(strategySelect(), { target: { value: 'breakout_swing_atr_v1' } })
    expect(submitButton()).toHaveTextContent('已選 1 檔')
    expect(boxes()[0].type).toBe('checkbox')
  })
})

describe('Backtest 頁面的錯誤顯示', () => {
  beforeEach(() => {
    vi.mocked(fetchWatchlist).mockResolvedValue(WATCHLIST)
    vi.mocked(listBacktestJobs).mockResolvedValue([])
    vi.mocked(submitBacktest).mockReset()
  })

  async function submitWith(err: unknown) {
    vi.mocked(submitBacktest).mockRejectedValue(err)
    render(Backtest)
    await screen.findByText('2330')
    await fireEvent.change(boxes()[0])
    // jsdom 裡結束日期的初始值是空的（送出前的日期檢查會先擋下），先明確填好兩個日期
    await fireEvent.input(screen.getByLabelText('起始日期'), { target: { value: '2025-01-01' } })
    await fireEvent.input(screen.getByLabelText('結束日期'), { target: { value: '2025-12-31' } })
    await fireEvent.click(submitButton())
  }

  it('400 顯示後端的原因', async () => {
    await submitWith(new ApiError(400, 'invalid_date:end_date'))
    expect(await screen.findByText(/結束日期不合法/)).toBeInTheDocument()
  })

  it('400 的標的數超過', async () => {
    await submitWith(new ApiError(400, 'too_many_symbols:21:20'))
    expect(await screen.findByText('一次最多回測 20 檔，這次選了 21 檔')).toBeInTheDocument()
  })

  it('5xx 維持原本的訊息', async () => {
    await submitWith(new ApiError(500, 'internal server error'))
    expect(await screen.findByText('送出回測失敗，請稍後再試')).toBeInTheDocument()
  })

  async function openJob(j: BacktestJob, r: BacktestResult | null) {
    vi.mocked(listBacktestJobs).mockResolvedValue([j])
    vi.mocked(getBacktestJob).mockResolvedValue({ job: j, result: r })
    vi.mocked(getBacktestTrades).mockResolvedValue([])
    const view = render(Backtest)
    await fireEvent.click(await screen.findByText(j.job_id))
    return view
  }

  it('失敗的任務顯示代碼對照的原因，⛔ 不顯示「尚未完成」', async () => {
    await openJob(job({ status: 'failed', error: 'task_too_large:2330:60001:60000' }), null)
    const box = await screen.findByTestId('job-failed')
    expect(box).toHaveTextContent('回測失敗')
    expect(box).toHaveTextContent('合計至少 60,001 根（含暖身），合計上限 60,000 根。請減少檔數')
    expect(screen.queryByText('尚未完成，稍後會自動刷新')).not.toBeInTheDocument()
  })

  it('單一檔超過每檔上限：提示縮短日期範圍（跟合計超過分開說明）', async () => {
    await openJob(job({ status: 'failed', error: 'symbol_too_large:2330:5001:5000' }), null)
    expect(await screen.findByTestId('job-failed')).toHaveTextContent('2330至少 5,001 根（含暖身），每檔上限 5,000 根。請縮短日期範圍')
  })

  it('資料不足顯示取得與需要的根數', async () => {
    await openJob(job({ status: 'failed', error: 'insufficient_data:2330:19:20' }), null)
    expect(await screen.findByTestId('job-failed')).toHaveTextContent('資料不足：2330 取得 19 根，至少需要 20 根')
  })

  it('internal_error 與認不得的文字顯示通用訊息、⛔ 不顯示原始文字', async () => {
    const view = await openJob(job({ status: 'failed', error: 'SELECT secret_col FROM t' }), null)
    expect(await screen.findByTestId('job-failed')).toHaveTextContent('回測失敗（內部錯誤）')
    expect(view.container.textContent).not.toContain('secret_col')
  })

  it('pending／running 顯示「尚未完成」', async () => {
    await openJob(job({ status: 'running' }), null)
    expect(await screen.findByText('尚未完成，稍後會自動刷新')).toBeInTheDocument()
  })
})

describe('Backtest 頁面的覆蓋範圍', () => {
  beforeEach(() => {
    vi.mocked(fetchWatchlist).mockResolvedValue([])
  })

  it('一檔一行：首尾日期與根數、暖身不足、已略過', async () => {
    const j = job({ status: 'done', strategy: 'breakout_swing_atr_v1' })
    vi.mocked(listBacktestJobs).mockResolvedValue([j])
    vi.mocked(getBacktestJob).mockResolvedValue({
      job: j,
      result: result({
        data_coverage: [
          { symbol: '2330', first_bar: '2024-01-02', last_bar: '2025-12-31', bars: 487, warmup_bars: 250, warmup_required: 250, skipped: false },
          { symbol: '6789', first_bar: '2025-03-03', last_bar: '2025-12-31', bars: 200, warmup_bars: 120, warmup_required: 250, skipped: false },
          { symbol: '1234', first_bar: null, last_bar: null, bars: 0, warmup_bars: 0, warmup_required: 250, skipped: true },
        ],
      }),
    })
    vi.mocked(getBacktestTrades).mockResolvedValue([])
    render(Backtest)
    await fireEvent.click(await screen.findByText('bt_x'))
    const list = await screen.findByTestId('data-coverage')
    const lines = Array.from(list.querySelectorAll('li')).map((li) => li.textContent?.replace(/\s+/g, ' ').trim())
    expect(lines[0]).toBe('2330： 2024-01-02～2025-12-31，487 根')
    expect(lines[1]).toContain('暖身不足（120／250 根）')
    expect(lines[2]).toContain('區間內沒有資料，已略過')
  })

  it('data_coverage 是 null（舊結果）時不顯示', async () => {
    const j = job({ status: 'done' })
    vi.mocked(listBacktestJobs).mockResolvedValue([j])
    vi.mocked(getBacktestJob).mockResolvedValue({ job: j, result: result({ data_coverage: null }) })
    vi.mocked(getBacktestTrades).mockResolvedValue([])
    render(Backtest)
    await fireEvent.click(await screen.findByText('bt_x'))
    await waitFor(() => expect(screen.getByText('總報酬率')).toBeInTheDocument())
    expect(screen.queryByTestId('data-coverage')).not.toBeInTheDocument()
  })
})
