import type { SREventTimelineChain, SREventTimelineSnapshot } from '../api/srZones'

/**
 * Event Timeline 的判讀輔助（docs/api-reference.md「GET /sr-zones/event-timeline」）。
 *
 * 這裡只放**語意判斷**，不放樣式——那些規則錯了會靜默誤導，值得單獨測。
 */

/** 未終結的鏈排前面：timeline 打開時最該先看到「現在還活著的」。 */
export function splitChains(chains: SREventTimelineChain[]): {
  open: SREventTimelineChain[]
  closed: SREventTimelineChain[]
} {
  const open: SREventTimelineChain[] = []
  const closed: SREventTimelineChain[] = []
  for (const c of chains) (c.closed ? closed : open).push(c)
  // 未終結的以「最近有動作」排序，已終結的以「最近結束」排序。
  //
  // **比的是時間戳不是字串**：Go 的 `time.Time` 在小數秒為 0 時不輸出小數部分，
  // 字串比較 `…:00.123456+08:00` 與 `…:00+08:00` 會落在 `.` 對 `+` 的標點權重上；
  // 一端是 UTC、另一端帶 +08:00 時更是直接錯。無法解析的值排到最後而不是插隊。
  const at = (ts: string): number => {
    const t = Date.parse(ts)
    return Number.isNaN(t) ? Number.NEGATIVE_INFINITY : t
  }
  const byLastSeenDesc = (a: SREventTimelineChain, b: SREventTimelineChain) =>
    at(b.last_seen_at) - at(a.last_seen_at)
  return { open: open.sort(byLastSeenDesc), closed: closed.sort(byLastSeenDesc) }
}

export interface ChainEndNote {
  label: string
  /** true 代表這不是事件自己走完生命週期，畫成一般結束會誤導。 */
  emphasise: boolean
}

/**
 * 鏈為什麼結束。
 *
 * **`ZONE_IDENTITY_ENDED` 要跟其他兩種分開**：那是 zone 因 SPLIT／MERGE／RESHAPE
 * 身分終止、鏈跟著收攤，不是事件自己走完 RESOLVED／EXPIRED。混在一起看會以為
 * 「這個事件結束了」，實際上是「這個 zone 不存在了」。
 */
export function chainEndNote(chain: SREventTimelineChain): ChainEndNote | null {
  if (!chain.closed) return null
  switch (chain.end_reason) {
    case 'ZONE_IDENTITY_ENDED':
      return { label: 'zone 身分終止', emphasise: true }
    case 'RESOLVED':
      return { label: '已解除', emphasise: false }
    case 'EXPIRED':
      return { label: '已過期', emphasise: false }
    default:
      // 終結了但沒有 end_reason：資料不完整，照實說而不是猜一個。
      return { label: chain.end_reason || '已終結', emphasise: false }
  }
}

/**
 * 觀測空白：`snapshots` 裡最大的 gap。
 *
 * **timeline 的解析度等於 SR 分析的執行頻率**，所以鏈上的空白不代表那段期間沒有事件，
 * 只代表那段期間沒有分析。少了這個提示，空白會被讀成「風平浪靜」。
 */
export function maxGapDays(snapshots: SREventTimelineSnapshot[]): number {
  return snapshots.reduce((max, s) => (s.gap_days > max ? s.gap_days : max), 0)
}

/**
 * 這條鏈會不會影響決策。
 *
 * **缺鍵一律 true**：既有事件型別都是決策可見的，舊後端也沒有這個欄位；當成 false
 * 會讓整批既有事件被標成「不參與決策」，那比漏標更嚴重。
 *
 * false 的鏈**照樣要顯示**——它們是事實紀錄，人工判讀「這個 zone 最近有沒有被測試過」
 * 要靠它們。這個函式回答的是「要不要標記」，不是「要不要過濾」。
 */
export function isDecisionVisible(chain: SREventTimelineChain): boolean {
  return chain.decision_visible !== false
}

/**
 * 鏈的身分標籤。
 *
 * 顯示 `zone_uid` 的前 8 碼——**它才是鏈的身分**；`zone_key` 只是最近一次觀測到時
 * 事件帶的 key，會隨 ATR 重算漂移，不能拿來當識別。SYMBOL scope 的事件沒有 zone。
 */
export function chainZoneLabel(chain: SREventTimelineChain): string {
  if (chain.event_scope === 'SYMBOL' || !chain.zone_uid) return '整檔事件'
  return `zone ${chain.zone_uid.slice(0, 8)}`
}
