import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import { fetchIntradayStatus, runIntradayRefill } from './intraday'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
  vi.mocked(apiFetch).mockResolvedValue({})
})

describe('intraday api', () => {
  it('reads the status endpoint', async () => {
    await fetchIntradayStatus()
    expect(apiFetch).toHaveBeenCalledWith('/intraday/status')
  })

  it('posts the refill with or without symbols', async () => {
    await runIntradayRefill()
    await runIntradayRefill(['2330'])
    expect(apiFetch).toHaveBeenNthCalledWith(1, '/scheduler/intraday-refill/run', { method: 'POST', body: '{}' })
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/scheduler/intraday-refill/run', {
      method: 'POST',
      body: '{"symbols":["2330"]}',
    })
  })
})
