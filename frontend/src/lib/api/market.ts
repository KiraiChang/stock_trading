import { apiFetch } from './client'

export interface BackfillOptions {
  days?: number
  symbols?: string[]
}

export async function triggerBackfill(opts: BackfillOptions = {}): Promise<{ symbols: number; days: number }> {
  return apiFetch('/market/backfill', {
    method: 'POST',
    body: JSON.stringify({ days: opts.days ?? 120, symbols: opts.symbols }),
  })
}
