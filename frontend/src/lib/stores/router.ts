import { writable } from 'svelte/store'

export type Route = 'dashboard' | 'users' | 'scheduler'

export const currentRoute = writable<Route>('dashboard')
