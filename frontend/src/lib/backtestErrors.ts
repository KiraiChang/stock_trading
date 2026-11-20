// 回測任務的錯誤代碼 → 畫面上的中文（I-137）。
//
// 後端（Go 的 POST /backtest 的 400、以及 backtest_jobs.error）只會給封閉值域的代碼，⛔ 不會有原始例外文字；
// 這裡只認得那些代碼，認不得的一律顯示通用訊息（⛔ 不把原字串顯示出來）。代碼的定義見
// python/backtest/job_errors.py 與 backend/internal/backtest/validate.go。

export const GENERIC_JOB_ERROR = '回測失敗（內部錯誤）'

const FIELD_LABEL: Record<string, string> = { start_date: '起始日期', end_date: '結束日期' }

function fmt(n: string): string {
  return Number(n).toLocaleString('en-US')
}

// 代碼的中文說明；認不得就回 null。
export function describeBacktestCode(code: string | null | undefined): string | null {
  if (!code) return null
  let m: RegExpMatchArray | null
  if ((m = code.match(/^invalid_date:(start_date|end_date)$/))) {
    return `${FIELD_LABEL[m[1]]}不合法（要是存在的日期，格式 YYYY-MM-DD，而且不能晚於 2099-12-31）`
  }
  if (code === 'start_after_end') return '起始日期不能晚於結束日期'
  if ((m = code.match(/^too_many_symbols:([0-9]+):([0-9]+)$/))) {
    return `一次最多回測 ${fmt(m[2])} 檔，這次選了 ${fmt(m[1])} 檔`
  }
  if ((m = code.match(/^backtrader_single_symbol:([0-9]+)$/))) {
    return `backtrader 版一次只能回測一檔，這次選了 ${fmt(m[1])} 檔`
  }
  if ((m = code.match(/^task_too_large:([0-9A-Z]{1,12}|\?):([0-9]+):([0-9]+)$/))) {
    const who = m[1] === '?' ? '' : `${m[1]} `
    return `範圍太大：到 ${who}為止所有股票合計至少 ${fmt(m[2])} 根（含暖身），合計上限 ${fmt(m[3])} 根。請減少檔數或縮短日期範圍`
  }
  if ((m = code.match(/^symbol_too_large:([0-9A-Z]{1,12}|\?):([0-9]+):([0-9]+)$/))) {
    const who = m[1] === '?' ? '其中一檔' : m[1]
    return `範圍太大：${who}至少 ${fmt(m[2])} 根（含暖身），每檔上限 ${fmt(m[3])} 根。請縮短日期範圍`
  }
  if ((m = code.match(/^insufficient_data:([0-9A-Z]{1,12}|\?):([0-9]+):([0-9]+)$/))) {
    const who = m[1] === '?' ? '' : `${m[1]} `
    return `資料不足：${who}取得 ${fmt(m[2])} 根，至少需要 ${fmt(m[3])} 根`
  }
  if (code === 'no_data_in_range') return '所選的股票在這段日期內都沒有資料'
  if (code === 'unknown_strategy') return '不認得這個策略'
  if (code === 'interrupted_on_restart') return '服務重新啟動時中斷，請重新送出'
  if (code === 'cancelled by user') return '已取消'
  return null
}

// 失敗任務的原因：認得的代碼顯示說明，其他（含 internal_error）顯示通用訊息。
export function describeJobFailure(code: string | null | undefined): string {
  return describeBacktestCode(code) ?? GENERIC_JOB_ERROR
}
