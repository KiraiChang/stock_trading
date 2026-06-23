import { apiFetch } from './client'
import type { Signal } from '../stores/signals'

export async function fetchSignals(limit = 50, symbol?: string): Promise<Signal[]> {
  const query = symbol ? `?limit=${limit}&symbol=${symbol}` : `?limit=${limit}`
  const res = await apiFetch<{ signals: Signal[] }>(`/signals${query}`)
  return res.signals ?? []
}
