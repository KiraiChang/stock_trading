import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import EvaluationUniverse from './EvaluationUniverse.svelte'
import {
  fetchSymbolCandidates,
  fetchSymbolFacets,
  type SymbolCandidatesResult,
  type SymbolFacets,
} from '../lib/api/stockSymbols'
import { triggerBackfill, getBackfillJob, type MarketBackfillJob } from '../lib/api/market'
import {
  fetchEvaluationUniverse,
  setEvaluationUniverseActive,
  triggerEvaluationUniverseSync,
  upsertEvaluationUniverse,
  type EvaluationUniverseList,
} from '../lib/api/evaluationUniverse'

// 元件層測試：驗這頁的狀態機與送出的參數，API 形狀由後端 handler 測試把關。
vi.mock('../lib/api/stockSymbols', () => ({
  fetchSymbolCandidates: vi.fn(),
  fetchSymbolFacets: vi.fn(),
}))
vi.mock('../lib/api/market', () => ({ triggerBackfill: vi.fn(), getBackfillJob: vi.fn() }))
// ③ 已入池區塊在 onMount 就會查一次；不 mock 的話每支測試都會打真 fetch。
vi.mock('../lib/api/evaluationUniverse', () => ({
  fetchEvaluationUniverse: vi.fn(),
  setEvaluationUniverseActive: vi.fn(),
  triggerEvaluationUniverseSync: vi.fn(),
  upsertEvaluationUniverse: vi.fn(),
}))

function candidates(overrides: Partial<SymbolCandidatesResult> = {}): SymbolCandidatesResult {
  return {
    count: 2,
    symbols: ['2330', '2603'],
    by_industry: { 半導體業: 1, 航運業: 1 },
    rows: [],
    truncated: false,
    ...overrides,
  }
}

function facets(overrides: Partial<SymbolFacets> = {}): SymbolFacets {
  return {
    security_types: [
      { value: '上市認購(售)權證', count: 31090 },
      { value: '股票', count: 1945 },
      { value: 'ETF', count: 354 },
    ],
    industries: [
      { value: '半導體業', count: 201 },
      { value: '航運業', count: 34 },
    ],
    ...overrides,
  }
}

function job(overrides: Partial<MarketBackfillJob> = {}): MarketBackfillJob {
  return {
    id: 1,
    job_id: 'bf_20260813_000000_000',
    symbols: '["2330"]',
    days: 130,
    status: 'pending',
    symbols_total: 2,
    symbols_done: 0,
    symbols_failed: 0,
    failures: [],
    error: null,
    started_at: null,
    finished_at: null,
    created_at: '2026-08-13T00:00:00Z',
    ...overrides,
  }
}

beforeEach(() => {
  vi.mocked(fetchSymbolCandidates).mockReset()
  vi.mocked(fetchSymbolFacets).mockReset()
  vi.mocked(fetchSymbolFacets).mockResolvedValue(facets())
  vi.mocked(triggerBackfill).mockReset()
  vi.mocked(getBackfillJob).mockReset()
  vi.mocked(fetchSymbolCandidates).mockResolvedValue(candidates())
  vi.mocked(triggerBackfill).mockResolvedValue(job())
  vi.mocked(fetchEvaluationUniverse).mockReset()
  vi.mocked(fetchEvaluationUniverse).mockResolvedValue(emptyPool())
  vi.mocked(upsertEvaluationUniverse).mockReset()
  vi.mocked(upsertEvaluationUniverse).mockResolvedValue({ upserted: 2 })
  vi.mocked(setEvaluationUniverseActive).mockReset()
  vi.mocked(setEvaluationUniverseActive).mockResolvedValue({ symbol: '2330', active: false })
  vi.mocked(triggerEvaluationUniverseSync).mockReset()
  vi.mocked(triggerEvaluationUniverseSync).mockResolvedValue({ message: 'evaluation_universe_sync 已在背景重新觸發' })
})

function pool(overrides: Partial<EvaluationUniverseList> = {}): EvaluationUniverseList {
  return {
    items: [
      {
        id: 1, symbol: '2330', bucket_hint: 'LOW_VOLATILITY',
        bucket_edge_low: 0.046, bucket_edge_high: 0.063,
        universe_version: 'v2', universe_role: 'primary',
        selected_at: '2026-08-17T00:00:00+08:00', source: 'T-040_STEP3',
        active: true, note: '',
      },
    ],
    total: 1,
    active_count: 1,
    active_buckets: { LOW_VOLATILITY: 1 },
    ...overrides,
  }
}

function emptyPool(): EvaluationUniverseList {
  return { items: [], total: 0, active_count: 0, active_buckets: {} }
}

afterEach(() => vi.restoreAllMocks())

async function clickGenerate() {
  await fireEvent.click(screen.getByRole('button', { name: '產生候選清單' }))
}

describe('產生候選清單', () => {
  // 這是整頁最容易出錯、且錯了不會有任何東西報錯的地方：
  // 「上市滿 N 年」是 Step 3 的規則，量分佈階段帶上去會讓 ETF 從 354 檔掉到 199 檔，
  // 而 ETF 是目前唯一落在低波動 bucket 的類型。
  it('listed_years 預設不送出', async () => {
    render(EvaluationUniverse)
    await clickGenerate()

    await waitFor(() => expect(fetchSymbolCandidates).toHaveBeenCalled())
    const opts = vi.mocked(fetchSymbolCandidates).mock.calls[0][0]!
    expect(opts.listedYears).toBeUndefined()
  })

  it('預設帶 股票,ETF 與 per_industry=9', async () => {
    render(EvaluationUniverse)
    await clickGenerate()

    await waitFor(() => expect(fetchSymbolCandidates).toHaveBeenCalled())
    const opts = vi.mocked(fetchSymbolCandidates).mock.calls[0][0]!
    expect(opts.securityTypes).toEqual(['股票', 'ETF'])
    expect(opts.perIndustry).toBe(9)
  })

  it('顯示筆數與產業分佈', async () => {
    render(EvaluationUniverse)
    await clickGenerate()

    // 用 title 而不是 text：產業名稱同時出現在上方的篩選標籤與這裡的結果分佈長條，
    // 只有長條的 span 帶 title（名稱過長會被 truncate，靠 title 顯示全名）。
    expect(await screen.findByTitle('半導體業')).toBeInTheDocument()
    expect(screen.getByTitle('航運業')).toBeInTheDocument()
  })

  it('0 筆時提示中文值問題，而不是只顯示空白', async () => {
    vi.mocked(fetchSymbolCandidates).mockResolvedValue(
      candidates({ count: 0, symbols: [], by_industry: {} }),
    )
    render(EvaluationUniverse)
    await clickGenerate()

    expect(await screen.findByText(/主檔存的是中文/)).toBeInTheDocument()
  })

  // truncated 代表清單被依代號順序砍掉，會整批砍掉高代號的產業——
  // 正是 per_industry 要消除的偏斜，不能默默拿去回補。
  it('truncated 時顯示警告', async () => {
    vi.mocked(fetchSymbolCandidates).mockResolvedValue(candidates({ truncated: true }))
    render(EvaluationUniverse)
    await clickGenerate()

    expect(await screen.findByText(/截斷依代號順序/)).toBeInTheDocument()
  })

  it('無產業分類的檔數另外標示（多為 ETF）', async () => {
    vi.mocked(fetchSymbolCandidates).mockResolvedValue(
      candidates({ count: 5, by_industry: { 半導體業: 1, '': 4 } }),
    )
    render(EvaluationUniverse)
    await clickGenerate()

    expect(await screen.findByText(/另有 4 檔無產業分類/)).toBeInTheDocument()
  })
})

describe('回補', () => {
  it('「加入下方回補清單」把代號帶進 textarea 並算出預估耗時', async () => {
    render(EvaluationUniverse)
    await clickGenerate()

    await fireEvent.click(await screen.findByRole('button', { name: '加入下方回補清單' }))

    const textarea = screen.getByPlaceholderText('股票代號，逗號分隔') as HTMLTextAreaElement
    expect(textarea.value).toBe('2330,2603')
    // 2 檔 ÷ 5 檔/分 → 無條件進位 1 分鐘
    expect(screen.getByText(/2 檔，預估 約 1 分鐘/)).toBeInTheDocument()
  })

  it('預估耗時超過一小時時以小時顯示', async () => {
    render(EvaluationUniverse)
    const textarea = screen.getByPlaceholderText('股票代號，逗號分隔')
    // 400 檔 ÷ 5 = 80 分鐘 → 1 小時 20 分鐘
    await fireEvent.input(textarea, {
      target: { value: Array.from({ length: 400 }, (_, i) => `A${i}`).join(',') },
    })

    expect(screen.getByText(/約 1 小時 20 分鐘/)).toBeInTheDocument()
  })

  it('送出時帶上代號與天數，預設 130 天', async () => {
    render(EvaluationUniverse)
    await clickGenerate()
    await fireEvent.click(await screen.findByRole('button', { name: '加入下方回補清單' }))
    await fireEvent.click(screen.getByRole('button', { name: '開始回補' }))

    await waitFor(() => expect(triggerBackfill).toHaveBeenCalled())
    expect(vi.mocked(triggerBackfill).mock.calls[0][0]).toEqual({
      days: 130,
      symbols: ['2330', '2603'],
    })
  })

  it('清單為空時停用回補按鈕', () => {
    render(EvaluationUniverse)
    expect(screen.getByRole('button', { name: '開始回補' })).toBeDisabled()
  })

  it('顯示 job 進度與百分比', async () => {
    vi.mocked(triggerBackfill).mockResolvedValue(job({ status: 'running', symbols_done: 1 }))
    render(EvaluationUniverse)
    await clickGenerate()
    await fireEvent.click(await screen.findByRole('button', { name: '加入下方回補清單' }))
    await fireEvent.click(screen.getByRole('button', { name: '開始回補' }))

    expect(await screen.findByText('bf_20260813_000000_000')).toBeInTheDocument()
    expect(await screen.findByText('50%')).toBeInTheDocument()
  })
})

describe('篩選選項來自 API', () => {
  // 使用者不該需要手打「半導體業」——值是 TWSE ISIN 的原始中文分類，
  // 打錯的後果是 HTTP 200 + 0 筆，與「條件真的沒匹配」無法區分。
  it('證券類型與產業都由 facets 產生，且標示母體筆數', async () => {
    render(EvaluationUniverse)

    expect(await screen.findByRole('button', { name: /股票\s*1,945/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /ETF\s*354/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /半導體業\s*201/ })).toBeInTheDocument()
  })

  // 權證要列出來但不預設勾選：全部列出讓使用者看到 31,090 這個數字自己判斷，
  // 用資訊而不是隱藏來防止誤選。
  it('權證有列出但預設不勾選', async () => {
    render(EvaluationUniverse)
    await screen.findByRole('button', { name: /股票\s*1,945/ })

    expect(screen.getByRole('button', { name: /上市認購\(售\)權證\s*31,090/ })).toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: '產生候選清單' }))
    await waitFor(() => expect(fetchSymbolCandidates).toHaveBeenCalled())
    expect(vi.mocked(fetchSymbolCandidates).mock.calls[0][0]!.securityTypes)
      .toEqual(['股票', 'ETF'])
  })

  // 首次載入就必須帶著已選類型：不帶的話產業清單會混入只存在於創新板／特別股的產業，
  // 使用者點了必然 0 筆，卻會看到「主檔存的是中文」這種怪罪他打錯字的訊息。
  it('首次載入就帶著已選類型抓產業清單', async () => {
    render(EvaluationUniverse)
    await waitFor(() => expect(fetchSymbolFacets).toHaveBeenCalled())
    expect(vi.mocked(fetchSymbolFacets).mock.calls[0][0]).toEqual(['股票', 'ETF'])
  })

  it('選取的證券類型會用來縮放產業清單', async () => {
    render(EvaluationUniverse)
    await screen.findByRole('button', { name: /股票\s*1,945/ })

    vi.mocked(fetchSymbolFacets).mockClear()
    await fireEvent.click(screen.getByRole('button', { name: /ETF\s*354/ }))

    // 取消勾選 ETF 後只剩股票，產業清單要跟著重抓
    await waitFor(() => expect(fetchSymbolFacets).toHaveBeenCalledWith(['股票']))
  })

  it('facets 載入失敗時顯示錯誤，不是空白選單', async () => {
    vi.mocked(fetchSymbolFacets).mockRejectedValue(new Error('boom'))
    render(EvaluationUniverse)

    expect(await screen.findByText('載入篩選選項失敗')).toBeInTheDocument()
  })
})

describe('證券類型全部取消勾選', () => {
  // 後端在收不到 security_type 時會套預設的 股票,ETF，而不是「不限」——
  // 與旁邊「產業不選 = 不限」的語意相反。使用者會以為自己在查全市場，
  // 實際拿到 647 檔，然後拿這個母體去算 Step 1 的 ATR% 分佈。
  it('擋在送出前並說明「不選 ≠ 不限」，不送出請求', async () => {
    render(EvaluationUniverse)
    await screen.findByRole('button', { name: /股票\s*1,945/ })

    await fireEvent.click(screen.getByRole('button', { name: /股票\s*1,945/ }))
    await fireEvent.click(screen.getByRole('button', { name: /ETF\s*354/ }))
    await fireEvent.click(screen.getByRole('button', { name: '產生候選清單' }))

    expect(await screen.findByText(/全部不選不等於/)).toBeInTheDocument()
    expect(fetchSymbolCandidates).not.toHaveBeenCalled()
  })
})

// ── ③ 已入池：匯入 selection report ─────────────────────────────
const IMPORT_LOW = 0.046089927430152715
const IMPORT_HIGH = 0.06278197721225691

function reportJson(overrides: Record<string, unknown> = {}): string {
  return JSON.stringify({
    quantile_edges: [IMPORT_LOW, IMPORT_HIGH],
    threshold_gap: { aligned: true, pipeline_absolute: { low_max: IMPORT_LOW, high_min: IMPORT_HIGH } },
    symbols: [
      { symbol: '0050', selection_bucket: 'LOW_VOLATILITY', universe_role: 'primary' },
      { symbol: '2454', selection_bucket: 'HIGH_VOLATILITY', universe_role: 'primary' },
    ],
    universe: { selected_symbols: ['0050', '2454'] },
    ...overrides,
  })
}

async function pasteReport(raw: string) {
  const ta = screen.getByPlaceholderText('或直接貼上報告 JSON')
  await fireEvent.input(ta, { target: { value: raw } })
}

describe('匯入 selection report', () => {
  it('貼上報告後顯示筆數與 bucket 分佈，確認後送出逐檔資料', async () => {
    render(EvaluationUniverse)
    await pasteReport(reportJson())

    await screen.findByText(/將匯入/)
    await fireEvent.click(screen.getByRole('button', { name: '確認匯入 2 檔' }))

    await waitFor(() => expect(upsertEvaluationUniverse).toHaveBeenCalledTimes(1))
    const items = vi.mocked(upsertEvaluationUniverse).mock.calls[0][0]
    expect(items.map((i) => i.symbol)).toEqual(['0050', '2454'])
    // 邊界照抄，不重算也不取整
    expect(items[0].bucket_edge_low).toBe(IMPORT_LOW)
    expect(items[0].bucket_hint).toBe('LOW_VOLATILITY')
    expect(items[1].bucket_hint).toBe('HIGH_VOLATILITY')
    // 匯入成功後要重新載入池，而不是就地拼湊本地狀態
    await waitFor(() => expect(fetchEvaluationUniverse).toHaveBeenCalledTimes(2))
  })

  it('aligned=false 時顯示警告但仍可匯入', async () => {
    render(EvaluationUniverse)
    await pasteReport(reportJson({ threshold_gap: { aligned: false } }))

    await screen.findByText(/bucket_hint 會與 runtime 算出的 bucket 不一致/)
    // 警告不是阻擋：使用者可能就是要記錄一個已知會漂的版本
    expect(screen.getByRole('button', { name: '確認匯入 2 檔' })).toBeTruthy()
  })

  it('不是 selection report 時顯示錯誤且不送出', async () => {
    render(EvaluationUniverse)
    await pasteReport('{"foo":1}')

    await screen.findByText(/selected_symbols/)
    expect(screen.queryByRole('button', { name: /確認匯入/ })).toBeNull()
    expect(upsertEvaluationUniverse).not.toHaveBeenCalled()
  })
})

describe('③ 已入池：維護狀態與手動對齊', () => {
  it('切換維護狀態後重新載入整份，而不是就地改本地狀態', async () => {
    vi.mocked(fetchEvaluationUniverse).mockResolvedValue(pool())
    render(EvaluationUniverse)

    const toggle = await screen.findByRole('button', { name: '維護中' })
    await fireEvent.click(toggle)

    await waitFor(() => expect(setEvaluationUniverseActive).toHaveBeenCalledWith('2330', false))
    // active_count 與 bucket 分佈都由後端算，本地各算一次就會有兩套邏輯
    await waitFor(() => expect(fetchEvaluationUniverse).toHaveBeenCalledTimes(2))
  })

  it('手動對齊的耗時由實際檔數推算，不寫死', async () => {
    vi.mocked(fetchEvaluationUniverse).mockResolvedValue(pool({ active_count: 131 }))
    render(EvaluationUniverse)
    await screen.findByRole('button', { name: '維護中' })

    await fireEvent.click(screen.getByRole('button', { name: '手動對齊日 K' }))

    // 131 檔 ÷ 5 req/min = 27 分鐘（向上取整）。寫死 26 在池變大變小時都會誤導。
    await screen.findByText(/約 27 分鐘（131 檔）/)
  })

  it('池是空的時候不顯示假的耗時', async () => {
    vi.mocked(fetchEvaluationUniverse).mockResolvedValue(
      { items: [], total: 0, active_count: 0, active_buckets: {} },
    )
    render(EvaluationUniverse)
    await fireEvent.click(await screen.findByRole('button', { name: '手動對齊日 K' }))

    // **要對訊息元素本身斷言**，不能用全文件查「約 N 分鐘」——
    // ② 回補區有自己的耗時格式化，輸入為空時就渲染「約 0 分鐘」，
    // 全域查詢會撞到它（第一版就是這樣誤判成程式有問題）。
    const msg = await screen.findByText(/evaluation_universe_sync 已在背景重新觸發/)
    expect(msg.textContent).toContain('進度見排程狀態')
    expect(msg.textContent).not.toContain('約')
  })
})
