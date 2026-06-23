import { writable } from 'svelte/store'

export interface WatchlistItem {
  id: number
  symbol: string
  name: string
  sector: string
}

export interface StockQuote {
  symbol: string
  name: string
  close: number
  open: number
  high: number
  low: number
  change: number
  changePct: number
  volume: number
  volRatio: number
  ma5: number
  ma20: number
  rsi14: number
  trend: 'BULLISH' | 'BEARISH' | 'SIDEWAYS' | ''
  hasSignal: boolean
}

export const watchlist = writable<WatchlistItem[]>([])
export const quotes = writable<Map<string, StockQuote>>(new Map())
export const selectedSymbol = writable<string>('')
