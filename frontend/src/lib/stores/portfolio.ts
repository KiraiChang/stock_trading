import { writable } from 'svelte/store'

const SELECTED_PORTFOLIO_KEY = 'trading_selected_portfolio_id'
const initial = Number(localStorage.getItem(SELECTED_PORTFOLIO_KEY) || '1') || 1

export const selectedPortfolioID = writable<number>(initial)

selectedPortfolioID.subscribe((id) => {
  if (id > 0) localStorage.setItem(SELECTED_PORTFOLIO_KEY, String(id))
})
