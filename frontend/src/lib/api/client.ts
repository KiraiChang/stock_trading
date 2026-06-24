import { authLogout, getToken } from '../stores/auth'

const BASE_URL = (import.meta.env.VITE_API_URL as string) ?? ''

export async function apiFetch<T>(path: string, opts?: RequestInit): Promise<T> {
  const token = getToken()
  const res = await fetch(`${BASE_URL}/api/v1${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...opts,
  })

  if (res.status === 401) {
    authLogout()
    throw new Error('Unauthorized')
  }

  if (!res.ok) throw new Error(`API ${path} failed: ${res.status}`)
  return res.json() as Promise<T>
}
