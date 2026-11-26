import type { TradeDataStatus, TradePlan } from '../api/tradeAnalysis'

/**
 * T-089 D3：交易計畫摘要卡的狀態推導（集中在這裡，頁面⛔ 不自己判斷）。
 * 規格見 docs/plans/t089-first-usable-trade-plan.md 第三節 D3（主狀態表由上往下，取第一個符合的）。
 *
 * 「最新」只能由倒數第二列的條件產生：行情日期等於庫內最新日 K，而且等於應有的行情日（或等待今日日 K 時等於 settled_date）。
 * ⛔ 不能因為沒有命中其他狀態就變成最新——最後一列是兜底的「行情狀態無法確認」。
 */
export type TradePlanMainState =
  | 'insufficient'        // 資料不足
  | 'unconfirmed'         // 行情狀態無法確認（查詢失敗、日曆未確認、持倉版本讀不到、來源回查失敗）
  | 'no_plan'             // 尚無計畫
  | 'source_unknown'      // 來源無法確認（舊紀錄或非日 K）
  | 'candle_changed'      // 歷史：日 K 已變動（計畫用的那根已不在庫內）
  | 'update_failed'       // 歷史：更新失敗
  | 'position_changed'    // 歷史：持倉已變動
  | 'newer_candle'        // 歷史：有較新的日 K
  | 'stale'               // 行情過期
  | 'history_view'        // 歷史：回看
  | 'latest'              // 最新
  | 'unconfirmed_other'   // 兜底：行情狀態無法確認（其他）

/** 這個鍵（投組＋代號）最近一次分析的結果。 */
export type TradeAnalysisOutcome =
  | { kind: 'ok' }
  | { kind: 'not_found' }
  | { kind: 'failed'; at: string; message?: string }

export interface TradePlanStatusInput {
  /** 摘要卡顯示的那一筆（pickSummaryPlan 或使用者從歷史點選的）；沒有計畫時是 null */
  plan: TradePlan | null
  dataStatus: TradeDataStatus | null
  lastAnalysis?: TradeAnalysisOutcome | null
  /** 使用者從分析歷史點選了非摘要那一筆 */
  viewingHistory?: boolean
  /**
   * 這個鍵有買賣或更正寫入成功，之後還沒有重新載入成功（review 2026-11-26 [P1]）：畫面上的持倉版本與資料狀態都是寫入前的，
   * 比對不出持倉已變動——寫入成功就撤下「最新」，等重新確認持倉與行情後才恢復。
   */
  positionWrittenSinceLoad?: boolean
}

export interface TradePlanStatus {
  main: TradePlanMainState
  /** 主狀態的畫面文字 */
  label: string
  /** 只有 latest 是 true；其他一律當歷史看 */
  current: boolean
  /** 資料不足時⛔ 不顯示判斷 */
  showJudgement: boolean
  /** 可以並存的提示 */
  hints: string[]
}

// 用 formatToParts 自己組字串：locale 的 format() 會在日期與時間之間放不換行空白之類的字元，畫面與測試都不穩定
const partsFormatter = new Intl.DateTimeFormat('en-US', {
  timeZone: 'Asia/Taipei', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
})

function taipeiParts(iso: string): { month: string; day: string; hour: string; minute: string } {
  const out = { month: '', day: '', hour: '', minute: '' }
  for (const p of partsFormatter.formatToParts(new Date(iso))) {
    if (p.type === 'month' || p.type === 'day' || p.type === 'hour' || p.type === 'minute') out[p.type] = p.value
  }
  return out
}

/** 'YYYY-MM-DD' → 'MM-DD' */
export function shortDate(d: string | null | undefined): string {
  if (!d) return '—'
  const s = d.slice(5)
  return s
}

/** 台北的 HH:MM */
export function hhmm(iso: string): string {
  const p = taipeiParts(iso)
  return `${p.hour}:${p.minute}`
}

/** 台北的 MM-DD HH:MM */
export function mdhm(iso: string): string {
  const p = taipeiParts(iso)
  return `${p.month}-${p.day} ${p.hour}:${p.minute}`
}

/** 台北的 MM-DD */
export function md(iso: string): string {
  const p = taipeiParts(iso)
  return `${p.month}-${p.day}`
}

/** 摘要卡用哪一筆：source_confirmed_1d 為 true 的最新一筆；一筆都沒有時才用最新的一筆（主狀態會是「來源無法確認」）。 */
export function pickSummaryPlan(history: TradePlan[]): TradePlan | null {
  const confirmed = history.find((p) => p.source_confirmed_1d)
  if (confirmed) return confirmed
  const first = history[0] ?? null
  return first
}

function unconfirmedReasons(ds: TradeDataStatus): string[] {
  const reasons: string[] = []
  if (ds.latest_candle === 'error') reasons.push('最新日 K 查詢失敗')
  if (ds.calendar === 'unconfirmed') reasons.push('交易日曆或休市設定無法確認')
  if (ds.position_version === null) reasons.push('目前持倉讀取失敗')
  if (ds.source_lookup === 'error') reasons.push('計畫來源回查失敗')
  return reasons
}

function result(main: TradePlanMainState, label: string, hints: string[], showJudgement = true): TradePlanStatus {
  return { main, label, current: main === 'latest', showJudgement, hints }
}

export function tradePlanStatus(input: TradePlanStatusInput): TradePlanStatus {
  const { plan, dataStatus: ds, lastAnalysis, viewingHistory, positionWrittenSinceLoad } = input
  const hints: string[] = []
  if (ds?.awaiting_today) {
    hints.push(`今日日 K 尚未入庫（收盤批次 15:00），目前最新是 ${shortDate(ds.latest_candle_date)}`)
  }

  // 1. 資料不足
  if (lastAnalysis?.kind === 'not_found' || ds?.latest_candle === 'no_data') {
    return result('insufficient', '資料不足：日 K 不夠，無法產生計畫', hints, false)
  }
  // 2. 行情狀態無法確認（沒有 data_status 也算：狀態本身讀不到）
  if (!ds) {
    return result('unconfirmed', '行情狀態無法確認：資料狀態讀取失敗', hints)
  }
  const reasons = unconfirmedReasons(ds)
  if (reasons.length > 0) {
    return result('unconfirmed', `行情狀態無法確認：${reasons.join('、')}`, hints)
  }
  // 3. 尚無計畫
  if (!plan) {
    let label = '尚無計畫'
    if (ds.latest_candle_date && ds.settled_date && ds.latest_candle_date < ds.settled_date) {
      label += `：最新日 K 只到 ${shortDate(ds.latest_candle_date)}（應有 ${shortDate(ds.settled_date)}），分析結果會是過期的行情`
    }
    return result('no_plan', label, hints)
  }
  // 4. 來源無法確認
  if (!plan.source_confirmed_1d || !plan.market_date) {
    return result('source_unknown', '舊紀錄或非日 K：無法確認行情日期', hints)
  }
  const marketDate = plan.market_date
  const latest = ds.latest_candle_date
  // 5. 日 K 已變動（計畫用的那根日 K 已不在庫內）
  if (latest && marketDate > latest) {
    return result('candle_changed', `計畫用的 ${shortDate(marketDate)} 日 K 已不在庫內（最新只到 ${shortDate(latest)}），請重新分析`, hints)
  }
  // 6. 更新失敗
  if (lastAnalysis?.kind === 'failed') {
    return result('update_failed', `更新失敗（${hhmm(lastAnalysis.at)}）：顯示的是 ${shortDate(marketDate)} 的歷史計畫`, hints)
  }
  // 7. 持倉已變動
  if (positionWrittenSinceLoad || plan.position_version !== ds.position_version) {
    return result('position_changed', '持倉已變動（買賣或更正後），請重新分析', hints)
  }
  // 8. 有較新的日 K
  if (latest && marketDate < latest) {
    return result('newer_candle', `用 ${shortDate(marketDate)} 的日 K · 已有 ${shortDate(latest)} 的日 K，請重新分析`, hints)
  }
  // 9. 行情過期（日 K 停更多天）
  if (latest && marketDate === latest && ds.settled_date && marketDate < ds.settled_date) {
    return result('stale', `行情過期：最新日 K 只到 ${shortDate(latest)}（應有 ${shortDate(ds.settled_date)}）`, hints)
  }
  // 10. 歷史：回看
  if (viewingHistory) {
    return result('history_view', `歷史計畫（${md(plan.created_at)} 分析）`, hints)
  }
  // 11. 最新
  if (latest && marketDate === latest &&
      (marketDate === ds.expected_date || (ds.awaiting_today && marketDate === ds.settled_date))) {
    return result('latest', `行情日期 ${shortDate(marketDate)} · 分析於 ${mdhm(plan.created_at)}`, hints)
  }
  // 12. 兜底
  let label = '行情狀態無法確認'
  if (latest && ds.expected_date && latest > ds.expected_date) {
    label += `：庫內日 K ${shortDate(latest)} 晚於應有行情日 ${shortDate(ds.expected_date)}`
  }
  return result('unconfirmed_other', label, hints)
}
