import { describe, expect, it } from 'vitest'

import { chipFreshness } from './chipFreshness'

// 日 K 的 ts 是 16:00Z＝台北隔日 00:00，所以這根的交易日是 08-20。
const BAR_2026_08_20 = '2026-08-19T16:00:00Z'

describe('chipFreshness', () => {
  it('籌碼與 K 棒同一個交易日時標成當日', () => {
    expect(chipFreshness(BAR_2026_08_20, { trade_date: '2026-08-20' }))
      .toEqual({ label: '當日籌碼', stale: false })
  })

  // **這條是整個 helper 的重點**：用 UTC 日期判斷會把 08-20 這根算成 08-19，
  // 於是「當日籌碼」永遠標不出來——而錯誤是靜默的，badge 只是一直顯示舊日期。
  it('基準是台北日曆日而不是 UTC 日期', () => {
    // 這個時間戳的 UTC 日期是 08-19，台北日期是 08-20。
    expect(new Date(BAR_2026_08_20).toISOString().slice(0, 10)).toBe('2026-08-19')
    expect(chipFreshness(BAR_2026_08_20, { trade_date: '2026-08-20' })?.stale).toBe(false)
  })

  it('籌碼落後時把實際日期攤出來，不寫死「前一日」', () => {
    // 落後兩天（連假／採集失敗都可能），寫「前一日」就是說謊。
    expect(chipFreshness(BAR_2026_08_20, { trade_date: '2026-08-18' }))
      .toEqual({ label: '籌碼 08-18', stale: true })
  })

  it('完全沒有籌碼資料時標成無籌碼', () => {
    expect(chipFreshness(BAR_2026_08_20, { missing: true })).toEqual({ label: '無籌碼', stale: true })
    expect(chipFreshness(BAR_2026_08_20, null)).toEqual({ label: '無籌碼', stale: true })
    expect(chipFreshness(BAR_2026_08_20, undefined)).toEqual({ label: '無籌碼', stale: true })
  })

  // 有籌碼但沒有 trade_date（舊分析）：不標，而不是猜一個。
  it('缺 trade_date 時回 null 而不是猜', () => {
    expect(chipFreshness(BAR_2026_08_20, { missing: false })).toBeNull()
    expect(chipFreshness(BAR_2026_08_20, { trade_date: null })).toBeNull()
  })
})
