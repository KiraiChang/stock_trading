import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import {
  addPositionTransaction,
  getPosition,
  listPositionTransactions,
  listPositions,
} from './positions'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('positions api portfolio scope', () => {
  it('reads positions with portfolio_id query', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ positions: [] })

    await listPositions(7)
    await getPosition('2330', 7)
    await listPositionTransactions('2330', 7)

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/positions?portfolio_id=7')
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/positions/2330?portfolio_id=7')
    expect(apiFetch).toHaveBeenNthCalledWith(3, '/positions/2330/transactions?portfolio_id=7')
  })

  it('writes transaction with portfolio_id in body', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ position: { symbol: '2330', portfolio_id: 7 } })

    await addPositionTransaction('2330', {
      portfolio_id: 7,
      event_type: 'BUY',
      shares: 100,
      price: 10,
      expected_version: 0,
    })

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    expect(JSON.parse(String(init?.body))).toMatchObject({ portfolio_id: 7, event_type: 'BUY' })
  })
})
