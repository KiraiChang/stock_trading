import { writable, derived } from 'svelte/store'

export interface Signal {
  id: number
  symbol: string
  signal_type: 'BREAKOUT' | 'BREAKDOWN' | 'VOLUME_SPIKE'
  direction: 'BUY' | 'SELL' | 'WATCH'
  price: number
  volume: number
  vol_ratio: number
  resistance: number
  support: number
  trend: string
  note: string
  ts: string
}

export const signals = writable<Signal[]>([])
export const unreadCount = derived(signals, ($s) => $s.length)
