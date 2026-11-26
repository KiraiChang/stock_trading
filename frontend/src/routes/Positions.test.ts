import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/svelte'
import { ApiError } from '../lib/api/client'
import type { Position } from '../lib/api/positions'
import type { Portfolio } from '../lib/api/portfolios'
import type { TradeDataStatus, TradePlan, TradePlanHistory } from '../lib/api/tradeAnalysis'

vi.mock('../lib/api/positions', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/positions')>()),
  getPosition: vi.fn(),
  listPositionTransactions: vi.fn(),
  listPositions: vi.fn(),
  addPositionTransaction: vi.fn(),
  adjustPosition: vi.fn(),
}))
vi.mock('../lib/api/portfolios', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/portfolios')>()),
  listPortfolios: vi.fn(),
  createPortfolio: vi.fn(),
}))
vi.mock('../lib/api/tradeAnalysis', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api/tradeAnalysis')>()),
  analyzeTrade: vi.fn(),
  listTradeAnalyses: vi.fn(),
}))

import Positions from './Positions.svelte'
import {
  addPositionTransaction, adjustPosition, getPosition, listPositionTransactions, listPositions,
} from '../lib/api/positions'
import { createPortfolio, listPortfolios } from '../lib/api/portfolios'
import { analyzeTrade, listTradeAnalyses } from '../lib/api/tradeAnalysis'
import { selectedPortfolioID } from '../lib/stores/portfolio'
import { selectedSymbol } from '../lib/stores/market'

// ── 測試資料 ───────────────────────────────────────────────────

function portfolio(id: number, canWrite = true): Portfolio {
  return {
    id, tenant_id: 1, name: `P${id}`, owner_type: 'USER', owner_id: 1, created_by_user_id: 1,
    is_default: false, can_write: canWrite, created_at: '', updated_at: '',
  }
}

const SHARES: Record<string, number> = { '1|2330': 100, '1|2454': 200, '2|2330': 300 }

function position(symbol: string, pid: number, version = 0): Position {
  return { portfolio_id: pid, symbol, shares: SHARES[`${pid}|${symbol}`] ?? 0, avg_cost: 10, realized_pnl: 0, version }
}

function ds(over: Partial<TradeDataStatus> = {}): TradeDataStatus {
  return {
    latest_candle: 'ok', latest_candle_date: '2026-11-25', expected_date: '2026-11-25', settled_date: '2026-11-24',
    awaiting_today: false, calendar: 'confirmed', position_version: 0, source_lookup: 'ok', as_of: '', ...over,
  }
}

function plan(id: number, symbol: string, pid: number, label = '等待'): TradePlan {
  return {
    id, portfolio_id: pid, symbol, position_state: 'FLAT', position_version: 0, shares: 0, avg_cost: 0, realized_pnl: 0,
    analyzed_at: '2026-11-24T16:00:00Z', current_price: 100, sr_zone_analysis_id: 9, action: 'WAIT', action_label: label,
    target_shares: 0, adjustment_shares: 0, adjustment_side: 'NONE', adjustment_amount: 0, entry_price: null,
    stop_loss_price: null, take_profit_price: null, risk_amount: null, expected_reward_amount: null, risk_reward_ratio: null,
    unrealized_pnl: 0, unrealized_pnl_pct: 0, config: {}, reason: [`${symbol} 的理由`], evidence: {},
    trigger_conditions: [`${symbol} 的觀察條件`], invalidation_conditions: [], rule_version: 'position_sr_zone_v1',
    created_at: '2026-11-25T08:02:00Z', market_date: '2026-11-25', source_confirmed_1d: true,
  }
}

// 每個鍵的歷史；測試可以改它模擬分析之後多一筆
let histories: Record<string, TradePlan[]> = {}

function history(symbol: string, pid: number): TradePlanHistory {
  return { analyses: histories[`${pid}|${symbol}`] ?? [], data_status: ds() }
}

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const flush = async () => {
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0))
}

function useDefaults(portfolios: Portfolio[] = [portfolio(1), portfolio(2)]) {
  vi.mocked(listPortfolios).mockResolvedValue(portfolios)
  vi.mocked(listPositions).mockResolvedValue([])
  vi.mocked(getPosition).mockImplementation(async (symbol, pid) => position(symbol, pid))
  vi.mocked(listPositionTransactions).mockResolvedValue([])
  vi.mocked(listTradeAnalyses).mockImplementation(async (symbol, pid) => history(symbol, pid))
}

beforeEach(() => {
  vi.mocked(getPosition).mockReset()
  vi.mocked(listPositionTransactions).mockReset()
  vi.mocked(listPositions).mockReset()
  vi.mocked(addPositionTransaction).mockReset()
  vi.mocked(adjustPosition).mockReset()
  vi.mocked(listPortfolios).mockReset()
  vi.mocked(createPortfolio).mockReset()
  vi.mocked(analyzeTrade).mockReset()
  vi.mocked(listTradeAnalyses).mockReset()
  histories = { '1|2330': [plan(11, '2330', 1)], '1|2454': [plan(21, '2454', 1)], '2|2330': [plan(31, '2330', 2)] }
  selectedSymbol.set('')
  selectedPortfolioID.set(1)
  useDefaults()
})

async function open(symbol = '2330') {
  selectedSymbol.set(symbol)
  const r = render(Positions)
  await flush()
  return r
}

async function switchSymbol(symbol: string) {
  await fireEvent.input(screen.getByTestId('symbol-input'), { target: { value: symbol } })
  await fireEvent.click(screen.getByText('載入'))
  await flush()
}

async function switchPortfolio(pid: number) {
  await fireEvent.change(screen.getByTestId('portfolio-select'), { target: { value: String(pid) } })
  await flush()
}

const shares = () => screen.queryByTestId('position-shares')?.textContent
const summaryText = () => screen.getByTestId('plan-summary').textContent ?? ''
const historyIDs = () => Array.from(screen.getByTestId('plan-history').querySelectorAll('[data-testid^="history-item-"]'))
  .map((el) => el.getAttribute('data-testid'))

// ── 入口 ───────────────────────────────────────────────────────

describe('Positions 入口（T-089 D4）', () => {
  it('有 selectedSymbol 時開頁就載入那一檔；⛔ 不呼叫分析', async () => {
    await open('2330')
    expect(getPosition).toHaveBeenCalledWith('2330', 1)
    expect(listTradeAnalyses).toHaveBeenCalledWith('2330', 1)
    expect(analyzeTrade).not.toHaveBeenCalled()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('latest')
    expect(summaryText()).toContain('2330 的理由')
    expect(summaryText()).toContain('2330 的觀察條件')
  })

  it('判斷「等待」與資料狀態分開顯示，並列出下一個觀察條件（驗收步驟 4 的元件證據）', async () => {
    await open('2330')
    const card = screen.getByTestId('plan-summary')
    const judgement = screen.getByTestId('plan-judgement')
    const status = screen.getByTestId('plan-status')
    expect(judgement.textContent?.trim()).toBe('等待')
    expect(status.getAttribute('data-state')).toBe('latest')
    // 兩個是同一張卡裡各自獨立的元素：判斷⛔ 不寫在資料狀態裡，資料狀態也⛔ 不寫在判斷裡
    expect(card.contains(judgement) && card.contains(status)).toBe(true)
    expect(status.contains(judgement) || judgement.contains(status)).toBe(false)
    expect(status.textContent).not.toContain('等待')
    expect(card.textContent).toContain('下一個觀察條件')
    expect(card.textContent).toContain('2330 的觀察條件')
  })

  it('沒有投組 → 建立 → 自動載入原本那一檔', async () => {
    useDefaults([])
    selectedPortfolioID.set(0)
    await open('2454')
    expect(screen.getByTestId('no-portfolio-hint').textContent).toContain('建立後會開啟 2454 的計畫')
    expect(getPosition).not.toHaveBeenCalled()

    vi.mocked(createPortfolio).mockResolvedValue(portfolio(1))
    vi.mocked(listPortfolios).mockResolvedValue([portfolio(1)])
    await fireEvent.input(screen.getByTestId('new-portfolio-name'), { target: { value: '我的' } })
    await fireEvent.click(screen.getByTestId('create-portfolio'))
    await flush()
    expect(getPosition).toHaveBeenCalledWith('2454', 1)
    expect(summaryText()).toContain('2454 的理由')
  })

  it('分析回 404 → 資料不足、⛔ 不顯示判斷', async () => {
    await open('2330')
    vi.mocked(analyzeTrade).mockRejectedValue(new ApiError(404, 'no candles'))
    await fireEvent.click(screen.getByTestId('analyze'))
    await flush()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('insufficient')
    expect(screen.queryByTestId('plan-judgement')).toBeNull()
  })

  it('latest_candle 是 error → 行情狀態無法確認、⛔ 不是資料不足', async () => {
    vi.mocked(listTradeAnalyses).mockResolvedValue({ analyses: [plan(11, '2330', 1)], data_status: ds({ latest_candle: 'error', latest_candle_date: null }) })
    await open('2330')
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('unconfirmed')
  })

  it('唯讀投組：分析停用並說明，歷史照樣可看', async () => {
    useDefaults([portfolio(1, false)])
    await open('2330')
    expect((screen.getByTestId('analyze') as HTMLButtonElement).disabled).toBe(true)
    expect(historyIDs()).toEqual(['history-item-11'])
  })
})

// ── D5：載入鏈 ────────────────────────────────────────────────

describe('Positions 載入鏈固定選擇（T-089 D5）', () => {
  it('A 的持倉回來前切到 B → A 的流水與歷史仍用 A 的選擇送出，回應⛔ 不套用到畫面', async () => {
    const slow = deferred<Position>()
    vi.mocked(getPosition).mockImplementationOnce(() => slow.promise)
    await open('2330')
    await switchSymbol('2454')
    expect(shares()).toBe('200')
    slow.resolve(position('2330', 1))
    await flush()
    expect(listPositionTransactions).toHaveBeenCalledWith('2330', 1)
    expect(listTradeAnalyses).toHaveBeenCalledWith('2330', 1)
    expect(shares()).toBe('200')
    expect(historyIDs()).toEqual(['history-item-21'])
    expect(summaryText()).toContain('2454 的理由')
    expect(summaryText()).not.toContain('2330 的理由')
  })

  it('同一檔重複載入，較舊的回應晚到 ⛔ 不覆蓋較新的', async () => {
    await open('2330')
    const slow = deferred<TradePlanHistory>()
    vi.mocked(listTradeAnalyses).mockImplementationOnce(() => slow.promise)
    await fireEvent.click(screen.getByText('載入')) // 第一次（慢）
    await flush()
    histories['1|2330'] = [plan(12, '2330', 1), plan(11, '2330', 1)]
    await fireEvent.click(screen.getByText('載入')) // 第二次（快）
    await flush()
    expect(historyIDs()).toEqual(['history-item-12', 'history-item-11'])
    slow.resolve({ analyses: [plan(11, '2330', 1)], data_status: ds() })
    await flush()
    expect(historyIDs()).toEqual(['history-item-12', 'history-item-11'])
  })

  it('切到 B 時，載入中顯示 B、⛔ 不保留 A 的內容', async () => {
    await open('2330')
    const slow = deferred<Position>()
    vi.mocked(getPosition).mockImplementationOnce(() => slow.promise)
    await switchSymbol('2454')
    expect(screen.getByTestId('plan-loading').textContent).toContain('載入 2454')
    expect(shares()).toBeUndefined()
    expect(historyIDs()).toEqual([])
    slow.resolve(position('2454', 1))
    await flush()
    expect(shares()).toBe('200')
  })
})

// ── D5：分析 ──────────────────────────────────────────────────

describe('Positions 分析回應晚到（T-089 D5）', () => {
  it('A 送出分析、切到 B，A 才回成功 → B ⛔ 不變；回到 A 看得到新的一筆', async () => {
    await open('2330')
    const slow = deferred<never>()
    vi.mocked(analyzeTrade).mockImplementationOnce(() => slow.promise)
    await fireEvent.click(screen.getByTestId('analyze'))
    await switchSymbol('2454')
    const before = { text: summaryText(), hist: historyIDs(), calls: vi.mocked(listTradeAnalyses).mock.calls.length }
    histories['1|2330'] = [plan(13, '2330', 1, '進場'), plan(11, '2330', 1)]
    slow.resolve(undefined as never)
    await flush()
    expect(summaryText()).toBe(before.text)
    expect(historyIDs()).toEqual(before.hist)
    expect(vi.mocked(listTradeAnalyses).mock.calls.length).toBe(before.calls) // ⛔ 沒有用 B 重新載入
    expect(screen.queryByTestId('plan-error')).toBeNull()
    await switchSymbol('2330')
    expect(historyIDs()).toEqual(['history-item-13', 'history-item-11'])
  })

  it('A 送出分析、切到 B，A 才回失敗 → B ⛔ 沒有錯誤；回到 A 才顯示「更新失敗」', async () => {
    await open('2330')
    const slow = deferred<never>()
    vi.mocked(analyzeTrade).mockImplementationOnce(() => slow.promise)
    await fireEvent.click(screen.getByTestId('analyze'))
    await switchSymbol('2454')
    slow.reject(new ApiError(500, 'python down'))
    await flush()
    expect(screen.queryByTestId('plan-error')).toBeNull()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('latest')
    await switchSymbol('2330')
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('update_failed')
    expect(screen.getByTestId('plan-error').textContent).toContain('python down')
  })

  it('A 分析中切到 B：B 的「分析」按鈕可以按（⛔ 不被 A 鎖住）', async () => {
    await open('2330')
    vi.mocked(analyzeTrade).mockImplementationOnce(() => new Promise(() => {}))
    await fireEvent.click(screen.getByTestId('analyze'))
    expect((screen.getByTestId('analyze') as HTMLButtonElement).disabled).toBe(true)
    await switchSymbol('2454')
    expect((screen.getByTestId('analyze') as HTMLButtonElement).disabled).toBe(false)
  })
})

// ── D5：寫入 ──────────────────────────────────────────────────

describe('Positions 寫入回應晚到（T-089 D5）', () => {
  it('A 送出買進、切到 B，A 才回成功 → B ⛔ 不變、⛔ 沒有用 B 重新載入；回到 A 看得到新的持倉', async () => {
    await open('2330')
    const slow = deferred<Position>()
    vi.mocked(addPositionTransaction).mockImplementationOnce(() => slow.promise)
    await fireEvent.input(screen.getByTestId('trade-shares'), { target: { value: '1000' } })
    await fireEvent.input(screen.getByTestId('trade-price'), { target: { value: '50' } })
    await fireEvent.click(screen.getByTestId('save-trade'))
    expect(addPositionTransaction).toHaveBeenCalledWith('2330', expect.objectContaining({ portfolio_id: 1, shares: 1000, expected_version: 0 }))
    await switchSymbol('2454')
    await fireEvent.input(screen.getByTestId('adjust-reason'), { target: { value: 'B 填到一半' } })
    const calls = vi.mocked(getPosition).mock.calls.length
    SHARES['1|2330'] = 1100
    slow.resolve(position('2330', 1, 1))
    await flush()
    expect(shares()).toBe('200')
    expect((screen.getByTestId('adjust-reason') as HTMLInputElement).value).toBe('B 填到一半')
    expect(screen.queryByTestId('plan-error')).toBeNull()
    expect(vi.mocked(getPosition).mock.calls.length).toBe(calls)
    await switchSymbol('2330')
    expect(shares()).toBe('1,100')
    SHARES['1|2330'] = 100
  })

  it('A 送出更正、切到 B，A 才回失敗（409）→ B ⛔ 沒有錯誤；回到 A 才顯示更正失敗', async () => {
    await open('2330')
    const slow = deferred<Position>()
    vi.mocked(adjustPosition).mockImplementationOnce(() => slow.promise)
    await fireEvent.input(screen.getByTestId('adjust-reason'), { target: { value: '修正' } })
    await fireEvent.click(screen.getByTestId('save-adjustment'))
    await switchSymbol('2454')
    slow.reject(new ApiError(409, 'version conflict'))
    await flush()
    expect(screen.queryByTestId('plan-error')).toBeNull()
    await switchSymbol('2330')
    expect(screen.getByTestId('plan-error').textContent).toContain('更正失敗：version conflict')
  })

  it('切到 B 時交易與更正的表單是空的（⛔ 不帶 A 填到一半的內容）', async () => {
    await open('2330')
    await fireEvent.input(screen.getByTestId('trade-shares'), { target: { value: '1000' } })
    await fireEvent.input(screen.getByTestId('trade-note'), { target: { value: 'A 的備註' } })
    await fireEvent.input(screen.getByTestId('adjust-reason'), { target: { value: 'A 的原因' } })
    const slow = deferred<Position>()
    vi.mocked(getPosition).mockImplementationOnce(() => slow.promise)
    await switchSymbol('2454')
    expect((screen.getByTestId('trade-shares') as HTMLInputElement).value).toBe('')
    expect((screen.getByTestId('trade-note') as HTMLInputElement).value).toBe('')
    expect((screen.getByTestId('adjust-reason') as HTMLInputElement).value).toBe('')
    slow.resolve(position('2454', 1))
    await flush()
  })
})

// ── D5：切換投組（同一檔）────────────────────────────────────

describe('Positions 切換投組（同一檔）', () => {
  it('載入：投組 1 的持倉回來前切到投組 2 → 投組 1 的回應⛔ 不套用', async () => {
    const slow = deferred<Position>()
    vi.mocked(getPosition).mockImplementationOnce(() => slow.promise)
    await open('2330')
    await switchPortfolio(2)
    expect(shares()).toBe('300')
    slow.resolve(position('2330', 1))
    await flush()
    expect(listTradeAnalyses).toHaveBeenCalledWith('2330', 1)
    expect(shares()).toBe('300')
    expect(historyIDs()).toEqual(['history-item-31'])
  })

  it('分析：投組 1 送出分析、切到投組 2，投組 1 才回失敗 → 投組 2 ⛔ 沒有錯誤；回到投組 1 才顯示更新失敗', async () => {
    await open('2330')
    const slow = deferred<never>()
    vi.mocked(analyzeTrade).mockImplementationOnce(() => slow.promise)
    await fireEvent.click(screen.getByTestId('analyze'))
    expect(analyzeTrade).toHaveBeenCalledWith('2330', 1, false)
    await switchPortfolio(2)
    slow.reject(new ApiError(500, 'python down'))
    await flush()
    expect(screen.queryByTestId('plan-error')).toBeNull()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('latest')
    await switchPortfolio(1)
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('update_failed')
  })

  it('寫入：投組 1 送出買進、切到投組 2，投組 1 才回成功 → 投組 2 ⛔ 不變；回到投組 1 看得到新的持倉', async () => {
    await open('2330')
    const slow = deferred<Position>()
    vi.mocked(addPositionTransaction).mockImplementationOnce(() => slow.promise)
    await fireEvent.input(screen.getByTestId('trade-shares'), { target: { value: '1000' } })
    await fireEvent.input(screen.getByTestId('trade-price'), { target: { value: '50' } })
    await fireEvent.click(screen.getByTestId('save-trade'))
    await switchPortfolio(2)
    const calls = vi.mocked(getPosition).mock.calls.length
    SHARES['1|2330'] = 1100
    slow.resolve(position('2330', 1, 1))
    await flush()
    expect(shares()).toBe('300')
    expect(vi.mocked(getPosition).mock.calls.length).toBe(calls)
    await switchPortfolio(1)
    expect(shares()).toBe('1,100')
    SHARES['1|2330'] = 100
  })
})

// ── review 2026-11-26 ─────────────────────────────────────────

describe('Positions review 2026-11-26', () => {
  it('[P1] 買進已成功、重新載入失敗 → ⛔ 不能仍標「最新」；重新載入成功後才恢復', async () => {
    await open('2330')
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('latest')
    vi.mocked(addPositionTransaction).mockResolvedValueOnce(position('2330', 1, 1))
    vi.mocked(getPosition).mockImplementation(async (symbol, pid) => position(symbol, pid, 1))
    vi.mocked(listTradeAnalyses).mockRejectedValueOnce(new ApiError(500, 'history down'))
    await fireEvent.input(screen.getByTestId('trade-shares'), { target: { value: '1000' } })
    await fireEvent.input(screen.getByTestId('trade-price'), { target: { value: '50' } })
    await fireEvent.click(screen.getByTestId('save-trade'))
    await flush()
    expect(screen.getByTestId('plan-error').textContent).toContain('history down')
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).not.toBe('latest')
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('position_changed')
    // 重新載入成功、而且持倉版本跟計畫一致時才恢復（這裡計畫仍是版本 0、持倉是 1 → 持倉已變動）
    vi.mocked(listTradeAnalyses).mockImplementation(async (symbol, pid) => ({
      analyses: [{ ...plan(14, symbol, pid), position_version: 1 }], data_status: ds({ position_version: 1 }),
    }))
    await fireEvent.click(screen.getByText('載入'))
    await flush()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('latest')
  })

  it('[P1] 更正成功、重新載入失敗 → 同樣⛔ 不是「最新」', async () => {
    await open('2330')
    vi.mocked(adjustPosition).mockResolvedValueOnce(position('2330', 1, 1))
    vi.mocked(getPosition).mockRejectedValueOnce(new ApiError(500, 'position down'))
    await fireEvent.input(screen.getByTestId('adjust-reason'), { target: { value: '修正' } })
    await fireEvent.click(screen.getByTestId('save-adjustment'))
    await flush()
    expect(screen.getByTestId('plan-status').getAttribute('data-state')).toBe('position_changed')
  })

  it('[P2] 投組切換中選了另一檔，持倉清單晚回來 ⛔ 不能跳回原本那一檔', async () => {
    await open('2330')
    const slowList = deferred<Position[]>()
    vi.mocked(listPositions).mockImplementationOnce(() => slowList.promise)
    await switchPortfolio(2)
    await switchSymbol('2454')
    expect(screen.getByTestId('plan-summary').textContent).toContain('2454')
    slowList.resolve([])
    await flush()
    expect(screen.getByTestId('plan-summary').textContent).toContain('2454')
    expect(screen.getByTestId('plan-summary').textContent).not.toContain('2330')
    expect((screen.getByTestId('symbol-input') as HTMLInputElement).value).toBe('2454')
  })

  it('[P2] 投組切換立即確定選擇：⛔ 不等持倉清單就用新投組載入同一檔', async () => {
    await open('2330')
    const slowList = deferred<Position[]>()
    vi.mocked(listPositions).mockImplementationOnce(() => slowList.promise)
    await switchPortfolio(2)
    expect(getPosition).toHaveBeenCalledWith('2330', 2)
    expect(shares()).toBe('300')
    slowList.resolve([])
    await flush()
  })

  it('[P2] 舊投組的持倉清單晚回來 ⛔ 不蓋掉新投組的清單', async () => {
    await open('2330')
    const slowList = deferred<Position[]>()
    vi.mocked(listPositions).mockImplementationOnce(() => slowList.promise)
    await switchPortfolio(2)
    vi.mocked(listPositions).mockResolvedValue([position('2330', 2)])
    await switchPortfolio(1)
    await switchPortfolio(2)
    expect(screen.getByText(/現有 LONG/).textContent).toContain('2330')
    slowList.resolve([position('9999', 1)])
    await flush()
    expect(screen.getByText(/現有 LONG/).textContent).not.toContain('9999')
  })
})

describe('Positions review 2026-11-26 第二輪', () => {
  it('[P3] 切換投組時頁尾「現有 LONG」立即清空，⛔ 不暫時顯示舊投組的股票', async () => {
    vi.mocked(listPositions).mockResolvedValue([position('2454', 1)])
    await open('2330')
    expect(screen.getByText(/現有 LONG/).textContent).toContain('2454')
    const slowList = deferred<Position[]>()
    vi.mocked(listPositions).mockImplementationOnce(() => slowList.promise)
    await switchPortfolio(2)
    expect(shares()).toBe('300') // 新投組的個股持倉已載入
    expect(screen.queryByText(/現有 LONG/)).toBeNull()
    slowList.resolve([position('2330', 2)])
    await flush()
    expect(screen.getByText(/現有 LONG/).textContent).toContain('2330')
    expect(screen.getByText(/現有 LONG/).textContent).not.toContain('2454')
  })
})
