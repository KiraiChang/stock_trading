import { describe, expect, it } from 'vitest'

import type { SREventTimelineChain } from '../api/srZones'
import { chainEndNote, chainZoneLabel, isDecisionVisible, maxGapDays, splitChains } from './eventTimeline'

function chain(over: Partial<SREventTimelineChain> = {}): SREventTimelineChain {
  return {
    event_uid: 'E1',
    zone_uid: '0f1c9a2e-8b5d-4c31-9a77-2f6e1d4b8c05',
    zone_key: 'SUPPORT:98.0000:100.0000',
    event_scope: 'ZONE',
    event_family: 'SUPPORT_BREAKDOWN',
    seq: 1,
    direction: 'BEARISH',
    root_event_type: 'HIGH_VOLUME_BREAKDOWN',
    latest_event_type: 'HIGH_VOLUME_BREAKDOWN',
    first_seen_at: '2026-08-01T00:00:00Z',
    last_seen_at: '2026-08-05T00:00:00Z',
    closed: false,
    active: true,
    final_state: 'CONFIRMED',
    transitions: [],
    ...over,
  }
}

describe('splitChains', () => {
  it('未終結的與已終結的分開，各自以最近有動作排前', () => {
    const { open, closed } = splitChains([
      chain({ event_uid: 'closedOld', closed: true, last_seen_at: '2026-08-02T00:00:00Z' }),
      chain({ event_uid: 'openOld', last_seen_at: '2026-08-03T00:00:00Z' }),
      chain({ event_uid: 'closedNew', closed: true, last_seen_at: '2026-08-09T00:00:00Z' }),
      chain({ event_uid: 'openNew', last_seen_at: '2026-08-08T00:00:00Z' }),
    ])

    expect(open.map((c) => c.event_uid)).toEqual(['openNew', 'openOld'])
    expect(closed.map((c) => c.event_uid)).toEqual(['closedNew', 'closedOld'])
  })

  it('空輸入不炸', () => {
    expect(splitChains([])).toEqual({ open: [], closed: [] })
  })

  // Go 的 time.Time 在小數秒為 0 時不輸出小數部分，且時區偏移可能與 UTC 混用；
  // 用字串比較會落在標點權重上而排錯。
  it('小數秒與時區偏移混用時仍照真正的時間排序', () => {
    const { open } = splitChains([
      chain({ event_uid: 'noFraction', last_seen_at: '2026-08-05T13:30:00+08:00' }),
      chain({ event_uid: 'withFraction', last_seen_at: '2026-08-05T13:30:00.123456+08:00' }),
      chain({ event_uid: 'utcLater', last_seen_at: '2026-08-05T06:00:00Z' }),
    ])

    // utcLater ＝ 14:00+08:00，比另外兩筆都晚。
    expect(open.map((c) => c.event_uid)).toEqual(['utcLater', 'withFraction', 'noFraction'])
  })

  it('無法解析的時間排到最後，不插隊到最新', () => {
    const { open } = splitChains([
      chain({ event_uid: 'broken', last_seen_at: 'not-a-time' }),
      chain({ event_uid: 'ok', last_seen_at: '2026-08-05T00:00:00Z' }),
    ])

    expect(open.map((c) => c.event_uid)).toEqual(['ok', 'broken'])
  })
})

describe('chainEndNote', () => {
  // **這條是重點**：zone 身分終止不是事件自己走完生命週期，畫成一般結束會讓人以為
  // 「這個事件結束了」，實際上是「這個 zone 不存在了」。
  it('ZONE_IDENTITY_ENDED 要標成需要強調', () => {
    expect(chainEndNote(chain({ closed: true, end_reason: 'ZONE_IDENTITY_ENDED' })))
      .toEqual({ label: 'zone 身分終止', emphasise: true })
  })

  it('自然結束不強調', () => {
    expect(chainEndNote(chain({ closed: true, end_reason: 'RESOLVED' })))
      .toEqual({ label: '已解除', emphasise: false })
    expect(chainEndNote(chain({ closed: true, end_reason: 'EXPIRED' })))
      .toEqual({ label: '已過期', emphasise: false })
  })

  it('未終結的鏈沒有結束註記', () => {
    expect(chainEndNote(chain({ closed: false }))).toBeNull()
  })

  // 終結了卻沒有 end_reason 是資料不完整，照實說而不是猜一個原因。
  it('缺 end_reason 時不猜', () => {
    expect(chainEndNote(chain({ closed: true }))).toEqual({ label: '已終結', emphasise: false })
  })
})

describe('maxGapDays', () => {
  it('取最大的觀測空白', () => {
    expect(maxGapDays([
      { analysis_id: 1, analyzed_at: '2026-08-01T00:00:00Z', gap_days: 0 },
      { analysis_id: 2, analyzed_at: '2026-08-06T00:00:00Z', gap_days: 5 },
      { analysis_id: 3, analyzed_at: '2026-08-07T00:00:00Z', gap_days: 1 },
    ])).toBe(5)
  })

  it('沒有快照時是 0', () => {
    expect(maxGapDays([])).toBe(0)
  })
})

describe('isDecisionVisible', () => {
  // **缺鍵一律 true**：既有事件型別都是決策可見的，舊後端也沒有這個欄位。
  // 當成 false 會讓整批既有事件被標成「不參與決策」，比漏標更嚴重。
  it('缺鍵視為決策可見', () => {
    expect(isDecisionVisible(chain())).toBe(true)
  })

  it('明確為 false 才算事實紀錄', () => {
    expect(isDecisionVisible(chain({ decision_visible: false }))).toBe(false)
    expect(isDecisionVisible(chain({ decision_visible: true }))).toBe(true)
  })
})

describe('chainZoneLabel', () => {
  it('顯示 zone_uid 而不是會漂移的 zone_key', () => {
    const label = chainZoneLabel(chain())
    expect(label).toBe('zone 0f1c9a2e')
    expect(label).not.toContain('SUPPORT')
  })

  it('SYMBOL scope 的事件不屬於任何 zone', () => {
    expect(chainZoneLabel(chain({ event_scope: 'SYMBOL', zone_uid: null }))).toBe('整檔事件')
  })
})
