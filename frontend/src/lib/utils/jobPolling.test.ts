import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pollUntilTerminal, stalledMessage } from './jobPolling'

interface FakeJob {
  status: string
  symbols_done: number
}

function fakeJob(status: string, done: number): FakeJob {
  return { status, symbols_done: done }
}

const isTerminal = (j: FakeJob) => j.status === 'done' || j.status === 'failed'
const progressOf = (j: FakeJob) => j.symbols_done

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

// 每個 tick 之間要把 pending 的 promise 沖乾淨——fetch 是 async，
// 只推進 timer 不 flush 的話 onUpdate 還沒被呼叫。
async function tick(times: number, intervalMs = 1000) {
  for (let i = 0; i < times; i++) {
    await vi.advanceTimersByTimeAsync(intervalMs)
  }
}

describe('pollUntilTerminal', () => {
  it('到終態時回報 terminal 並停止輪詢', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fakeJob('running', 1))
      .mockResolvedValueOnce(fakeJob('done', 2))
    const onSettled = vi.fn()
    const onUpdate = vi.fn()

    pollUntilTerminal({ fetch, isTerminal, progressOf, onUpdate, onSettled, intervalMs: 1000 })

    await tick(2)
    expect(onUpdate).toHaveBeenCalledTimes(2)
    expect(onSettled).toHaveBeenCalledWith('terminal', fakeJob('done', 2))

    // 終態之後不該再查
    await tick(3)
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('進度持續推進時不會被判成停滯', async () => {
    let done = 0
    const fetch = vi.fn(async () => fakeJob('running', ++done))
    const onSettled = vi.fn()

    pollUntilTerminal({
      fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled,
      intervalMs: 1000, stallMinutes: 1 / 60 * 3, // 停滯門檻 = 3 ticks
    })

    // 跑 10 輪、每輪都有推進——這正是 650 檔長任務的樣子，不能被誤判
    await tick(10)
    expect(onSettled).not.toHaveBeenCalled()
  })

  it('進度連續沒推進達門檻時回報 stalled', async () => {
    const fetch = vi.fn(async () => fakeJob('running', 7))
    const onSettled = vi.fn()

    pollUntilTerminal({
      fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled,
      intervalMs: 1000, stallMinutes: 3 / 60, // 3 ticks
    })

    await tick(2)
    expect(onSettled).not.toHaveBeenCalled()

    await tick(2)
    expect(onSettled).toHaveBeenCalledWith('stalled', fakeJob('running', 7))
  })

  it('progress 從 0 開始不會讓停滯計數提早一格', async () => {
    // job 剛建立時 symbols_done 就是 0。若初始值用 0 而不是 -1，
    // 第一次比對就會算成「沒有推進」，停滯門檻等於少一個 tick。
    const fetch = vi.fn(async () => fakeJob('running', 0))
    const onSettled = vi.fn()

    pollUntilTerminal({
      fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled,
      intervalMs: 1000, stallMinutes: 3 / 60, // 3 ticks
    })

    await tick(3)
    expect(onSettled).not.toHaveBeenCalled() // 第 3 tick 才累積到 2 次未推進
    await tick(1)
    expect(onSettled).toHaveBeenCalledWith('stalled', fakeJob('running', 0))
  })

  it('fetch 失敗時回報 error 並停止輪詢', async () => {
    const boom = new Error('network down')
    const fetch = vi.fn().mockRejectedValue(boom)
    const onSettled = vi.fn()

    pollUntilTerminal({ fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled, intervalMs: 1000 })

    await tick(1)
    expect(onSettled).toHaveBeenCalledWith('error', null, boom)

    await tick(3)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  // setInterval 不等前一次 callback 跑完，所以慢請求可能在收尾後才回來。
  // 沒有防護的話：畫面會從「完成」被舊資料蓋回「進行中」，而輪詢已經停了、
  // 再也不會更新；onSettled 也會被呼叫兩次。
  it('收尾後才回來的慢回應不會蓋掉畫面，也不會重複收尾', async () => {
    let resolveSlow: ((j: FakeJob) => void) | undefined
    const fetch = vi.fn()
      // 第一次：卡住不回，模擬慢請求
      .mockImplementationOnce(() => new Promise<FakeJob>((r) => { resolveSlow = r }))
      // 第二次：正常回到終態
      .mockResolvedValueOnce(fakeJob('done', 10))
    const onUpdate = vi.fn()
    const onSettled = vi.fn()

    pollUntilTerminal({ fetch, isTerminal, progressOf, onUpdate, onSettled, intervalMs: 1000 })

    await tick(2)
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(onSettled).toHaveBeenCalledWith('terminal', fakeJob('done', 10))
    const updatesAfterTerminal = onUpdate.mock.calls.length

    // 慢請求現在才回來，帶著過時的進度
    resolveSlow?.(fakeJob('running', 3))
    await vi.advanceTimersByTimeAsync(0)

    expect(onUpdate).toHaveBeenCalledTimes(updatesAfterTerminal)
    expect(onSettled).toHaveBeenCalledTimes(1)
  })

  it('stop 之後才失敗的請求不會觸發 error 收尾', async () => {
    let rejectSlow: ((e: unknown) => void) | undefined
    const fetch = vi.fn(() => new Promise<FakeJob>((_, rej) => { rejectSlow = rej }))
    const onSettled = vi.fn()

    const stop = pollUntilTerminal({
      fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled, intervalMs: 1000,
    })

    await tick(1)
    stop()
    rejectSlow?.(new Error('too late'))
    await vi.advanceTimersByTimeAsync(0)

    expect(onSettled).not.toHaveBeenCalled()
  })

  it('回傳的 stop 會停止輪詢，且重複呼叫安全', async () => {
    const fetch = vi.fn(async () => fakeJob('running', 1))
    const stop = pollUntilTerminal({
      fetch, isTerminal, progressOf, onUpdate: () => {}, onSettled: () => {}, intervalMs: 1000,
    })

    await tick(1)
    expect(fetch).toHaveBeenCalledTimes(1)

    stop()
    stop()
    await tick(5)
    expect(fetch).toHaveBeenCalledTimes(1)
  })
})

describe('stalledMessage', () => {
  it('帶上 job_id，讓使用者事後查得到', () => {
    expect(stalledMessage('bf_123')).toContain('bf_123')
    expect(stalledMessage('bf_123')).toContain('後端可能仍在執行')
  })
})
