import { apiFetch } from './client'

/**
 * 評估標的池（T-040 Step 5）。
 *
 * **這個池不是 watchlist**：它只驅動每日盤後的日 K 維護，不進盤中掃描、籌碼同步、
 * signal 或 production SR 分析。規格見 docs/evaluation-universe-selection-plan.md
 * 的「Step 5 執行計畫書」。
 */
export interface EvaluationUniverseEntry {
  id: number
  symbol: string
  bucket_hint: string
  /**
   * 入池時**實際使用的分位數邊界**。
   *
   * 為什麼每一列都存：`bucket_hint` 單獨存在無法回答「這個 bucket 是用哪組邊界判的」。
   * 實測 2026-08-17 有 3 檔 `atr_pct` 完全未變卻換桶，只因母體變了、邊界移動。
   */
  bucket_edge_low: number
  bucket_edge_high: number
  universe_version: string
  /** `primary` 參與股票 builder 決策；`supplemental` 只作交叉觀察。 */
  universe_role: string
  selected_at: string
  source: string
  /** false ＝ 保留紀錄但不再納入每日維護。刻意不刪除：入退池歷史本身是研究紀錄。 */
  active: boolean
  note: string
}

export interface EvaluationUniverseList {
  items: EvaluationUniverseEntry[]
  total: number
  active_count: number
  /** 只統計 active 成員——停用的標的不再進 evaluation，算進去會高估樣本量。 */
  active_buckets: Record<string, number>
}

/** 預設回全部（含停用者）；`activeOnly` 才只取仍在維護的成員。 */
export async function fetchEvaluationUniverse(activeOnly = false): Promise<EvaluationUniverseList> {
  const qs = activeOnly ? '?active=true' : ''
  return apiFetch<EvaluationUniverseList>(`/evaluation-universe${qs}`)
}

export interface EvaluationUniverseUpsertItem {
  symbol: string
  bucket_hint: string
  bucket_edge_low: number
  bucket_edge_high: number
  universe_version: string
  universe_role?: string
  source: string
  note?: string
}

/**
 * 匯入（或更新）選池成員。
 *
 * **刻意不接受 `active`**：入退池是獨立的人工決定，不該被一次重新匯入靜默覆寫。
 * 要改用 `setEvaluationUniverseActive`。`selected_at` 由伺服器決定。
 */
export async function upsertEvaluationUniverse(
  items: EvaluationUniverseUpsertItem[],
): Promise<{ upserted: number }> {
  return apiFetch<{ upserted: number }>('/evaluation-universe', {
    method: 'POST',
    body: JSON.stringify({ items }),
  })
}

/** 切換單一標的是否納入每日日 K 維護。標的不在池內時後端回 404。 */
export async function setEvaluationUniverseActive(
  symbol: string,
  active: boolean,
): Promise<{ symbol: string; active: boolean }> {
  return apiFetch<{ symbol: string; active: boolean }>(
    `/evaluation-universe/${encodeURIComponent(symbol)}`,
    { method: 'PATCH', body: JSON.stringify({ active }) },
  )
}

/** 手動觸發每日維護。cron 預設關閉，所以在排程開啟前這是唯一的對齊方式。 */
export async function triggerEvaluationUniverseSync(): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('/scheduler/evaluation-universe-sync/run', {
    method: 'POST',
  })
}
