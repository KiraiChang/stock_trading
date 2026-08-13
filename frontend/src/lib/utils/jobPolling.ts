// 背景任務的輪詢：送出後每隔幾秒查一次進度，直到終態或判定停滯。
//
// 抽出來的理由：股價回補、籌碼回補、標的池回補是同一套流程，先前是各寫一份幾乎相同的
// setInterval。三份之後任何一次修正（例如停滯門檻）都會漏掉其中一兩份。
//
// **停滯保護刻意不用固定逾時**：回補在 FinMind rate limit（5 req/min）下，650 檔本來就要
// 跑兩個多小時，任何固定上限都會把正常的長任務誤判成卡住。改成看「進度有沒有推進」——
// 連續 stallMinutes 分鐘 progress 沒變才停止追蹤。
// **停止追蹤不代表任務失敗**：後端可能還在跑（或已重啟），呼叫端的訊息要講清楚。

export type PollSettleReason = 'terminal' | 'stalled' | 'error'

export interface PollUntilTerminalOptions<T> {
  /** 查一次目前狀態。丟出例外會以 reason='error' 收尾。 */
  fetch: () => Promise<T>
  /** 是否已到終態（done / partial / failed）。 */
  isTerminal: (job: T) => boolean
  /** 用來判斷「有沒有推進」的單調數值，通常是 symbols_done。 */
  progressOf: (job: T) => number
  /** 每次成功取得狀態時呼叫，包含終態那一次。 */
  onUpdate: (job: T) => void
  /**
   * 輪詢結束時呼叫一次。reason='error' 時 job 為 null、err 帶原始錯誤。
   * 呼叫端要在這裡解鎖 UI——三種收尾都必須解鎖，否則按鈕會鎖死到重新整理為止。
   */
  onSettled: (reason: PollSettleReason, job: T | null, err?: unknown) => void
  intervalMs?: number
  stallMinutes?: number
}

export const DEFAULT_POLL_INTERVAL_MS = 3000
export const DEFAULT_STALL_MINUTES = 5

/**
 * 開始輪詢，回傳一個停止函式（重複呼叫安全）。
 * 元件在 onDestroy 一定要呼叫它，否則離開頁面後 timer 還在跑。
 */
export function pollUntilTerminal<T>(opts: PollUntilTerminalOptions<T>): () => void {
  const intervalMs = opts.intervalMs ?? DEFAULT_POLL_INTERVAL_MS
  const stallMinutes = opts.stallMinutes ?? DEFAULT_STALL_MINUTES
  const stallTicks = (stallMinutes * 60 * 1000) / intervalMs

  let timer: ReturnType<typeof setInterval> | null = null
  let stalled = 0
  // -1 而不是 0：job 剛建立時 progress 就是 0，用 0 當初始值會讓第一次比對
  // 誤判成「已經沒有推進」，把停滯計數提早一格。
  let lastProgress = -1
  // setInterval 不會等前一次的 async callback 跑完，所以慢的請求可能在後續的 tick
  // 已經收尾之後才回來。沒有這個旗標的話：一次慢回應會在畫面顯示「完成 / 100%」之後
  // 用舊資料把它蓋回「回補中 / 60%」，而輪詢已經停了、再也不會更新；
  // onSettled 也可能被呼叫兩次。
  let stopped = false

  const stop = () => {
    stopped = true
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  timer = setInterval(async () => {
    let job: T
    try {
      job = await opts.fetch()
    } catch (err) {
      if (stopped) return
      stop()
      opts.onSettled('error', null, err)
      return
    }

    // 收尾之後才抵達的回應一律丟掉，不更新畫面也不再收尾一次。
    if (stopped) return

    opts.onUpdate(job)

    if (opts.isTerminal(job)) {
      stop()
      opts.onSettled('terminal', job)
      return
    }

    const progress = opts.progressOf(job)
    stalled = progress === lastProgress ? stalled + 1 : 0
    lastProgress = progress
    if (stalled >= stallTicks) {
      stop()
      opts.onSettled('stalled', job)
    }
  }, intervalMs)

  return stop
}

/** 停止追蹤時的統一文案——三個呼叫端要講同一件事：後端可能還在跑。 */
export function stalledMessage(jobId: string, stallMinutes = DEFAULT_STALL_MINUTES): string {
  return `進度已 ${stallMinutes} 分鐘沒有推進，停止追蹤。後端可能仍在執行（或已重啟），` +
    `可看後端 log 或稍後重新查詢 ${jobId}。`
}
