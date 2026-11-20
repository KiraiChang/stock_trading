import { describe, expect, it } from 'vitest'
import type { IntradayRound, IntradaySymbolRun, IntradaySymbolStatus } from '../api/intraday'
import { describeIntraday, describeRefill, intradayCauseLabel } from './intradayStatus'

// 2026-11-02 09:xx 台北
const at = (hm: string) => `2026-11-02T${hm}:00+08:00`

function row(p: Partial<IntradaySymbolRun>): IntradaySymbolRun {
  return {
    run_id: 1, symbol: '2330', source: 'scheduled', data_at: at('09:15'), data_status: 'written', data_cause: null,
    last_bar_ts: at('09:14'), fetch_attempts: 1, http_recovered: false, eval_at: at('09:15'), eval_status: 'no_signal',
    eval_cause: null, ...p,
  }
}

function status(p: Partial<IntradaySymbolStatus>): IntradaySymbolStatus {
  return {
    symbol: '2330', latest: row({}), refill: null, last_success_eval_at: at('09:10'), last_success_run_id: 1,
    last_bar_ts: at('09:14'), lag_minutes: 1, stale: false, ...p,
  }
}

const skippedRound: IntradayRound = {
  run_id: 9, source: 'scheduled', scheduled_at: at('09:15'), lock_acquired_at: null, exec_finished_at: null,
  lock_wait_ms: 60000, outcome: 'skipped_lock_timeout', evidence_failures: 0, running: false,
}

describe('describeIntraday', () => {
  it('shows 未觸發 only for no_signal', () => {
    expect(describeIntraday(status({}), null).evaluation).toBe('未觸發')
  })

  // 首版最重要的一條：抓取失敗時畫面是「行情更新失敗、本輪未評估」，⛔ 不是「未觸發」。
  it('shows a fetch failure as not evaluated with the cause label', () => {
    const v = describeIntraday(
      status({ latest: row({ data_status: 'fetch_failed', data_cause: 'http_5xx', last_bar_ts: null,
        eval_status: 'not_evaluated', eval_cause: 'fetch_failed' }), last_bar_ts: at('09:10') }),
      null,
    )
    expect(v.data).toBe('09:15 行情更新失敗（伺服器錯誤（5xx））｜最後有效 K 棒 09:10')
    expect(v.evaluation).toBe('本輪未評估（行情更新失敗）')
    expect(v.evaluation).not.toContain('未觸發')
    expect(v.tone).toBe('error')
  })

  it('uses the round record when the latest round was skipped', () => {
    const v = describeIntraday(status({ round_state: 'skipped', last_bar_ts: at('09:10') }), skippedRound)
    expect(v.data).toBe('09:15 本輪未執行（等鎖逾時）｜最後有效 K 棒 09:10')
    expect(v.evaluation).toBe('本輪未評估｜上一次評估 09:10')
  })

  it('shows a newer refill as refilled and not evaluated', () => {
    const v = describeIntraday(status({ refill: row({ source: 'refill', data_at: at('13:40'), eval_status: null }) }), null)
    expect(v.data).toContain('13:40 已補抓')
    expect(v.evaluation).toBe('本次未評估（上一次評估 09:10）')
  })

  // review 2026-11-20：09:10 正常評估、09:15 讀不到自選清單 → 顯示輪次失敗，⛔ 不沿用 09:10 的「未觸發」，保留上一次評估時間。
  it('shows a failed round instead of the previous result', () => {
    const v = describeIntraday(
      status({ round_state: 'failed', last_bar_ts: at('09:10'), last_success_eval_at: at('09:10') }),
      { ...skippedRound, outcome: 'failed' },
    )
    expect(v.data).toBe('09:15 本輪執行失敗｜最後有效 K 棒 09:10')
    expect(v.evaluation).toBe('本輪未評估｜上一次評估 09:10')
    expect(v.evaluation).not.toContain('未觸發')
    expect(describeIntraday(status({ round_state: 'running' }), skippedRound).evaluation).toContain('本輪評估中')
    expect(describeIntraday(status({ round_state: 'incomplete' }), skippedRound).evaluation).toContain('本輪結果未知')
  })

  // review 2026-11-20：補抓失敗或沒有當日 K 棒時⛔ 不能顯示「已補抓」，並保留最後有效 K 棒。
  it('shows the refill result per symbol', () => {
    const failed = describeIntraday(
      status({ refill: row({ source: 'refill', data_at: at('13:40'), data_status: 'fetch_failed', data_cause: 'http_5xx',
        last_bar_ts: null, eval_status: null }), last_bar_ts: at('13:20') }),
      null,
    )
    expect(failed.data).toBe('13:40 補抓失敗（伺服器錯誤（5xx））｜最後有效 K 棒 13:20')
    expect(failed.data).not.toContain('已補抓')
    expect(failed.tone).toBe('error')
    const empty = describeIntraday(
      status({ refill: row({ source: 'refill', data_at: at('13:40'), data_status: 'no_data', eval_status: null }) }),
      null,
    )
    expect(empty.data).toContain('13:40 補抓：沒有當日 K 棒')
  })

  it('separates evaluating from interrupted', () => {
    expect(describeIntraday(status({ latest: row({ eval_status: null }), eval_pending: 'evaluating' }), null).evaluation).toBe('本輪評估中')
    expect(describeIntraday(status({ latest: row({ eval_status: null }), eval_pending: 'interrupted' }), null).evaluation).toBe(
      '本輪評估未完成（中斷）',
    )
  })

  it('labels suppressed, withheld and unchanged results', () => {
    expect(describeIntraday(status({ latest: row({ eval_status: 'signal_suppressed' }) }), null).evaluation).toBe(
      '已觸發（與先前的訊號相同，未重送）',
    )
    expect(describeIntraday(status({ latest: row({ eval_status: 'withheld_stale' }) }), null).evaluation).toBe('資料延遲，未即時送出')
    expect(
      describeIntraday(status({ latest: row({ eval_status: 'not_evaluated', eval_cause: 'unchanged' }) }), null).evaluation,
    ).toBe('本輪未評估（資料沒有變化）')
  })

  it('appends the delay when stale', () => {
    const v = describeIntraday(status({ stale: true, lag_minutes: 15 }), null)
    expect(v.data).toContain('｜行情延遲 15 分鐘')
    expect(v.tone).toBe('warn')
  })

  it('never exposes unknown cause text', () => {
    expect(intradayCauseLabel('something_new')).toBe('其他錯誤')
    expect(intradayCauseLabel(null)).toBe('原因不明')
  })
})

describe('describeRefill', () => {
  it('reports counts and unconfirmed gaps', () => {
    expect(
      describeRefill({ symbol: 'A', status: 'written', inserted: 1, updated: 1, unchanged: 1, last_bar_ts: at('09:03'),
        bars_today: 3, gap_minutes: 3, gaps: ['09:00', '09:04-09:05'], gaps_truncated: false }),
    ).toBe('A：新增 1、更新 1、不變 1｜當日 3 根，最後 09:03｜仍無法確認的缺口 3 分鐘（09:00、09:04-09:05）')
  })

  it('reports deadline as incomplete', () => {
    expect(
      describeRefill({ symbol: 'B', status: 'fetch_failed', cause: 'refill_deadline', inserted: 0, updated: 0, unchanged: 0,
        last_bar_ts: null, bars_today: 0, gap_minutes: 0, gaps: [], gaps_truncated: false }),
    ).toBe('B：未完成（補抓逾時）')
  })
})
