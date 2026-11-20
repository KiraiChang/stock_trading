// 盤中狀態的輪詢（T-088）。盤中每 60 秒一次；非盤中**仍繼續**、改成每 5 分鐘一次（review 2026-11-20：
// 開盤前開著 Dashboard、開盤後不重開頁面也要能自動恢復 60 秒的刷新）。用 setTimeout 串接，每次依最新的 market_open 決定下一次。

export const INTRADAY_POLL_OPEN_MS = 60_000
export const INTRADAY_POLL_CLOSED_MS = 300_000

export interface IntradayPoller {
  stop(): void
}

// load 回傳這一次是否盤中；失敗時回 null（沿用上一次的判斷）。
export function startIntradayPolling(
  load: () => Promise<boolean | null>,
  opts: { openMs?: number; closedMs?: number } = {},
): IntradayPoller {
  const openMs = opts.openMs ?? INTRADAY_POLL_OPEN_MS
  const closedMs = opts.closedMs ?? INTRADAY_POLL_CLOSED_MS
  let stopped = false
  let timer: ReturnType<typeof setTimeout> | null = null
  let lastOpen = false

  async function tick() {
    if (stopped) return
    const open = await load().catch(() => null)
    if (open !== null) lastOpen = open
    if (stopped) return
    timer = setTimeout(tick, lastOpen ? openMs : closedMs)
  }

  void tick()
  return {
    stop() {
      stopped = true
      if (timer) clearTimeout(timer)
    },
  }
}
