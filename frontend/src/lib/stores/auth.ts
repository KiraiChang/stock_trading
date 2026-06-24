import { writable, derived } from 'svelte/store'

const TOKEN_KEY = 'trading_token'
const EMAIL_KEY = 'trading_email'

const _token = writable<string>(localStorage.getItem(TOKEN_KEY) ?? '')
const _email = writable<string>(localStorage.getItem(EMAIL_KEY) ?? '')

export const authToken  = { subscribe: _token.subscribe }
export const authEmail  = { subscribe: _email.subscribe }
export const isAuthenticated = derived(_token, ($t) => !!$t)

export function authLogin(token: string, email: string) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(EMAIL_KEY, email)
  _token.set(token)
  _email.set(email)
}

export function authLogout() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(EMAIL_KEY)
  _token.set('')
  _email.set('')
}

/** 讀取當前 token（非響應式，用於 fetch 時注入 header） */
export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}
