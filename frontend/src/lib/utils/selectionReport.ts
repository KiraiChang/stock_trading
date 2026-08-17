import type { EvaluationUniverseUpsertItem } from '../api/evaluationUniverse'

/**
 * 把 `scripts/build-selection-report.sh` 產出的 JSON 轉成匯入用的 items。
 *
 * **為什麼吃整份報告而不是讓使用者手打**：`bucket_hint` 是逐檔不同的，而
 * `bucket_edge_low/high` 是 17 位有效數字的凍結分位數——手打必然出錯，
 * 而錯了不會有任何東西擋下（DB 的 CHECK 只驗 high > low > 0）。
 * 報告裡已經有全部資訊，直接取用是唯一不會漂的做法。
 *
 * 純函式，不碰 DOM 也不發請求。
 */

export interface ParsedSelectionReport {
  items: EvaluationUniverseUpsertItem[]
  /** 依 bucket 統計，供匯入前核對三桶樣本是否足夠。 */
  bucketCounts: Record<string, number>
  /** 依 universe_role 統計：supplemental 不參與股票 builder 決策，數量要看得見。 */
  roleCounts: Record<string, number>
  edges: [number, number]
  /** `threshold_gap.aligned`：報告的切點是否等於 pipeline 當下的凍結門檻。 */
  aligned: boolean | null
  pipelineThresholds: { low: number; high: number } | null
  /** 不阻擋匯入但必須讓使用者看到的問題。 */
  warnings: string[]
}

export class SelectionReportError extends Error {}

interface ReportRow {
  symbol?: unknown
  selection_bucket?: unknown
  universe_role?: unknown
}

function asNumberPair(value: unknown): [number, number] | null {
  if (!Array.isArray(value) || value.length !== 2) return null
  const [a, b] = value
  if (typeof a !== 'number' || typeof b !== 'number') return null
  if (!Number.isFinite(a) || !Number.isFinite(b)) return null
  return [a, b]
}

export function parseSelectionReport(
  raw: string,
  opts: { universeVersion: string; source: string },
): ParsedSelectionReport {
  let doc: unknown
  try {
    doc = JSON.parse(raw)
  } catch {
    throw new SelectionReportError('不是合法的 JSON')
  }
  if (typeof doc !== 'object' || doc === null) {
    throw new SelectionReportError('不是合法的 JSON 物件')
  }
  const report = doc as Record<string, unknown>

  const universe = report.universe as Record<string, unknown> | undefined
  const selected = universe?.selected_symbols
  if (!Array.isArray(selected) || selected.length === 0) {
    throw new SelectionReportError('找不到 universe.selected_symbols——這份 JSON 不是 selection report？')
  }

  const edges = asNumberPair(report.quantile_edges)
  if (!edges) {
    throw new SelectionReportError('找不到 quantile_edges——沒有它無法記錄 bucket_hint 的判定依據')
  }
  if (!(edges[0] > 0) || !(edges[1] > edges[0])) {
    throw new SelectionReportError(`quantile_edges 不合法：${JSON.stringify(edges)}`)
  }

  // **權威的 supplemental 清單是 universe.supplemental_symbols，不是 row.universe_role。**
  // 兩者不等價：前者另外包含「分級保留下來的不合格 watchlist」，那些標的的
  // row.universe_role 仍是 primary（後綴判定只看 ETF 代號）。只讀 row 會讓一檔掉到
  // low_liquidity 的 watchlist 股票被存成 primary 而正式參與股票 builder 調參——
  // 正是選取原則第 3 點要避免的事。
  const supplemental = (universe?.supplemental_symbols ?? {}) as Record<string, unknown>
  const depthDetail = (universe?.insufficient_depth_detail ?? {}) as Record<string, unknown>

  const rows = Array.isArray(report.symbols) ? (report.symbols as ReportRow[]) : []
  const bySymbol = new Map<string, ReportRow>()
  for (const row of rows) {
    if (typeof row?.symbol === 'string') bySymbol.set(row.symbol, row)
  }

  const warnings: string[] = []
  const gap = report.threshold_gap as Record<string, unknown> | undefined
  const aligned = typeof gap?.aligned === 'boolean' ? gap.aligned : null
  const absolute = gap?.pipeline_absolute as Record<string, unknown> | undefined
  const pipelineThresholds =
    typeof absolute?.low_max === 'number' && typeof absolute?.high_min === 'number'
      ? { low: absolute.low_max, high: absolute.high_min }
      : null

  if (aligned === false) {
    // 這是最重要的一條警告：報告的切點與 pipeline 的凍結門檻不同，代表存進去的
    // bucket_hint 與 runtime 算出的 bucket 會不一致——那正是凍結機制要避免的事。
    warnings.push(
      '報告的分位數切點與 pipeline 當下的門檻不同（threshold_gap.aligned=false）。' +
        '存進去的 bucket_hint 會與 runtime 算出的 bucket 不一致；' +
        '先確認是否該升 universe_version 或重定門檻。',
    )
  } else if (aligned === null) {
    warnings.push('這份報告沒有 threshold_gap.aligned 欄位（舊版），無法自動判斷切點是否與 pipeline 門檻一致。')
  }

  const items: EvaluationUniverseUpsertItem[] = []
  const bucketCounts: Record<string, number> = {}
  const roleCounts: Record<string, number> = {}
  const missing: string[] = []
  for (const sym of selected) {
    if (typeof sym !== 'string') continue
    const row = bySymbol.get(sym)
    const bucket = typeof row?.selection_bucket === 'string' ? row.selection_bucket : ''
    if (!bucket) {
      // 沒有 bucket 就無法決定 bucket_hint。**不要用預設值帶過**——
      // 那會讓一個沒有依據的 bucket 進入調參母體。
      missing.push(sym)
      continue
    }
    if (bucket === 'STALE') {
      warnings.push(`${sym} 的 bucket 是 STALE（資料不新鮮），不該進調參母體。`)
    }
    bucketCounts[bucket] = (bucketCounts[bucket] ?? 0) + 1

    const supReason = supplemental[sym]
    const role = sym in supplemental
      ? 'supplemental'
      : typeof row?.universe_role === 'string'
        ? row.universe_role
        : 'primary'

    // note 保留「為什麼是 supplemental」與「深度不足到什麼程度」——
    // 這兩件事只在報告裡，不存下來就得回頭翻當時那份 JSON（計畫書指定 note 承接它們）。
    const notes: string[] = []
    if (typeof supReason === 'string' && supReason !== 'etf_suffix') {
      notes.push(`supplemental:${supReason}`)
    }
    const depth = depthDetail[sym] as { total_candle_count?: unknown; kind?: unknown } | undefined
    if (depth && typeof depth.kind === 'string') {
      notes.push(`insufficient_depth:${depth.kind}(${String(depth.total_candle_count ?? '?')})`)
    }

    items.push({
      symbol: sym,
      bucket_hint: bucket,
      bucket_edge_low: edges[0],
      bucket_edge_high: edges[1],
      universe_version: opts.universeVersion,
      universe_role: role,
      source: opts.source,
      note: notes.join('; '),
    })
    roleCounts[role] = (roleCounts[role] ?? 0) + 1
  }

  if (missing.length > 0) {
    throw new SelectionReportError(
      `${missing.length} 檔在 symbols[] 裡找不到 selection_bucket：${missing.slice(0, 5).join(', ')}` +
        (missing.length > 5 ? ' …' : ''),
    )
  }

  return { items, bucketCounts, roleCounts, edges, aligned, pipelineThresholds, warnings }
}
