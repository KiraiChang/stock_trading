import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import { analyzeTrade, listTradeAnalyses } from './tradeAnalysis'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('trade analysis api portfolio scope', () => {
  it('sends portfolio_id for analysis writes and history reads', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({ analysis: {} })
    vi.mocked(apiFetch).mockResolvedValueOnce({ analyses: [] })

    await analyzeTrade('2330', 7, true)
    await listTradeAnalyses('2330', 7)

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    expect(JSON.parse(String(init?.body))).toMatchObject({ symbol: '2330', portfolio_id: 7, force_refresh: true })
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/trade-analysis/2330/history?limit=20&portfolio_id=7')
  })
})
