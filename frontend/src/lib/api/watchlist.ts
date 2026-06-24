import { apiFetch } from './client'
import type { WatchlistItem } from '../stores/market'

export async function fetchWatchlist(): Promise<WatchlistItem[]> {
  const res = await apiFetch<{ watchlist: WatchlistItem[] }>('/watchlist')
  return res.watchlist ?? []
}

export async function addToWatchlist(symbol: string, name: string, sector = ''): Promise<void> {
  await apiFetch('/watchlist', {
    method: 'POST',
    body: JSON.stringify({ symbol, name, sector }),
  })
}

export async function bulkAddToWatchlist(
  items: { symbol: string; name: string; sector?: string }[],
): Promise<{ added: number; failed: number }> {
  return apiFetch('/watchlist/bulk', {
    method: 'POST',
    body: JSON.stringify({ items }),
  })
}

export async function removeFromWatchlist(symbol: string): Promise<void> {
  await apiFetch(`/watchlist/${symbol}`, { method: 'DELETE' })
}
