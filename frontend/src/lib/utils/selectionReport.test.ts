import { describe, expect, it } from 'vitest'
import { parseSelectionReport, SelectionReportError } from './selectionReport'

// 2026-08-17 凍結的分位數邊界（= zone_builder 的 LOW/HIGH_VOLATILITY_THRESHOLD）
const LOW = 0.046089927430152715
const HIGH = 0.06278197721225691

const OPTS = { universeVersion: 'v2', source: 'T-040_STEP3' }

function report(overrides: Record<string, unknown> = {}): string {
  return JSON.stringify({
    quantile_edges: [LOW, HIGH],
    threshold_gap: {
      pipeline_absolute: { low_max: LOW, high_min: HIGH },
      aligned: true,
    },
    symbols: [
      { symbol: '0050', selection_bucket: 'LOW_VOLATILITY', universe_role: 'primary' },
      { symbol: '2330', selection_bucket: 'LOW_VOLATILITY', universe_role: 'primary' },
      { symbol: '2454', selection_bucket: 'HIGH_VOLATILITY', universe_role: 'primary' },
      { symbol: '00981A', selection_bucket: 'NORMAL_VOLATILITY', universe_role: 'supplemental' },
      { symbol: '9999', selection_bucket: 'LOW_VOLATILITY', universe_role: 'primary' },
    ],
    universe: { selected_symbols: ['0050', '2330', '2454', '00981A'] },
    ...overrides,
  })
}

describe('parseSelectionReport', () => {
  it('從報告取出逐檔 bucket 與凍結邊界', () => {
    const got = parseSelectionReport(report(), OPTS)

    expect(got.items).toHaveLength(4)
    // 只取 selected_symbols，報告裡其他標的（9999）不進池
    expect(got.items.map((i) => i.symbol)).toEqual(['0050', '2330', '2454', '00981A'])
    // 邊界逐檔照抄，不重算也不四捨五入——貼邊界的十幾檔會因取整而與 bucket_hint 不一致
    expect(got.items[0].bucket_edge_low).toBe(LOW)
    expect(got.items[0].bucket_edge_high).toBe(HIGH)
    expect(got.edges).toEqual([LOW, HIGH])
    expect(got.bucketCounts).toEqual({ LOW_VOLATILITY: 2, HIGH_VOLATILITY: 1, NORMAL_VOLATILITY: 1 })
    expect(got.warnings).toEqual([])
  })

  it('universe_role 照報告帶，不一律填 primary', () => {
    const got = parseSelectionReport(report(), OPTS)
    const a = got.items.find((i) => i.symbol === '00981A')
    // 主動式 ETF 是 supplemental：誤標 primary 會讓它影響股票的 builder 參數
    expect(a?.universe_role).toBe('supplemental')
    expect(got.roleCounts).toEqual({ primary: 3, supplemental: 1 })
  })

  it('supplemental 以 universe.supplemental_symbols 為準，不是只看 row.universe_role', () => {
    // 6243 是股票，row.universe_role=primary，但因為分級保留（low_liquidity）
    // 被報告列進 supplemental_symbols。只讀 row 會讓它正式參與股票 builder 調參。
    const got = parseSelectionReport(
      report({
        symbols: [
          { symbol: '6243', selection_bucket: 'HIGH_VOLATILITY', universe_role: 'primary' },
          { symbol: '2330', selection_bucket: 'LOW_VOLATILITY', universe_role: 'primary' },
        ],
        universe: {
          selected_symbols: ['6243', '2330'],
          supplemental_symbols: { '6243': 'low_liquidity' },
        },
      }),
      OPTS,
    )
    const s6243 = got.items.find((i) => i.symbol === '6243')
    expect(s6243?.universe_role).toBe('supplemental')
    // 原因要留在 note，否則得回頭翻當時那份 JSON
    expect(s6243?.note).toContain('supplemental:low_liquidity')
    expect(got.items.find((i) => i.symbol === '2330')?.universe_role).toBe('primary')
    expect(got.roleCounts).toEqual({ supplemental: 1, primary: 1 })
  })

  it('ETF 後綴造成的 supplemental 不寫進 note（那不是額外資訊）', () => {
    const got = parseSelectionReport(
      report({ universe: { selected_symbols: ['00981A'], supplemental_symbols: { '00981A': 'etf_suffix' } } }),
      OPTS,
    )
    expect(got.items[0].universe_role).toBe('supplemental')
    expect(got.items[0].note).toBe('')
  })

  it('深度不足的細節寫進 note', () => {
    const got = parseSelectionReport(
      report({
        universe: {
          selected_symbols: ['2330'],
          insufficient_depth_detail: { '2330': { total_candle_count: 299, kind: 'listing_age' } },
        },
      }),
      OPTS,
    )
    expect(got.items[0].note).toBe('insufficient_depth:listing_age(299)')
  })

  it('aligned=false 要出警告但不阻擋匯入', () => {
    const got = parseSelectionReport(
      report({ threshold_gap: { aligned: false, pipeline_absolute: { low_max: 0.015, high_min: 0.035 } } }),
      OPTS,
    )
    expect(got.items).toHaveLength(4)
    expect(got.aligned).toBe(false)
    expect(got.warnings.join()).toContain('bucket_hint 會與 runtime 算出的 bucket 不一致')
    expect(got.pipelineThresholds).toEqual({ low: 0.015, high: 0.035 })
  })

  it('舊版報告缺 aligned 欄位時明說無法判斷', () => {
    const got = parseSelectionReport(report({ threshold_gap: { pipeline_absolute: null } }), OPTS)
    expect(got.aligned).toBeNull()
    expect(got.warnings.join()).toContain('無法自動判斷')
  })

  it('STALE bucket 要出警告', () => {
    const got = parseSelectionReport(
      report({
        symbols: [
          { symbol: '4804', selection_bucket: 'STALE' },
          { symbol: '2330', selection_bucket: 'LOW_VOLATILITY' },
        ],
        universe: { selected_symbols: ['4804', '2330'] },
      }),
      OPTS,
    )
    expect(got.warnings.join()).toContain('4804')
    expect(got.warnings.join()).toContain('STALE')
  })

  it('選中的標的在 symbols[] 找不到 bucket 時直接拒絕', () => {
    // 不能用預設值帶過：那會讓一個沒有依據的 bucket 進入調參母體
    expect(() =>
      parseSelectionReport(
        report({ universe: { selected_symbols: ['0050', 'NOPE'] } }),
        OPTS,
      ),
    ).toThrow(/找不到 selection_bucket/)
  })

  it('缺 quantile_edges 直接拒絕', () => {
    expect(() => parseSelectionReport(report({ quantile_edges: null }), OPTS)).toThrow(
      /找不到 quantile_edges/,
    )
  })

  it('顛倒或非正的 edges 直接拒絕', () => {
    expect(() => parseSelectionReport(report({ quantile_edges: [HIGH, LOW] }), OPTS)).toThrow(
      /quantile_edges 不合法/,
    )
    expect(() => parseSelectionReport(report({ quantile_edges: [0, HIGH] }), OPTS)).toThrow(
      /quantile_edges 不合法/,
    )
  })

  it('不是 selection report 的 JSON 要說清楚', () => {
    expect(() => parseSelectionReport('{"foo":1}', OPTS)).toThrow(/selected_symbols/)
    expect(() => parseSelectionReport('not json', OPTS)).toThrow(SelectionReportError)
    expect(() => parseSelectionReport('[]', OPTS)).toThrow(/selected_symbols/)
  })

  it('universe_version 與 source 由呼叫端決定', () => {
    const got = parseSelectionReport(report(), { universeVersion: 'v3', source: 'MANUAL' })
    expect(got.items.every((i) => i.universe_version === 'v3' && i.source === 'MANUAL')).toBe(true)
  })
})
