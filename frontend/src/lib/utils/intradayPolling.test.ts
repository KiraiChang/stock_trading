import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { startIntradayPolling } from './intradayPolling'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

const flush = () => vi.advanceTimersByTimeAsync(0)

describe('startIntradayPolling', () => {
  // 跨開盤：開盤前開著頁面（非盤中、每 5 分鐘），開盤後不重開頁面也恢復每 60 秒。
  it('keeps polling while closed and speeds up after the market opens', async () => {
    const answers = [false, false, true, true, true]
    let calls = 0
    const poller = startIntradayPolling(async () => answers[Math.min(calls++, answers.length - 1)])
    await flush()
    expect(calls).toBe(1) // 08:55 載入：非盤中

    await vi.advanceTimersByTimeAsync(60_000)
    expect(calls).toBe(1) // 非盤中⛔ 不是每 60 秒
    await vi.advanceTimersByTimeAsync(240_000)
    expect(calls).toBe(2) // 5 分鐘後再抓一次：仍非盤中
    await vi.advanceTimersByTimeAsync(300_000)
    expect(calls).toBe(3) // 開盤了
    await vi.advanceTimersByTimeAsync(60_000)
    expect(calls).toBe(4) // 之後每 60 秒
    await vi.advanceTimersByTimeAsync(60_000)
    expect(calls).toBe(5)
    poller.stop()
    await vi.advanceTimersByTimeAsync(600_000)
    expect(calls).toBe(5)
  })

  it('keeps the previous cadence when a request fails', async () => {
    const answers: (boolean | null)[] = [true, null, true]
    let calls = 0
    const poller = startIntradayPolling(async () => answers[Math.min(calls++, answers.length - 1)])
    await flush()
    await vi.advanceTimersByTimeAsync(60_000)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(calls).toBe(3) // 失敗那次沿用盤中的 60 秒
    poller.stop()
  })
})
