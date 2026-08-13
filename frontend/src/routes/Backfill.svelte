<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import {
    triggerBackfill,
    getBackfillJob,
    type MarketBackfillJob,
    type MarketBackfillStatus,
  } from '../lib/api/market'
  import { computeIndicators, type IndicatorSnapshot } from '../lib/api/indicators'
  import { evaluateSignal, type EvaluateResult } from '../lib/api/signals'
  import { triggerChipSync, getChipSyncJob, type ChipSyncJob, type ChipSyncStatus } from '../lib/api/chips'
  import { ApiError } from '../lib/api/client'
  import { todayStr, daysAgo } from '../lib/utils/date'
  import { pollUntilTerminal, stalledMessage } from '../lib/utils/jobPolling'
  import type { WatchlistItem } from '../lib/stores/market'

  // items（監控清單）留著給兩塊回補 UI 當「留空 ＝ 整個監控清單」的 fallback。
  // 勾選清單已移除：回補不再侷限於 watchlist，改由代號輸入框指定（見 todo.md T-040）。
  let items: WatchlistItem[] = []
  let symbolsInput = ''
  let days = 120
  let loading = true
  let submitting = false
  let error = ''
  let backfillJob: MarketBackfillJob | null = null
  let backfillPollTimer: (() => void) | null = null

  // ── 手動計算指標：不限監控清單，任意股票代號都可以 ──────────
  let computeSymbol = ''
  let computeTimeframe = '1d'
  let computing = false
  let computeError = ''
  let computeResult: IndicatorSnapshot | null = null

  // ── 手動評估訊號：不限監控清單，任意股票代號都可以 ──────────
  let evalSymbol = ''
  let evalTimeframe = '1d'
  let evaluating = false
  let evalError = ''
  let evalResult: EvaluateResult | null = null

  // ── 籌碼資料回補：留空股票代號 = 整個監控清單，有填 = 只回補填入的股票。
  // 與上方股價回補現在是同一套 UX——兩支 API 都要求明確帶 symbols，
  // 「留空 ＝ 監控清單」一律由前端解析後填入。──────────────────────
  let chipSymbols = ''
  let chipDays = 500
  let chipSubmitting = false
  let chipError = ''
  let chipJob: ChipSyncJob | null = null
  let chipPollTimer: (() => void) | null = null

  // 輪詢與停滯判定已抽到 lib/utils/jobPolling.ts（三個頁面共用，見該檔說明）。
  // 股價回補與籌碼回補的 job 狀態是同一組值，共用一份對應表。
  const chipSyncStatusText: Record<ChipSyncStatus, string> = {
    pending: '排隊中', running: '同步中', done: '完成', partial: '部分成功', failed: '失敗',
  }
  const chipSyncStatusClass: Record<ChipSyncStatus, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    partial: 'bg-yellow-900/40 text-yellow-400',
    failed: 'bg-red-900/40 text-red-400',
  }
  const backfillStatusText: Record<MarketBackfillStatus, string> = chipSyncStatusText
  const backfillStatusClass: Record<MarketBackfillStatus, string> = chipSyncStatusClass

  onMount(load)
  onDestroy(() => {
    stopChipPolling()
    stopPolling()
  })

  async function load() {
    loading = true
    error = ''
    try {
      items = await fetchWatchlist()
    } catch {
      error = '載入監控清單失敗'
    } finally {
      loading = false
    }
  }

  function parseSymbols(raw: string): string[] | null {
    const list = raw
      .split(/[,\s]+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
    return list.length > 0 ? list : null
  }

  async function submit() {
    // 留空 ＝ 整個監控清單，由前端在這裡填入。API 層的 symbols 是必填、
    // 不認識 watchlist（與 /chips/sync 一致）。
    const symbols = parseSymbols(symbolsInput) ?? items.map((i) => i.symbol)
    if (symbols.length === 0) {
      error = '沒有可回補的股票：請先至 Dashboard 新增監控股票，或自行輸入股票代號'
      return
    }
    if (!Number.isFinite(days) || days <= 0) { error = '回補天數需大於 0'; return }

    submitting = true
    error = ''
    backfillJob = null
    try {
      backfillJob = await triggerBackfill({ days, symbols })
      pollJob(backfillJob.job_id)
    } catch (err) {
      error = err instanceof ApiError ? err.message : '回補請求送出失敗，請稍後再試'
      submitting = false
    }
  }

  function pollJob(jobId: string) {
    stopPolling()
    backfillPollTimer = pollUntilTerminal<MarketBackfillJob>({
      fetch: () => getBackfillJob(jobId),
      isTerminal: (j) => j.status === 'done' || j.status === 'partial' || j.status === 'failed',
      progressOf: (j) => j.symbols_done,
      onUpdate: (j) => { backfillJob = j },
      onSettled: (reason) => {
        submitting = false
        if (reason === 'stalled') error = stalledMessage(jobId)
        if (reason === 'error') error = '查詢回補狀態失敗'
      },
    })
  }

  function stopPolling() {
    backfillPollTimer?.()
    backfillPollTimer = null
  }

  async function submitCompute() {
    if (!computeSymbol.trim()) {
      computeError = '請輸入股票代號'
      return
    }
    computing = true
    computeError = ''
    computeResult = null
    try {
      computeResult = await computeIndicators(computeSymbol.trim(), computeTimeframe)
    } catch {
      computeError = '計算失敗，最常見原因是 candles 不足 35 根（請先確認已 backfill 足夠天數）'
    } finally {
      computing = false
    }
  }

  async function submitEvaluate() {
    if (!evalSymbol.trim()) {
      evalError = '請輸入股票代號'
      return
    }
    evaluating = true
    evalError = ''
    evalResult = null
    try {
      evalResult = await evaluateSignal(evalSymbol.trim(), evalTimeframe)
    } catch {
      evalError = '評估失敗，最常見原因是 candles 不足 35 根（請先確認已 backfill 足夠天數）'
    } finally {
      evaluating = false
    }
  }

  // 解析使用者填入的股票代號（逗號分隔）；留空時回傳 null，呼叫端據此
  // fallback 為整個監控清單。
  function parseChipSymbols(): string[] | null {
    const list = chipSymbols
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
    return list.length > 0 ? list : null
  }

  async function submitChipBackfill() {
    const symbols = parseChipSymbols() ?? items.map((i) => i.symbol)
    if (symbols.length === 0) {
      chipError = '沒有可回補的股票：請先至 Dashboard 新增監控股票，或自行輸入股票代號'
      return
    }
    if (!Number.isFinite(chipDays) || chipDays <= 0) {
      chipError = '回補天數需大於 0'
      return
    }

    chipSubmitting = true
    chipError = ''
    chipJob = null
    try {
      const job = await triggerChipSync({
        mode: 'backfill',
        symbols,
        from: daysAgo(chipDays),
        to: todayStr(),
        dataTypes: ['institutional', 'margin', 'broker', 'scores'],
      })
      chipJob = job
      pollChipJob(job.job_id)
    } catch (err) {
      chipError = err instanceof ApiError ? err.message : '回補請求送出失敗，請稍後再試'
      chipSubmitting = false
    }
  }

  function pollChipJob(jobId: string) {
    stopChipPolling()
    chipPollTimer = pollUntilTerminal<ChipSyncJob>({
      fetch: () => getChipSyncJob(jobId),
      isTerminal: (j) => j.status === 'done' || j.status === 'partial' || j.status === 'failed',
      progressOf: (j) => j.symbols_done,
      onUpdate: (j) => { chipJob = j },
      onSettled: (reason) => {
        chipSubmitting = false
        if (reason === 'stalled') chipError = stalledMessage(jobId)
        if (reason === 'error') chipError = '查詢回補狀態失敗'
      },
    })
  }

  function stopChipPolling() {
    chipPollTimer?.()
    chipPollTimer = null
  }
</script>

<Layout>
  <div class="max-w-2xl mx-auto">
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-white font-semibold">歷史資料回補</h1>
      <button
        class="text-xs text-muted hover:text-white px-3 py-1.5 border border-border rounded-lg transition-colors"
        on:click={load}
      >
        重新整理
      </button>
    </div>

    {#if error}
      <p class="text-rise text-sm mb-4">{error}</p>
    {/if}

    <div class="bg-panel border border-border rounded-xl mb-4">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">股價資料回補</h2>
        <p class="text-muted text-xs mt-1">
          回補歷史日K。股票代號留空 = 整個監控清單；有填 = 只回補填入的股票，
          <strong class="text-white">不限於監控清單</strong>（可直接補尚未加入監控的標的）。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        <div class="flex gap-3 flex-wrap">
          <input
            bind:value={symbolsInput}
            placeholder="股票代號，逗號分隔（留空 = 整個監控清單）"
            class="flex-1 min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <input
            type="number"
            min="1"
            step="10"
            bind:value={days}
            title="回補天數"
            class="w-28 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors"
            disabled={submitting || loading}
            on:click={submit}
          >
            {submitting ? '回補中...' : '開始回補'}
          </button>
        </div>

        {#if backfillJob}
          <div class="px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
            <span class="text-muted font-mono">{backfillJob.job_id}</span>
            <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {backfillStatusClass[backfillJob.status]}">
              {backfillStatusText[backfillJob.status]}
            </span>
            <span class="text-muted">
              {backfillJob.symbols_done}/{backfillJob.symbols_total} 檔，失敗 {backfillJob.symbols_failed}
            </span>
            {#if backfillJob.status === 'failed' || backfillJob.status === 'partial'}
              {#each backfillJob.failures as f}
                <span class="text-rise block w-full">{f.symbol}: {f.error}</span>
              {/each}
            {/if}
          </div>
        {/if}

        <p class="text-muted text-xs">
          回補會在後端背景執行（呼叫 POST /api/v1/market/backfill），此頁面每 3 秒輪詢一次進度；
          離開頁面不影響後端繼續執行。受 FinMind rate limit 節流，檔數多時請耐心等候。
        </p>
      </div>
    </div>

    <!-- ── 籌碼資料回補 ──────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl mt-6">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">籌碼資料回補</h2>
        <p class="text-muted text-xs mt-1">
          回補三大法人、融資融券、券商分點資料並重新計算籌碼分數（分點資料若來源不支援會自動略過，不影響其他分數）。
          股票代號留空 = 整個監控清單；有填 = 只回補填入的股票。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        {#if chipError}
          <p class="text-rise text-sm">{chipError}</p>
        {/if}

        <div class="flex gap-3 flex-wrap">
          <input
            bind:value={chipSymbols}
            placeholder="股票代號，逗號分隔（留空 = 整個監控清單）"
            class="flex-1 min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <input
            type="number"
            min="1"
            step="10"
            bind:value={chipDays}
            title="回補天數"
            class="w-28 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors"
            disabled={chipSubmitting}
            on:click={submitChipBackfill}
          >
            {chipSubmitting ? '回補中...' : '開始回補'}
          </button>
        </div>

        {#if chipJob}
          <div class="px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
            <span class="text-muted font-mono">{chipJob.job_id}</span>
            <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {chipSyncStatusClass[chipJob.status]}">
              {chipSyncStatusText[chipJob.status]}
            </span>
            <span class="text-muted">
              {chipJob.symbols_done}/{chipJob.symbols_total} 檔，失敗 {chipJob.symbols_failed}
            </span>
            {#if chipJob.status === 'failed' || chipJob.status === 'partial'}
              {#each chipJob.failures as f}
                <span class="text-rise block w-full">{f.symbol}: {f.error}</span>
              {/each}
            {/if}
          </div>
        {/if}

        <p class="text-muted text-xs">
          回補會在後端背景執行（呼叫 POST /api/v1/chips/sync），此頁面每 3 秒輪詢一次進度；
          離開頁面不影響後端繼續執行，之後可到「籌碼分析」頁面用 job_id 或直接查詢個股確認結果。
        </p>
      </div>
    </div>

    <!-- ── 手動計算指標 ──────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl mt-6">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">手動計算指標</h2>
        <p class="text-muted text-xs mt-1">
          不限監控清單，任意股票代號只要 candles 有 ≥35 根就能立即計算並寫入，
          適合用來補算「剛 backfill 完、但還沒被排程算過指標」的股票。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        {#if computeError}
          <p class="text-rise text-sm">{computeError}</p>
        {/if}

        <div class="flex gap-3">
          <input
            bind:value={computeSymbol}
            placeholder="股票代號，例如 00981A"
            on:keydown={(e) => e.key === 'Enter' && submitCompute()}
            class="flex-1 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <select
            bind:value={computeTimeframe}
            class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          >
            <option value="1d">1d</option>
            <option value="1m">1m</option>
          </select>
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors"
            disabled={computing}
            on:click={submitCompute}
          >
            {computing ? '計算中...' : '計算'}
          </button>
        </div>

        {#if computeResult}
          <div class="grid grid-cols-3 sm:grid-cols-5 gap-3 text-xs pt-2">
            <div><p class="text-muted mb-1">MA5</p><p class="text-white font-mono">{computeResult.ma5.toFixed(2)}</p></div>
            <div><p class="text-muted mb-1">MA20</p><p class="text-white font-mono">{computeResult.ma20.toFixed(2)}</p></div>
            <div><p class="text-muted mb-1">MA60</p><p class="text-white font-mono">{computeResult.ma60.toFixed(2)}</p></div>
            <div><p class="text-muted mb-1">RSI14</p><p class="text-white font-mono">{computeResult.rsi14.toFixed(2)}</p></div>
            <div><p class="text-muted mb-1">量比</p><p class="text-white font-mono">{computeResult.vol_ratio.toFixed(2)}x</p></div>
          </div>
          <p class="text-green-400 text-xs">
            {computeResult.symbol}（{computeResult.timeframe}）指標已算好並寫入，資料時間：{new Date(computeResult.ts).toLocaleString('zh-TW', { hour12: false })}
          </p>
        {/if}
      </div>
    </div>

    <!-- ── 手動評估訊號 ──────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl mt-6">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">手動評估訊號</h2>
        <p class="text-muted text-xs mt-1">
          完全基於 candles 計算（突破/跌破/爆量），不限監控清單，不用等排程；
          適合收盤後想立刻確認某支股票當天有沒有觸發訊號。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        {#if evalError}
          <p class="text-rise text-sm">{evalError}</p>
        {/if}

        <div class="flex gap-3">
          <input
            bind:value={evalSymbol}
            placeholder="股票代號，例如 00981A"
            on:keydown={(e) => e.key === 'Enter' && submitEvaluate()}
            class="flex-1 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
          <select
            bind:value={evalTimeframe}
            class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          >
            <option value="1d">1d</option>
            <option value="1m">1m</option>
          </select>
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors"
            disabled={evaluating}
            on:click={submitEvaluate}
          >
            {evaluating ? '評估中...' : '評估'}
          </button>
        </div>

        {#if evalResult}
          {#if evalResult.signal}
            {@const sig = evalResult.signal}
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs pt-2">
              <div><p class="text-muted mb-1">類型</p><p class="text-white font-mono">{sig.signal_type}</p></div>
              <div><p class="text-muted mb-1">方向</p><p class="text-white font-mono">{sig.direction}</p></div>
              <div><p class="text-muted mb-1">價格</p><p class="text-white font-mono">{sig.price.toFixed(2)}</p></div>
              <div><p class="text-muted mb-1">量比</p><p class="text-white font-mono">{sig.vol_ratio.toFixed(2)}x</p></div>
            </div>
            <p class="text-rise text-xs">{sig.note}</p>
          {:else}
            <p class="text-muted text-xs pt-1">{evalResult.message ?? '沒有觸發訊號'}</p>
          {/if}
        {/if}
      </div>
    </div>
  </div>
</Layout>
