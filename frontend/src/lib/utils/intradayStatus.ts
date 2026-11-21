import type {
  IntradayRefillSymbol,
  IntradayRound,
  IntradayStatusResponse,
  IntradaySymbolStatus,
  TradingDay,
} from '../api/intraday'

// 盤中狀態 → 畫面文字（T-088＋I-103）。行情與評估分兩段；「未觸發」只在 no_signal 時出現，
// 抓取失敗、沒有新 K 棒、輸入沒變、等鎖逾時都顯示「本輪未評估」與原因，⛔ 不跟「未觸發」混在一起。
// 原因只顯示原因碼對照的中文，⛔ 不顯示原始錯誤。

export type IntradayTone = 'ok' | 'muted' | 'warn' | 'error'

export interface IntradayView {
  data: string
  evaluation: string
  tone: IntradayTone
}

const causeLabels: Record<string, string> = {
  http_5xx: '伺服器錯誤（5xx）',
  http_non_5xx: '請求被拒',
  http_timeout: '請求逾時',
  dns_timeout: 'DNS 逾時',
  dns_temporary: 'DNS 暫時失敗',
  dns_permanent: 'DNS 查詢失敗',
  net_unreachable: '網路不可達',
  host_unreachable: '主機不可達',
  conn_refused: '連線被拒',
  conn_reset: '連線被重設',
  net_timeout: '網路逾時',
  unexpected_eof: '連線中斷',
  db_connection: '資料庫連線失敗',
  retry_budget: '重試時間用完',
  refill_deadline: '補抓逾時',
}

export function intradayCauseLabel(cause: string | null | undefined): string {
  if (!cause) return '原因不明'
  return causeLabels[cause] ?? '其他錯誤'
}

// 台北時間的 HH:MM。
export function hhmm(iso: string | null | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleTimeString('zh-TW', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Taipei',
  })
}

const evalLabels: Record<string, string> = {
  no_signal: '未觸發',
  signal_sent: '已觸發並送出',
  signal_suppressed: '已觸發（與先前的訊號相同，未重送）',
  withheld_stale: '資料延遲，未即時送出',
  degraded: '評估完成，部分階段降級',
  evaluate_failed: '評估失敗',
}

const notEvaluatedLabels: Record<string, string> = {
  fetch_failed: '本輪未評估（行情更新失敗）',
  store_failed: '本輪未評估（行情寫入失敗）',
  no_data: '本輪未評估（沒有新 K 棒）',
  unchanged: '本輪未評估（資料沒有變化）',
}

const closedLabels: Record<string, string> = {
  calendar: '今日休市（日曆）',
  manual: '今日休市（手動設定）',
  weekend: '今日休市（週末）',
}

export function describeIntraday(
  st: IntradaySymbolStatus,
  round: IntradayRound | null,
  tradingDay?: TradingDay | null,
): IntradayView {
  // 休市日（I-139）：⛔ 不顯示上一個交易日的「已更新」。
  if (tradingDay && !tradingDay.is_trading) {
    return { data: closedLabels[tradingDay.source] ?? '今日休市', evaluation: '', tone: 'muted' }
  }
  const lastBar = `最後有效 K 棒 ${hhmm(st.last_bar_ts)}`
  const delay = st.stale && st.lag_minutes != null ? `｜行情延遲 ${st.lag_minutes} 分鐘` : ''

  // 最近一輪比這一檔的逐檔列新：以輪次紀錄為準，⛔ 不把上一輪的評估結果當成這一輪的（保留上一次評估的時間）。
  if (st.round_state && round) {
    const at = hhmm(round.scheduled_at)
    const lastEval = `上一次評估 ${hhmm(st.last_success_eval_at)}`
    switch (st.round_state) {
      case 'skipped':
        return { data: `${at} 本輪未執行（等鎖逾時）｜${lastBar}${delay}`, evaluation: `本輪未評估｜${lastEval}`, tone: 'warn' }
      case 'failed':
        return { data: `${at} 本輪執行失敗｜${lastBar}${delay}`, evaluation: `本輪未評估｜${lastEval}`, tone: 'error' }
      case 'running':
        return { data: `${at} 本輪執行中｜${lastBar}${delay}`, evaluation: `本輪評估中｜${lastEval}`, tone: 'muted' }
      case 'incomplete':
        return { data: `${at} 本輪紀錄不完整｜${lastBar}${delay}`, evaluation: `本輪結果未知｜${lastEval}`, tone: 'warn' }
    }
  }
  // 手動補抓比排程新：只寫了 K 棒、⛔ 不評估。**依補抓的逐檔結果顯示**（review 2026-11-20）：失敗或沒有當日 K 棒時⛔ 不能說「已補抓」。
  if (st.refill) {
    const at = hhmm(st.refill.data_at)
    let data: string
    let tone: IntradayTone = 'muted'
    switch (st.refill.data_status) {
      case 'written':
        data = `${at} 已補抓｜${lastBar}${delay}`
        break
      case 'no_data':
        data = `${at} 補抓：沒有當日 K 棒｜${lastBar}${delay}`
        tone = 'warn'
        break
      default:
        data = `${at} 補抓失敗（${intradayCauseLabel(st.refill.data_cause)}）｜${lastBar}${delay}`
        tone = 'error'
    }
    return { data, evaluation: `本次未評估（上一次評估 ${hhmm(st.last_success_eval_at)}）`, tone }
  }
  const row = st.latest
  if (!row) {
    return { data: '盤中尚無紀錄', evaluation: '', tone: 'muted' }
  }

  let data: string
  let tone: IntradayTone = 'ok'
  switch (row.data_status) {
    case 'written':
      data = `${hhmm(row.data_at)} 已更新｜最後 K 棒 ${hhmm(row.last_bar_ts)}`
      break
    case 'no_data':
      data = `${hhmm(row.data_at)} 本輪未取得新 K 棒｜${lastBar}`
      tone = 'muted'
      break
    default:
      data = `${hhmm(row.data_at)} 行情更新失敗（${intradayCauseLabel(row.data_cause)}）｜${lastBar}`
      tone = 'error'
  }
  if (delay) {
    data += delay
    if (tone === 'ok') tone = 'warn'
  }

  let evaluation: string
  if (!row.eval_status) {
    evaluation = st.eval_pending === 'evaluating' ? '本輪評估中' : '本輪評估未完成（中斷）'
    if (tone === 'ok') tone = 'muted'
  } else if (row.eval_status === 'not_evaluated') {
    evaluation = notEvaluatedLabels[row.eval_cause ?? ''] ?? '本輪未評估'
  } else {
    evaluation = evalLabels[row.eval_status] ?? row.eval_status
    if (row.eval_status === 'evaluate_failed') tone = 'error'
    else if ((row.eval_status === 'withheld_stale' || row.eval_status === 'degraded') && tone === 'ok') tone = 'warn'
  }
  return { data, evaluation, tone }
}

// 手動補抓的一檔結果 → 一行文字。最後一根與根數只代表寫入進度，缺口標「仍無法確認」。
export function describeRefill(r: IntradayRefillSymbol): string {
  if (r.status === 'fetch_failed' || r.status === 'store_failed') {
    const what = r.cause === 'refill_deadline' ? '未完成' : r.status === 'fetch_failed' ? '抓取失敗' : '寫入失敗'
    return `${r.symbol}：${what}（${intradayCauseLabel(r.cause)}）`
  }
  if (r.status === 'no_data') {
    return `${r.symbol}：沒有當日 K 棒`
  }
  let text = `${r.symbol}：新增 ${r.inserted}、更新 ${r.updated}、不變 ${r.unchanged}｜當日 ${r.bars_today} 根，最後 ${hhmm(r.last_bar_ts)}`
  if (r.gap_minutes > 0) {
    const list = r.gaps.join('、') + (r.gaps_truncated ? '…' : '')
    text += `｜仍無法確認的缺口 ${r.gap_minutes} 分鐘（${list}）`
  }
  return text
}

export interface IntradayNotice {
  text: string
  tone: IntradayTone
  key: 'calendar_fallback' | 'none_yet' | 'empty_watchlist' | 'unavailable'
}

// 清單標題列的提示（I-139）：日曆取不到、還沒收到今日盤中資料、自選清單是空的、讀不到盤中資料狀態。
// 抓取或寫入失敗（fetch_failing／store_failing）⛔ 不另出提示：沿用每一列的 T-088 失敗顯示，避免同一個提示同時代表資料源故障與休市。
export function intradayNotices(resp: IntradayStatusResponse | null): IntradayNotice[] {
  if (!resp) return []
  const out: IntradayNotice[] = []
  if (resp.trading_day?.source === 'fallback') {
    out.push({ key: 'calendar_fallback', text: '交易日曆取不到，照平日執行', tone: 'warn' })
  }
  switch (resp.session_data) {
    case 'none_yet':
      out.push({
        key: 'none_yet',
        text: `截至 ${hhmm(resp.now)} 尚未收到今日盤中資料，請核對行情來源與交易所公告`,
        tone: 'warn',
      })
      break
    case 'empty_watchlist':
      out.push({ key: 'empty_watchlist', text: '自選清單是空的', tone: 'muted' })
      break
    case 'unavailable':
      out.push({ key: 'unavailable', text: '讀不到盤中資料狀態', tone: 'error' })
      break
  }
  return out
}
