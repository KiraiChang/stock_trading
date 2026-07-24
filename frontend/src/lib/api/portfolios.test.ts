import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import { createPortfolio, listPortfolios } from './portfolios'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('portfolios api', () => {
  it('lists and creates portfolios', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({ portfolios: [] })
    vi.mocked(apiFetch).mockResolvedValueOnce({ portfolio: { id: 9, name: 'Desk' } })

    await listPortfolios()
    await createPortfolio({ name: 'Desk', group_id: 2 })

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/portfolios')
    const [, init] = vi.mocked(apiFetch).mock.calls[1]
    expect(JSON.parse(String(init?.body))).toEqual({ name: 'Desk', group_id: 2 })
  })
})
