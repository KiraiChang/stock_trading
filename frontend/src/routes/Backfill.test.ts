import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import Backfill from './Backfill.svelte'
import { triggerBackfill, getBackfillJob, type MarketBackfillJob } from '../lib/api/market'
import { fetchWatchlist } from '../lib/api/watchlist'
import { triggerChipSync } from '../lib/api/chips'

// 元件層測試：只驗股價回補這塊的狀態機（送出的 symbols、輪詢顯示、
// 勾選清單確實已移除），API 形狀由後端 handler 測試把關。
vi.mock('../lib/api/market', () => ({
  triggerBackfill: vi.fn(),
  getBackfillJob: vi.fn(),
}))
vi.mock('../lib/api/watchlist', () => ({ fetchWatchlist: vi.fn() }))
vi.mock('../lib/api/chips', () => ({ triggerChipSync: vi.fn(), getChipSyncJob: vi.fn() }))
vi.mock('../lib/api/indicators', () => ({ computeIndicators: vi.fn() }))
vi.mock('../lib/api/signals', () => ({ evaluateSignal: vi.fn() }))

function job(overrides: Partial<MarketBackfillJob> = {}): MarketBackfillJob {
  return {
    id: 1,
    job_id: 'bf_20260807_000000_000',
    symbols: '["2330"]',
    days: 120,
    status: 'pending',
    symbols_total: 1,
    symbols_done: 0,
    symbols_failed: 0,
    failures: [],
    error: null,
    started_at: null,
    finished_at: null,
    created_at: '2026-08-07T00:00:00Z',
    ...overrides,
  }
}

beforeEach(() => {
  vi.mocked(fetchWatchlist).mockReset()
  vi.mocked(triggerBackfill).mockReset()
  vi.mocked(getBackfillJob).mockReset()
  vi.mocked(triggerChipSync).mockReset()
  vi.mocked(fetchWatchlist).mockResolvedValue([
    { symbol: '2330', name: '台積電' },
    { symbol: '2454', name: '聯發科' },
  ] as never)
  vi.mocked(triggerBackfill).mockResolvedValue(job())
})

afterEach(() => {
  vi.useRealTimers()
})

// 兩塊回補 UI 的按鈕文字相同（「開始回補」），用區塊標題往上找容器再取按鈕，
// 避免抓錯到籌碼那一塊。
async function renderPage() {
  render(Backfill)
  const heading = await screen.findByText('股價資料回補')
  const section = heading.closest('div.bg-panel') as HTMLElement
  return {
    section,
    input: section.querySelector('input[placeholder*="留空"]') as HTMLInputElement,
    daysInput: section.querySelector('input[type="number"]') as HTMLInputElement,
    submit: () => {
      const btns = Array.from(section.querySelectorAll('button'))
      return btns.find((b) => b.textContent?.includes('回補')) as HTMLButtonElement
    },
  }
}

describe('股價回補的代號輸入', () => {
  it('填入代號時只送出填入的股票，不受監控清單影響', async () => {
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '1101, 2603 6505' } })
    await fireEvent.click(page.submit())

    await waitFor(() => expect(triggerBackfill).toHaveBeenCalledTimes(1))
    // 逗號與空白都要當分隔（使用者從試算表貼上時兩種都會出現）。
    expect(vi.mocked(triggerBackfill).mock.calls[0][0]).toEqual({
      days: 120,
      symbols: ['1101', '2603', '6505'],
    })
  })

  it('留空時 fallback 為整個監控清單（symbols 仍由前端填入，不是空陣列）', async () => {
    const page = await renderPage()

    await fireEvent.click(page.submit())

    await waitFor(() => expect(triggerBackfill).toHaveBeenCalledTimes(1))
    expect(vi.mocked(triggerBackfill).mock.calls[0][0].symbols).toEqual(['2330', '2454'])
  })

  it('留空且監控清單也是空的時候顯示錯誤且不打 API', async () => {
    vi.mocked(fetchWatchlist).mockResolvedValue([] as never)
    const page = await renderPage()

    await fireEvent.click(page.submit())

    expect(await screen.findByText(/沒有可回補的股票/)).toBeInTheDocument()
    expect(triggerBackfill).not.toHaveBeenCalled()
  })

  it('天數會一起送出', async () => {
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '2330' } })
    await fireEvent.input(page.daysInput, { target: { value: '1825' } })
    await fireEvent.click(page.submit())

    await waitFor(() => expect(triggerBackfill).toHaveBeenCalledTimes(1))
    expect(vi.mocked(triggerBackfill).mock.calls[0][0].days).toBe(1825)
  })

  it('天數 <= 0 時顯示錯誤且不打 API', async () => {
    const page = await renderPage()

    await fireEvent.input(page.daysInput, { target: { value: '0' } })
    await fireEvent.click(page.submit())

    expect(await screen.findByText('回補天數需大於 0')).toBeInTheDocument()
    expect(triggerBackfill).not.toHaveBeenCalled()
  })
})

describe('勾選監控清單的舊 UI 已移除', () => {
  it('沒有全選按鈕與 checkbox', async () => {
    const page = await renderPage()

    expect(screen.queryByText('全選')).not.toBeInTheDocument()
    expect(screen.queryByText('取消全選')).not.toBeInTheDocument()
    expect(page.section.querySelectorAll('input[type="checkbox"]')).toHaveLength(0)
  })
})

describe('回補進度輪詢', () => {
  it('送出後顯示 job_id 與 pending 狀態', async () => {
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '2330' } })
    await fireEvent.click(page.submit())

    expect(await screen.findByText('bf_20260807_000000_000')).toBeInTheDocument()
    expect(page.section.textContent).toContain('排隊中')
    expect(page.section.textContent).toContain('0/1 檔，失敗 0')
  })

  it('每 3 秒輪詢一次，完成後停止輪詢並解除按鈕禁用', async () => {
    vi.useFakeTimers()
    vi.mocked(getBackfillJob)
      .mockResolvedValueOnce(job({ status: 'running', symbols_done: 1, symbols_total: 2 }))
      .mockResolvedValueOnce(job({ status: 'done', symbols_done: 2, symbols_total: 2 }))
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '2330,2454' } })
    await fireEvent.click(page.submit())
    await vi.waitFor(() => expect(triggerBackfill).toHaveBeenCalled())

    await vi.advanceTimersByTimeAsync(3000)
    expect(getBackfillJob).toHaveBeenCalledWith('bf_20260807_000000_000')
    expect(page.section.textContent).toContain('同步中')
    expect(page.section.textContent).toContain('1/2 檔')
    // 未完成前按鈕保持禁用，避免使用者重複送出把 rate limit 燒光。
    expect(page.submit().disabled).toBe(true)

    await vi.advanceTimersByTimeAsync(3000)
    expect(page.section.textContent).toContain('完成')
    expect(page.submit().disabled).toBe(false)

    // done 之後不該再打 API。
    const callsAfterDone = vi.mocked(getBackfillJob).mock.calls.length
    await vi.advanceTimersByTimeAsync(9000)
    expect(vi.mocked(getBackfillJob).mock.calls).toHaveLength(callsAfterDone)
  })

  it('partial 時逐檔列出失敗原因', async () => {
    vi.useFakeTimers()
    vi.mocked(getBackfillJob).mockResolvedValue(
      job({
        status: 'partial',
        symbols_total: 2,
        symbols_done: 2,
        symbols_failed: 1,
        failures: [{ symbol: '2454', error: 'finmind 429' }],
      }),
    )
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '2330,2454' } })
    await fireEvent.click(page.submit())
    await vi.waitFor(() => expect(triggerBackfill).toHaveBeenCalled())
    await vi.advanceTimersByTimeAsync(3000)

    expect(page.section.textContent).toContain('部分成功')
    expect(page.section.textContent).toContain('2454: finmind 429')
  })

  it('輪詢失敗時顯示錯誤並停止輪詢', async () => {
    vi.useFakeTimers()
    vi.mocked(getBackfillJob).mockRejectedValue(new Error('boom'))
    const page = await renderPage()

    await fireEvent.input(page.input, { target: { value: '2330' } })
    await fireEvent.click(page.submit())
    await vi.waitFor(() => expect(triggerBackfill).toHaveBeenCalled())
    await vi.advanceTimersByTimeAsync(3000)

    expect(screen.getByText('查詢回補狀態失敗')).toBeInTheDocument()
    expect(page.submit().disabled).toBe(false)

    const calls = vi.mocked(getBackfillJob).mock.calls.length
    await vi.advanceTimersByTimeAsync(9000)
    expect(vi.mocked(getBackfillJob).mock.calls).toHaveLength(calls)
  })
})

describe('籌碼回補不受本次改動影響', () => {
  it('留空時仍 fallback 為整個監控清單', async () => {
    render(Backfill)
    const heading = await screen.findByText('籌碼資料回補')
    const section = heading.closest('div.bg-panel') as HTMLElement
    const btn = Array.from(section.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('回補'),
    ) as HTMLButtonElement

    vi.mocked(triggerChipSync).mockResolvedValue({ job_id: 'chip_1' } as never)
    await fireEvent.click(btn)

    await waitFor(() => expect(triggerChipSync).toHaveBeenCalledTimes(1))
    expect(vi.mocked(triggerChipSync).mock.calls[0][0].symbols).toEqual(['2330', '2454'])
  })
})
