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

export async function updateWatchlist(symbol: string, name: string, sector = ''): Promise<void> {
  await apiFetch(`/watchlist/${symbol}`, {
    method: 'PUT',
    body: JSON.stringify({ name, sector }),
  })
}

export async function removeFromWatchlist(symbol: string): Promise<void> {
  await apiFetch(`/watchlist/${symbol}`, { method: 'DELETE' })
}

// 設定/取消即時監聽（WebSocket 訂閱），最多同時 3 檔；超過上限時後端回 409，
// apiFetch 的錯誤訊息會包含狀態碼（"API ... failed: 409"），呼叫端可以用
// error.message.includes('409') 判斷是否為「已達上限」
export async function setWatched(symbol: string, watched: boolean): Promise<void> {
  await apiFetch(`/watchlist/${symbol}/watch`, {
    method: 'PATCH',
    body: JSON.stringify({ watched }),
  })
}
