import { describe, expect, it, vi } from 'vitest'
import { render } from '@testing-library/svelte'

vi.mock('../lib/api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/client')>()),
  apiFetch: vi.fn().mockResolvedValue({}),
}))
vi.mock('../lib/api/watchlist', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/watchlist')>()),
  fetchWatchlist: vi.fn(),
}))
vi.mock('../lib/api/signals', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/signals')>()),
  fetchSignals: vi.fn(),
}))
vi.mock('../lib/ws/socket', () => ({
  socket: { connect: vi.fn(), subscribe: vi.fn(), unsubscribe: vi.fn() },
}))
vi.mock('../lib/utils/intradayPolling', () => ({
  startIntradayPolling: vi.fn(() => ({ stop: vi.fn() })),
}))

import Dashboard from './Dashboard.svelte'
import { fetchWatchlist } from '../lib/api/watchlist'
import { fetchSignals } from '../lib/api/signals'
import { socket } from '../lib/ws/socket'
import { startIntradayPolling } from '../lib/utils/intradayPolling'

describe('Dashboard lifecycle', () => {
  // review 2026-11-20：初次請求還沒回來就切頁，⛔ 不能在已銷毀的頁面啟動輪詢與 WebSocket。
  it('does not start polling when unmounted before the first requests finish', async () => {
    let resolveWatchlist!: (v: []) => void
    vi.mocked(fetchWatchlist).mockReturnValue(new Promise((r) => { resolveWatchlist = r }))
    vi.mocked(fetchSignals).mockResolvedValue([])

    const { unmount } = render(Dashboard)
    unmount()
    resolveWatchlist([])
    await new Promise((r) => setTimeout(r, 0))

    expect(startIntradayPolling).not.toHaveBeenCalled()
    expect(socket.connect).not.toHaveBeenCalled()
  })

  it('starts polling once after the first requests finish', async () => {
    vi.mocked(startIntradayPolling).mockClear()
    vi.mocked(fetchWatchlist).mockResolvedValue([])
    vi.mocked(fetchSignals).mockResolvedValue([])

    const { unmount } = render(Dashboard)
    await new Promise((r) => setTimeout(r, 0))
    expect(startIntradayPolling).toHaveBeenCalledTimes(1)
    unmount()
  })
})
