import { writable } from 'svelte/store'

export type Route = 'dashboard' | 'users' | 'scheduler' | 'backfill'

export const currentRoute = writable<Route>('dashboard')
