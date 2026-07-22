import { apiFetch } from './client'
import type { WatchlistItem } from '../stores/market'

export interface StockSymbol {
  id: number
  symbol: string
  name: string
  isin_code: string
  market: string
  security_type: string
  industry: string
  cfi_code: string
  remarks: string
  listed_date?: string | null
  is_listed: boolean
  last_seen_at: string
}

export async function fetchWatchlist(): Promise<WatchlistItem[]> {
  const res = await apiFetch<{ watchlist: WatchlistItem[] }>('/watchlist')
  return res.watchlist ?? []
}

export async function searchStockSymbols(
  q: string,
  opts: { listed?: boolean; limit?: number; securityType?: string } = {},
): Promise<StockSymbol[]> {
  const params = new URLSearchParams()
  params.set('q', q)
  params.set('limit', String(opts.limit ?? 20))
  if (opts.listed !== undefined) params.set('listed', String(opts.listed))
  if (opts.securityType) params.set('security_type', opts.securityType)
  const res = await apiFetch<{ symbols: StockSymbol[] }>(`/stock-symbols/search?${params.toString()}`)
  return res.symbols ?? []
}

export async function addToWatchlist(symbol: string, name = '', sector = ''): Promise<void> {
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

export async function bulkAddSymbolsToWatchlist(symbols: string[]): Promise<{ added: number; failed: number }> {
  return apiFetch('/watchlist/bulk', {
    method: 'POST',
    body: JSON.stringify({ symbols }),
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
