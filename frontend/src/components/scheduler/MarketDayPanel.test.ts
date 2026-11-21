import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import MarketDayPanel from './MarketDayPanel.svelte'
import { clearMarketDayOverride, fetchMarketDayOverrides, setMarketDayOverride } from '../../lib/api/marketDays'
import { ApiError } from '../../lib/api/client'

// I-139：臨時休市的手動設定。三種狀態、原因必填、手動休市要確認全日休市、錯誤碼對照中文、操作紀錄。
vi.mock('../../lib/api/marketDays', () => ({
  fetchMarketDayOverrides: vi.fn(),
  setMarketDayOverride: vi.fn(),
  clearMarketDayOverride: vi.fn(),
}))

const base = {
  today: { date: '2026-11-02', is_trading: true, source: 'calendar' as const },
  current: [],
  history: [],
}

beforeEach(() => {
  vi.mocked(fetchMarketDayOverrides).mockReset().mockResolvedValue(base)
  vi.mocked(setMarketDayOverride).mockReset()
  vi.mocked(clearMarketDayOverride).mockReset()
})

describe('MarketDayPanel', () => {
  it('shows today and the full-day closure warning', async () => {
    render(MarketDayPanel)
    await waitFor(() => expect(screen.getByTestId('market-day-today').textContent).toContain('依日曆：開市'))
    expect(screen.getByTestId('market-day-warning').textContent).toContain('本設定代表全日休市')
  })

  it.each([
    [{ is_trading: false, source: 'manual' }, '手動休市'],
    [{ is_trading: true, source: 'manual' }, '手動照常開市'],
    [{ is_trading: true, source: 'fallback' }, '日曆取不到，照平日執行'],
    [{ is_trading: false, source: 'calendar' }, '依日曆：休市'],
  ])('labels today %o as %s', async (t, label) => {
    vi.mocked(fetchMarketDayOverrides).mockResolvedValue({ ...base, today: { date: '2026-11-02', ...t } as never })
    render(MarketDayPanel)
    await waitFor(() => expect(screen.getByTestId('market-day-today').textContent).toContain(label))
  })

  it('requires a reason and the full-day confirmation before closing', async () => {
    render(MarketDayPanel)
    await waitFor(() => screen.getByTestId('market-day-today'))
    await fireEvent.click(screen.getByTestId('market-day-save'))
    expect(screen.getByTestId('market-day-error').textContent).toBe('請填寫原因')
    await fireEvent.input(screen.getByTestId('market-day-reason'), { target: { value: '證交所公告颱風全日休市' } })
    await fireEvent.click(screen.getByTestId('market-day-save'))
    expect(screen.getByTestId('market-day-error').textContent).toBe('請先確認交易所公告的是全日休市')
    expect(setMarketDayOverride).not.toHaveBeenCalled()

    vi.mocked(setMarketDayOverride).mockResolvedValue({
      override: { id: 1, trade_date: '2026-11-02', action: 'closed', reason: '證交所公告颱風全日休市', user_id: 7, created_at: '2026-11-02T00:00:00Z' },
      trading_day: { is_trading: false, source: 'manual' },
    })
    await fireEvent.click(screen.getByTestId('market-day-confirm'))
    await fireEvent.click(screen.getByTestId('market-day-save'))
    await waitFor(() => expect(setMarketDayOverride).toHaveBeenCalledWith('2026-11-02', 'closed', '證交所公告颱風全日休市'))
    await waitFor(() => expect(screen.getByTestId('market-day-message').textContent).toContain('下一輪排程生效'))
  })

  // v1.2 實作 review [P2]：勾選確認之後換日期，確認要清掉、必須重新勾選，⛔ 不能把休市設到新日期。
  it('binds the full-day confirmation to the selected date', async () => {
    vi.mocked(setMarketDayOverride).mockResolvedValue({
      override: { id: 3, trade_date: '2026-11-04', action: 'closed', reason: '公告', user_id: 7, created_at: '2026-11-02T00:00:00Z' },
      trading_day: { is_trading: false, source: 'manual' },
    })
    render(MarketDayPanel)
    await waitFor(() => screen.getByTestId('market-day-today'))
    await fireEvent.input(screen.getByTestId('market-day-reason'), { target: { value: '公告' } })
    const confirm = screen.getByTestId('market-day-confirm') as HTMLInputElement
    await fireEvent.click(confirm)
    expect(confirm.checked).toBe(true)

    await fireEvent.input(screen.getByTestId('market-day-date'), { target: { value: '2026-11-04' } })
    expect((screen.getByTestId('market-day-confirm') as HTMLInputElement).checked).toBe(false)
    await fireEvent.click(screen.getByTestId('market-day-save'))
    expect(screen.getByTestId('market-day-error').textContent).toBe('請先確認交易所公告的是全日休市')
    expect(setMarketDayOverride).not.toHaveBeenCalled()

    await fireEvent.click(screen.getByTestId('market-day-confirm'))
    expect(screen.getByTestId('market-day-confirm').parentElement?.textContent).toContain('2026-11-04 全日休市')
    await fireEvent.click(screen.getByTestId('market-day-save'))
    await waitFor(() => expect(setMarketDayOverride).toHaveBeenCalledWith('2026-11-04', 'closed', '公告'))
  })

  it('maps API error codes to text', async () => {
    vi.mocked(setMarketDayOverride).mockRejectedValue(new ApiError(422, 'past_date'))
    render(MarketDayPanel)
    await waitFor(() => screen.getByTestId('market-day-today'))
    await fireEvent.click(screen.getByTestId('market-day-action-open'))
    await fireEvent.input(screen.getByTestId('market-day-reason'), { target: { value: 'x' } })
    await fireEvent.click(screen.getByTestId('market-day-save'))
    await waitFor(() => expect(screen.getByTestId('market-day-error').textContent).toBe('只能設定今天或之後的日期'))
  })

  it('clears with a reason and lists the history', async () => {
    vi.mocked(fetchMarketDayOverrides).mockResolvedValue({
      ...base,
      current: [{ id: 2, trade_date: '2026-11-03', action: 'closed', reason: '颱風', user_id: 7, created_at: '2026-11-02T01:00:00Z' }],
      history: [{ id: 2, trade_date: '2026-11-03', action: 'closed', reason: '颱風', user_id: 7, created_at: '2026-11-02T01:00:00Z' }],
    })
    vi.mocked(clearMarketDayOverride).mockRejectedValueOnce(new ApiError(409, 'no_active_override'))
    render(MarketDayPanel)
    await waitFor(() => expect(screen.getByTestId('market-day-current').textContent).toContain('2026-11-03：手動休市（颱風）'))
    expect(screen.getByTestId('market-day-history').textContent).toContain('使用者 #7')
    await fireEvent.input(screen.getByTestId('market-day-reason'), { target: { value: '誤設' } })
    await fireEvent.click(screen.getByTestId('market-day-clear'))
    await waitFor(() => expect(screen.getByTestId('market-day-error').textContent).toBe('這一天沒有生效中的手動設定'))
    expect(clearMarketDayOverride).toHaveBeenCalledWith('2026-11-02', '誤設')
  })
})
