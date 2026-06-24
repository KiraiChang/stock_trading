import { writable } from 'svelte/store'

export type Route = 'dashboard' | 'users'

export const currentRoute = writable<Route>('dashboard')
