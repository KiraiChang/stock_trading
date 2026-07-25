import { writable } from 'svelte/store'

const SELECTED_PORTFOLIO_KEY = 'trading_selected_portfolio_id'
const initial = Number(localStorage.getItem(SELECTED_PORTFOLIO_KEY) || '0') || 0

export const selectedPortfolioID = writable<number>(initial)

selectedPortfolioID.subscribe((id) => {
  if (id > 0) localStorage.setItem(SELECTED_PORTFOLIO_KEY, String(id))
  else localStorage.removeItem(SELECTED_PORTFOLIO_KEY)
})
