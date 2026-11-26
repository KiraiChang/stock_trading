import { describe, expect, it } from 'vitest'
import type { TradeDataStatus, TradePlan } from '../api/tradeAnalysis'
import { pickSummaryPlan, tradePlanStatus, type TradeAnalysisOutcome } from './tradePlanStatus'

function ds(over: Partial<TradeDataStatus> = {}): TradeDataStatus {
  return {
    latest_candle: 'ok', latest_candle_date: '2026-11-25',
    expected_date: '2026-11-25', settled_date: '2026-11-24', awaiting_today: false,
    calendar: 'confirmed', position_version: 0, source_lookup: 'ok', as_of: '2026-11-25T08:10:00Z',
    ...over,
  }
}

function plan(over: Partial<TradePlan> = {}): TradePlan {
  return {
    id: 1, portfolio_id: 2, symbol: '2330', position_state: 'FLAT', position_version: 0,
    shares: 0, avg_cost: 0, realized_pnl: 0, analyzed_at: '2026-11-24T16:00:00Z', current_price: 100,
    sr_zone_analysis_id: 9, action: 'WAIT', action_label: '等待', target_shares: 0, adjustment_shares: 0,
    adjustment_side: 'NONE', adjustment_amount: 0, unrealized_pnl: 0, unrealized_pnl_pct: 0,
    reason: ['r1'], rule_version: 'position_sr_zone_v1',
    created_at: '2026-11-25T08:02:00Z', // 台北 16:02
    market_date: '2026-11-25', source_confirmed_1d: true,
    ...over,
  } as TradePlan
}

const failed: TradeAnalysisOutcome = { kind: 'failed', at: '2026-11-25T08:05:00Z' }

describe('tradePlanStatus 主狀態表（由上往下取第一個符合的）', () => {
  it('資料不足：最近一次分析回 404', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds(), lastAnalysis: { kind: 'not_found' } })
    expect(s.main).toBe('insufficient')
    expect(s.showJudgement).toBe(false)
    expect(s.current).toBe(false)
    expect(s.label).toContain('資料不足')
  })
  it('資料不足：latest_candle 是 no_data', () => {
    const s = tradePlanStatus({ plan: null, dataStatus: ds({ latest_candle: 'no_data', latest_candle_date: null }) })
    expect(s.main).toBe('insufficient')
  })
  it.each([
    ['latest_candle 是 error（⛔ 不是資料不足）', { latest_candle: 'error' as const, latest_candle_date: null }, '最新日 K 查詢失敗'],
    ['calendar 是 unconfirmed', { calendar: 'unconfirmed' as const }, '交易日曆或休市設定無法確認'],
    ['position_version 是 null', { position_version: null }, '目前持倉讀取失敗'],
    ['source_lookup 是 error', { source_lookup: 'error' as const }, '計畫來源回查失敗'],
  ])('行情狀態無法確認：%s', (_name, over, reason) => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds(over) })
    expect(s.main).toBe('unconfirmed')
    expect(s.current).toBe(false)
    expect(s.showJudgement).toBe(true) // 有計畫的話照樣顯示、標成歷史
    expect(s.label).toContain(reason)
  })
  it('行情狀態無法確認：data_status 讀不到', () => {
    expect(tradePlanStatus({ plan: plan(), dataStatus: null }).main).toBe('unconfirmed')
  })
  it('尚無計畫', () => {
    const s = tradePlanStatus({ plan: null, dataStatus: ds() })
    expect(s.main).toBe('no_plan')
    expect(s.label).toBe('尚無計畫')
  })
  it('尚無計畫：日 K 停更時帶提示', () => {
    const s = tradePlanStatus({ plan: null, dataStatus: ds({ latest_candle_date: '2026-11-20' }) })
    expect(s.main).toBe('no_plan')
    expect(s.label).toContain('最新日 K 只到 11-20（應有 11-24），分析結果會是過期的行情')
  })
  it('來源無法確認', () => {
    const s = tradePlanStatus({ plan: plan({ source_confirmed_1d: false, market_date: null }), dataStatus: ds() })
    expect(s.main).toBe('source_unknown')
    expect(s.current).toBe(false)
  })
  it('歷史：日 K 已變動（計畫用 11-25、庫內最新退回 11-24、expected 11-25）→ ⛔ 不是最新', () => {
    const s = tradePlanStatus({ plan: plan({ market_date: '2026-11-25' }), dataStatus: ds({ latest_candle_date: '2026-11-24' }) })
    expect(s.main).toBe('candle_changed')
    expect(s.label).toBe('計畫用的 11-25 日 K 已不在庫內（最新只到 11-24），請重新分析')
  })
  it('歷史：更新失敗（帶失敗時間）', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds(), lastAnalysis: failed })
    expect(s.main).toBe('update_failed')
    expect(s.label).toBe('更新失敗（16:05）：顯示的是 11-25 的歷史計畫')
  })
  it('歷史：持倉已變動（同一天空手計畫、之後買進）', () => {
    const s = tradePlanStatus({ plan: plan({ position_version: 0 }), dataStatus: ds({ position_version: 1 }) })
    expect(s.main).toBe('position_changed')
    expect(s.label).toContain('持倉已變動')
  })
  it('歷史：有較新的日 K', () => {
    const s = tradePlanStatus({ plan: plan({ market_date: '2026-11-24' }), dataStatus: ds() })
    expect(s.main).toBe('newer_candle')
    expect(s.label).toBe('用 11-24 的日 K · 已有 11-25 的日 K，請重新分析')
  })
  it('行情過期：日 K 停更多天、計畫日期等於最新入庫日期 → ⛔ 不是最新', () => {
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-20' }),
      dataStatus: ds({ latest_candle_date: '2026-11-20', expected_date: '2026-11-24', settled_date: '2026-11-24' }),
    })
    expect(s.main).toBe('stale')
    expect(s.current).toBe(false)
    expect(s.label).toBe('行情過期：最新日 K 只到 11-20（應有 11-24）')
  })
  it('歷史：回看', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds(), viewingHistory: true })
    expect(s.main).toBe('history_view')
    expect(s.label).toBe('歷史計畫（11-25 分析）')
  })
  it('最新：行情日期等於最新日 K 與應有行情日', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds() })
    expect(s.main).toBe('latest')
    expect(s.current).toBe(true)
    expect(s.label).toBe('行情日期 11-25 · 分析於 11-25 16:02')
  })
  it('最新：等待今日日 K 時，用 settled_date 的計畫仍算最新，另有提示', () => {
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-24' }),
      dataStatus: ds({ latest_candle_date: '2026-11-24', expected_date: '2026-11-25', settled_date: '2026-11-24', awaiting_today: true }),
    })
    expect(s.main).toBe('latest')
    expect(s.hints).toContain('今日日 K 尚未入庫（收盤批次 15:00），目前最新是 11-24')
  })
  it('兜底：庫內日 K 晚於應有行情日 → 無法確認、⛔ 不是最新', () => {
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-26' }),
      dataStatus: ds({ latest_candle_date: '2026-11-26', expected_date: '2026-11-25' }),
    })
    expect(s.main).toBe('unconfirmed_other')
    expect(s.current).toBe(false)
    expect(s.label).toBe('行情狀態無法確認：庫內日 K 11-26 晚於應有行情日 11-25')
  })
  it('兜底：等待今日日 K 但計畫早於 settled_date 以外的情況，⛔ 不會變成最新', () => {
    // 最新日 K 是 settled_date、計畫也是，但 awaiting_today 是 false、expected 是今天（例如資料其實不一致）
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-24' }),
      dataStatus: ds({ latest_candle_date: '2026-11-24', expected_date: '2026-11-25', settled_date: '2026-11-24', awaiting_today: false }),
    })
    expect(s.main).toBe('unconfirmed_other')
  })
})

describe('tradePlanStatus 排在前面的優先', () => {
  it('持倉已變動＋有較新的日 K → 持倉已變動', () => {
    const s = tradePlanStatus({ plan: plan({ market_date: '2026-11-24', position_version: 0 }), dataStatus: ds({ position_version: 2 }) })
    expect(s.main).toBe('position_changed')
  })
  it('更新失敗＋持倉已變動 → 更新失敗', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds({ position_version: 2 }), lastAnalysis: failed })
    expect(s.main).toBe('update_failed')
  })
  it('日 K 已變動＋更新失敗 → 日 K 已變動', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds({ latest_candle_date: '2026-11-24' }), lastAnalysis: failed })
    expect(s.main).toBe('candle_changed')
  })
  it('資料不足優先於行情狀態無法確認', () => {
    const s = tradePlanStatus({ plan: null, dataStatus: ds({ calendar: 'unconfirmed' }), lastAnalysis: { kind: 'not_found' } })
    expect(s.main).toBe('insufficient')
  })
  it('行情狀態無法確認優先於尚無計畫', () => {
    expect(tradePlanStatus({ plan: null, dataStatus: ds({ calendar: 'unconfirmed' }) }).main).toBe('unconfirmed')
  })
  it('來源無法確認優先於日 K 相關的判斷', () => {
    const s = tradePlanStatus({ plan: plan({ source_confirmed_1d: false, market_date: null }), dataStatus: ds({ position_version: 3 }) })
    expect(s.main).toBe('source_unknown')
  })
  it('行情過期優先於回看', () => {
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-20' }), viewingHistory: true,
      dataStatus: ds({ latest_candle_date: '2026-11-20', expected_date: '2026-11-24', settled_date: '2026-11-24' }),
    })
    expect(s.main).toBe('stale')
  })
  it('最近一次分析成功不影響判斷', () => {
    expect(tradePlanStatus({ plan: plan(), dataStatus: ds(), lastAnalysis: { kind: 'ok' } }).main).toBe('latest')
  })
})

describe('tradePlanStatus 寫入後待確認（review 2026-11-26 [P1]）', () => {
  it('寫入成功、還沒重新載入成功 → 持倉已變動（就算畫面上的版本還一致）', () => {
    const s = tradePlanStatus({ plan: plan(), dataStatus: ds(), positionWrittenSinceLoad: true })
    expect(s.main).toBe('position_changed')
    expect(s.current).toBe(false)
  })
  it('沒有待確認的寫入 → 照常判斷', () => {
    expect(tradePlanStatus({ plan: plan(), dataStatus: ds(), positionWrittenSinceLoad: false }).main).toBe('latest')
  })
})

describe('tradePlanStatus 提示', () => {
  it('awaiting_today 的提示可以跟任何主狀態並存', () => {
    const s = tradePlanStatus({
      plan: plan({ market_date: '2026-11-24', position_version: 0 }),
      dataStatus: ds({ latest_candle_date: '2026-11-24', awaiting_today: true, position_version: 1 }),
    })
    expect(s.main).toBe('position_changed')
    expect(s.hints).toEqual(['今日日 K 尚未入庫（收盤批次 15:00），目前最新是 11-24'])
  })
  it('沒有 awaiting_today 時沒有提示', () => {
    expect(tradePlanStatus({ plan: plan(), dataStatus: ds() }).hints).toEqual([])
  })
})

describe('pickSummaryPlan', () => {
  it('取 source_confirmed_1d 為 true 的最新一筆', () => {
    const rows = [plan({ id: 3, source_confirmed_1d: false, market_date: null }), plan({ id: 2 }), plan({ id: 1 })]
    expect(pickSummaryPlan(rows)?.id).toBe(2)
  })
  it('一筆都沒有確認時才用最新的一筆', () => {
    const rows = [plan({ id: 3, source_confirmed_1d: false, market_date: null }), plan({ id: 2, source_confirmed_1d: false, market_date: null })]
    expect(pickSummaryPlan(rows)?.id).toBe(3)
  })
  it('沒有計畫 → null', () => {
    expect(pickSummaryPlan([])).toBeNull()
  })
})
