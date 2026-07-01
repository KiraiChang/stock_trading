import { writable } from 'svelte/store'

export type Route = 'dashboard' | 'users' | 'scheduler' | 'backfill' | 'backtest'

export const currentRoute = writable<Route>('dashboard')
