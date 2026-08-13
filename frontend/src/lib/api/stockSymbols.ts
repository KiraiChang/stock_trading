import { apiFetch } from './client'
import type { StockSymbol } from './watchlist'

// 研究用的候選標的清單（後端 GET /stock-symbols/candidates，見 docs/todo.md T-040）。
//
// **為什麼另開一個模組**：`/stock-symbols/search` 借放在 watchlist.ts，因為那支端點的用途
// 就是 watchlist 新增股票時的 autocomplete。候選清單與 watchlist 無關——它是研究母體的
// 產生器，回傳的 symbols 直接餵給 market/backfill。放同一個檔案會讓兩種用途互相牽制。
// `StockSymbol` 型別本身是共用的，從 watchlist.ts re-export 而不是複製一份。
export type { StockSymbol }

export interface SymbolCandidateOptions {
  /** 留空時**後端**會套用預設 `股票,ETF`；前端不重複實作這個預設值。 */
  securityTypes?: string[]
  industries?: string[]
  /** 上市滿幾年。0 或 undefined = 不限。 */
  listedYears?: number
  /** 每個產業最多幾檔。0 或 undefined = 不限。 */
  perIndustry?: number
  limit?: number
  includeDelisted?: boolean
}

export interface SymbolCandidatesResult {
  count: number
  /** 扁平代號清單，可直接餵給 triggerBackfill。 */
  symbols: string[]
  /** 產業 → 取樣檔數。用來人工核對抽樣有沒有被單一產業壓垮。 */
  by_industry: Record<string, number>
  rows: StockSymbol[]
  /**
   * true 代表還有更多符合條件的標的被 limit 砍掉。**截斷依代號順序**，會整批砍掉
   * 高代號的產業，正是 per_industry 要消除的偏斜——拿到 true 時應該調高 limit
   * 或收緊條件，而不是直接使用這份清單。
   */
  truncated: boolean
}

/** 篩選選項與**母體**筆數（不是取樣後的數量）。 */
export interface SymbolFacet {
  value: string
  count: number
}

export interface SymbolFacets {
  security_types: SymbolFacet[]
  industries: SymbolFacet[]
}

/**
 * 取得可選的證券類型與產業。
 *
 * `securityTypes` 只縮放回傳的 industries，**不影響 security_types 清單**——
 * 選單本身要一直完整，否則使用者選了某個類型之後就換不回來。
 */
export async function fetchSymbolFacets(securityTypes?: string[]): Promise<SymbolFacets> {
  const params = new URLSearchParams()
  if (securityTypes?.length) params.set('security_type', securityTypes.join(','))
  const qs = params.toString()
  const res = await apiFetch<SymbolFacets>(`/stock-symbols/facets${qs ? `?${qs}` : ''}`)
  return {
    security_types: res.security_types ?? [],
    industries: res.industries ?? [],
  }
}

export async function fetchSymbolCandidates(
  opts: SymbolCandidateOptions = {},
): Promise<SymbolCandidatesResult> {
  const params = new URLSearchParams()
  if (opts.securityTypes?.length) params.set('security_type', opts.securityTypes.join(','))
  if (opts.industries?.length) params.set('industry', opts.industries.join(','))
  // 0 不送出：後端把 0 當成「不限制」，但不送更明確，也少一個會漂移的約定。
  if (opts.listedYears) params.set('listed_years', String(opts.listedYears))
  if (opts.perIndustry) params.set('per_industry', String(opts.perIndustry))
  if (opts.limit) params.set('limit', String(opts.limit))
  if (opts.includeDelisted) params.set('include_delisted', 'true')

  const qs = params.toString()
  const res = await apiFetch<SymbolCandidatesResult>(
    `/stock-symbols/candidates${qs ? `?${qs}` : ''}`,
  )
  return {
    count: res.count ?? 0,
    symbols: res.symbols ?? [],
    by_industry: res.by_industry ?? {},
    rows: res.rows ?? [],
    truncated: res.truncated ?? false,
  }
}
