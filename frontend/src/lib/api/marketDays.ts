import { apiFetch } from './client'
import type { MarketDayOverride, TradingDay } from './intraday'

// 臨時休市（颱風假）與推翻日曆的手動設定（I-139）。規格見 docs/api-reference.md 的 /market-day-overrides。
// 每次儲存與取消各留一筆紀錄（操作者、時間、原因），下一次判斷就生效；只接受今天或未來的平日。

export interface MarketDayToday extends TradingDay {
  date: string
}

export interface MarketDayOverridesResponse {
  today: MarketDayToday
  current: MarketDayOverride[]
  history: MarketDayOverride[]
}

export interface MarketDayWriteResponse {
  override: MarketDayOverride
  trading_day: TradingDay
}

export async function fetchMarketDayOverrides(from?: string): Promise<MarketDayOverridesResponse> {
  const q = from ? `?from=${encodeURIComponent(from)}` : ''
  return apiFetch<MarketDayOverridesResponse>(`/market-day-overrides${q}`)
}

export async function setMarketDayOverride(
  date: string,
  action: 'closed' | 'open',
  reason: string,
): Promise<MarketDayWriteResponse> {
  return apiFetch<MarketDayWriteResponse>(`/market-day-overrides/${encodeURIComponent(date)}`, {
    method: 'PUT',
    body: JSON.stringify({ action, reason }),
  })
}

export async function clearMarketDayOverride(date: string, reason: string): Promise<MarketDayWriteResponse> {
  return apiFetch<MarketDayWriteResponse>(`/market-day-overrides/${encodeURIComponent(date)}`, {
    method: 'DELETE',
    body: JSON.stringify({ reason }),
  })
}
