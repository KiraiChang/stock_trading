import { taipeiDateOf } from './date'

/**
 * 這筆分析用的籌碼，相對它站的那根 K 棒是不是當天的。
 *
 * 判讀規則與同日兩輪的欄位差異見
 * `docs/sr-zone-scoring.md`「`trade_date`：這份籌碼是哪一天的」。
 *
 * **為什麼需要**：SR 分析排程一天跑兩輪（17:00／22:00），兩輪站在**同一根 K 棒**上——
 * `analyzed_at` 與 `current_price` 完全相同，差別只在籌碼，而 Chip 佔 `trading_score`
 * 的 15%。少了這個標示，歷史清單上會出現兩列看起來一模一樣、分數卻不同的紀錄。
 *
 * FinMind 的法人／融資券晚間才發布，所以 17:00 那輪拿到的必然是前一日籌碼。
 */
export interface ChipFreshness {
  label: string
  /** true 代表籌碼不是這根 K 棒當天的（含完全沒有籌碼）。 */
  stale: boolean
}

export function chipFreshness(
  analyzedAt: string,
  chip?: { missing?: boolean; trade_date?: string | null } | null,
): ChipFreshness | null {
  if (!chip || chip.missing) return { label: '無籌碼', stale: true }
  const tradeDate = chip.trade_date
  if (!tradeDate) return null

  // **基準是 K 棒的台北日曆日，不是 UTC 日期**：日 K 的 ts 是 16:00Z＝台北隔日 00:00，
  // 用 UTC 比會整批差一天（同一個坑在 Go 端的 taipeiDate 也踩過）。
  const bar = taipeiDateOf(analyzedAt)
  if (tradeDate === bar) return { label: '當日籌碼', stale: false }

  // **不寫死「前一日」**：籌碼可能落後不只一天（採集失敗、停牌、連假），
  // 直接把日期攤出來比一個可能說謊的相對詞誠實。
  return { label: `籌碼 ${tradeDate.slice(5)}`, stale: true }
}
