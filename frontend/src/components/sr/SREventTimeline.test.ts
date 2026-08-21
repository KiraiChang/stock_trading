import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import SREventTimeline from './SREventTimeline.svelte'
import {
  getEventTimeline,
  type SREventTimeline as SREventTimelineData,
  type SREventTimelineChain,
} from '../../lib/api/srZones'

vi.mock('../../lib/api/srZones', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/api/srZones')>()
  return {
    ...actual,
    getEventTimeline: vi.fn(),
  }
})

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

function timeline(chains: SREventTimelineChain[]): SREventTimelineData {
  return { symbol: '6182', timeframe: '1d', identity_since: null, chains, snapshots: [] }
}

beforeEach(() => {
  vi.mocked(getEventTimeline).mockReset()
})

describe('SREventTimeline', () => {
  // **標記而不是隱藏**：decision_visible=false 的鏈是事實紀錄，人工判讀
  // 「這個 zone 最近有沒有被測試過」要靠它們。濾掉等於把資訊藏起來。
  it('不參與決策的鏈仍然顯示，只是被標記出來', async () => {
    vi.mocked(getEventTimeline).mockResolvedValue(timeline([
      chain({ event_uid: 'E1', event_family: 'RESISTANCE_BREAKOUT', decision_visible: false }),
      chain({ event_uid: 'E2', event_family: 'SUPPORT_BREAKDOWN' }),
    ]))

    const { getByText, getAllByText, findByText } = render(SREventTimeline, { symbol: '6182' })
    await fireEvent.click(getByText('Event Timeline（跨分析事件鏈）'))

    expect(await findByText('RESISTANCE_BREAKOUT')).toBeInTheDocument()
    expect(getByText('SUPPORT_BREAKDOWN')).toBeInTheDocument()
    expect(getAllByText('事實紀錄・不參與決策')).toHaveLength(1)
  })

  // 缺鍵（舊後端）一律視為決策可見，不能整批被標成事實紀錄。
  it('沒有 decision_visible 欄位時不標記', async () => {
    vi.mocked(getEventTimeline).mockResolvedValue(timeline([chain()]))

    const { getByText, queryByText, findByText } = render(SREventTimeline, { symbol: '6182' })
    await fireEvent.click(getByText('Event Timeline（跨分析事件鏈）'))

    await findByText('SUPPORT_BREAKDOWN')
    expect(queryByText('事實紀錄・不參與決策')).toBeNull()
  })
})
