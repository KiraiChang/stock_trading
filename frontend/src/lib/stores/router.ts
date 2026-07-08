import { writable } from 'svelte/store'

export type Route = 'dashboard' | 'users' | 'scheduler' | 'backfill' | 'backtest' | 'analysis' | 'sr-zones' | 'chips' | 'holdings'

export const currentRoute = writable<Route>('dashboard')
