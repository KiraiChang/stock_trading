<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import {
    submitBacktest,
    listBacktestJobs,
    getBacktestJob,
    getBacktestTrades,
    cancelBacktestJob,
    BACKTEST_STRATEGIES,
    isBacktraderStrategy,
    type BacktestJob,
    type BacktestResult,
    type BacktestTrade,
  } from '../lib/api/backtest'
  import { ApiError } from '../lib/api/client'
  import { describeBacktestCode, describeJobFailure } from '../lib/backtestErrors'
  import type { WatchlistItem } from '../lib/stores/market'

  const POLL_MS = 5000

  // ── 送出表單 ──────────────────────────────────────────────────
  let watchlistItems: WatchlistItem[] = []
  let selectedSymbols = new Set<string>()
  let strategy: string = BACKTEST_STRATEGIES[0].value
  let startDate = defaultStartDate()
  let endDate = defaultEndDate()
  let submitting = false
  let submitError = ''
  // backtrader 策略一次只能回測一檔（I-137）：首次載入與切換到它時⛔ 不預選，要使用者自己選
  const SINGLE_HINT = 'backtrader 版一次只能回測一檔，請選一檔'
  let selectHint = ''
  let lastStrategy = strategy
  $: singleMode = isBacktraderStrategy(strategy)
  $: onStrategyChange(strategy)

  // ── Job 清單（輪詢） ───────────────────────────────────────────
  let jobs: BacktestJob[] = []
  let jobsLoading = true
  let jobsError = ''
  let timer: ReturnType<typeof setInterval>

  // ── 選中 job 的結果詳情 ──────────────────────────────────────
  let selectedJobId = ''
  let selectedJob: BacktestJob | null = null
  let selectedResult: BacktestResult | null = null
  let selectedTrades: BacktestTrade[] = []
  let detailLoading = false
  let detailError = ''

  function defaultEndDate(): string {
    return new Date().toISOString().slice(0, 10)
  }

  function defaultStartDate(): string {
    const d = new Date()
    d.setDate(d.getDate() - 180)
    return d.toISOString().slice(0, 10)
  }

  onMount(async () => {
    watchlistItems = await fetchWatchlist().catch(() => [])
    if (isBacktraderStrategy(strategy)) {
      selectedSymbols = new Set()
      selectHint = SINGLE_HINT
    } else {
      selectedSymbols = new Set(watchlistItems.map((i) => i.symbol))
    }
    await loadJobs()
    timer = setInterval(loadJobs, POLL_MS)
  })

  onDestroy(() => clearInterval(timer))

  async function loadJobs() {
    jobsError = ''
    try {
      jobs = await listBacktestJobs(20)
    } catch {
      jobsError = '載入回測任務清單失敗'
    } finally {
      jobsLoading = false
    }

    // 正在看的 job 若已經跑完，順便靜默刷新結果
    const current = jobs.find((j) => j.job_id === selectedJobId)
    if (current && (current.status === 'done' || current.status === 'failed')) {
      loadDetail(selectedJobId, true)
    }
  }

  function onStrategyChange(s: string) {
    if (s === lastStrategy) return
    lastStrategy = s
    // 從 modular 切到 backtrader、而且已選超過 1 檔：清空並提示，⛔ 不自動挑一檔
    if (isBacktraderStrategy(s) && selectedSymbols.size > 1) {
      selectedSymbols = new Set()
      selectHint = SINGLE_HINT
    } else if (!isBacktraderStrategy(s)) {
      selectHint = ''
    }
  }

  function toggleSymbol(symbol: string) {
    if (singleMode) {
      // 單選：選新的一檔就取代舊的
      selectedSymbols = selectedSymbols.has(symbol) ? new Set() : new Set([symbol])
      if (selectedSymbols.size === 1) selectHint = ''
      return
    }
    if (selectedSymbols.has(symbol)) selectedSymbols.delete(symbol)
    else selectedSymbols.add(symbol)
    selectedSymbols = selectedSymbols
  }

  function toggleAllSymbols() {
    selectedSymbols =
      selectedSymbols.size === watchlistItems.length
        ? new Set()
        : new Set(watchlistItems.map((i) => i.symbol))
  }

  async function submit() {
    if (selectedSymbols.size === 0) {
      submitError = '請至少勾選一檔股票'
      return
    }
    if (!startDate || !endDate || startDate > endDate) {
      submitError = '請確認起訖日期（起始不能晚於結束）'
      return
    }

    submitting = true
    submitError = ''
    try {
      const job = await submitBacktest({
        strategy,
        symbols: Array.from(selectedSymbols),
        start_date: startDate,
        end_date: endDate,
      })
      await loadJobs()
      await selectJob(job.job_id)
    } catch (e) {
      // 400 是後端明確拒絕（日期、檔數、backtrader 多檔），顯示原因；其他錯誤維持原本的訊息
      submitError =
        e instanceof ApiError && e.status === 400
          ? describeBacktestCode(e.message) ?? '送出的內容不合法，請檢查日期與股票'
          : '送出回測失敗，請稍後再試'
    } finally {
      submitting = false
    }
  }

  async function selectJob(jobId: string) {
    selectedJobId = jobId
    await loadDetail(jobId)
  }

  async function loadDetail(jobId: string, silent = false) {
    if (!silent) {
      detailLoading = true
      detailError = ''
    }
    try {
      const { job, result } = await getBacktestJob(jobId)
      selectedJob = job
      selectedResult = result
      selectedTrades = result ? await getBacktestTrades(jobId) : []
    } catch {
      if (!silent) detailError = '載入回測結果失敗'
    } finally {
      if (!silent) detailLoading = false
    }
  }

  async function cancel(jobId: string) {
    try {
      await cancelBacktestJob(jobId)
      await loadJobs()
    } catch {
      jobsError = '取消失敗（可能已經開始執行）'
    }
  }

  function parseSymbols(symbolsJson: string): string {
    try {
      return (JSON.parse(symbolsJson) as string[]).join(', ')
    } catch {
      return symbolsJson
    }
  }

  function formatDateTime(ts?: string): string {
    if (!ts) return '—'
    return new Date(ts).toLocaleString('zh-TW', { hour12: false })
  }

  function formatPct(v: number): string {
    return `${(v * 100).toFixed(2)}%`
  }

  const statusLabel: Record<string, string> = {
    pending: '等待中',
    running: '執行中',
    done: '完成',
    failed: '失敗',
  }
  const statusClass: Record<string, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    failed: 'bg-red-900/40 text-red-400',
  }
</script>

<Layout>
  <div class="max-w-5xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">策略回測</h1>

    <!-- ── 送出表單 ──────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl">
      <div class="px-5 py-3 border-b border-border">
        <h2 class="text-sm font-semibold text-white">建立回測任務</h2>
      </div>

      <div class="px-5 py-4 space-y-4">
        {#if submitError}
          <p class="text-rise text-sm">{submitError}</p>
        {/if}

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label for="bt-strategy" class="block text-xs text-muted mb-1">策略</label>
            <select
              id="bt-strategy"
              bind:value={strategy}
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            >
              {#each BACKTEST_STRATEGIES as s}
                <option value={s.value}>{s.label}</option>
              {/each}
            </select>
          </div>
          <div>
            <label for="bt-start" class="block text-xs text-muted mb-1">起始日期</label>
            <input
              id="bt-start"
              type="date"
              bind:value={startDate}
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </div>
          <div>
            <label for="bt-end" class="block text-xs text-muted mb-1">結束日期</label>
            <input
              id="bt-end"
              type="date"
              bind:value={endDate}
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </div>
        </div>

        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs text-muted">選擇股票</span>
            {#if !singleMode}
              <button class="text-xs text-muted hover:text-white transition-colors" on:click={toggleAllSymbols}>
                {selectedSymbols.size === watchlistItems.length && watchlistItems.length > 0 ? '取消全選' : '全選'}
              </button>
            {/if}
          </div>
          {#if selectHint}
            <p class="text-xs text-amber-400 mb-2" data-testid="select-hint">{selectHint}</p>
          {/if}
          <div class="max-h-48 overflow-y-auto border border-border rounded-lg">
            {#if watchlistItems.length === 0}
              <p class="px-4 py-6 text-center text-muted text-sm">尚無監控股票，請先至 Dashboard 新增</p>
            {:else}
              {#each watchlistItems as item (item.symbol)}
                <label
                  class="flex items-center gap-3 px-4 py-2 border-b border-border/50 last:border-0
                         hover:bg-border/20 cursor-pointer transition-colors"
                >
                  <input
                    type={singleMode ? 'radio' : 'checkbox'}
                    name={singleMode ? 'bt-symbol' : undefined}
                    checked={selectedSymbols.has(item.symbol)}
                    on:change={() => toggleSymbol(item.symbol)}
                    class="accent-indigo-600 w-4 h-4"
                  />
                  <span class="text-white text-sm font-medium">{item.symbol}</span>
                  <span class="text-muted text-xs truncate">{item.name}</span>
                </label>
              {/each}
            {/if}
          </div>
        </div>

        <button
          class="w-full bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium py-2.5 rounded-lg transition-colors"
          disabled={submitting}
          on:click={submit}
        >
          {submitting ? '送出中...' : `送出回測（已選 ${selectedSymbols.size} 檔）`}
        </button>
      </div>
    </div>

    <!-- ── Job 清單 ──────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl overflow-hidden">
      <div class="px-5 py-3 border-b border-border flex items-center justify-between">
        <h2 class="text-sm font-semibold text-white">回測任務</h2>
        <button class="text-xs text-muted hover:text-white transition-colors" on:click={loadJobs}>重新整理</button>
      </div>

      {#if jobsError}
        <p class="text-rise text-sm px-5 py-2">{jobsError}</p>
      {/if}

      <table class="w-full text-sm">
        <thead>
          <tr class="text-muted text-xs border-b border-border">
            <th class="text-left px-5 py-2">Job ID</th>
            <th class="text-left px-3 py-2">策略</th>
            <th class="text-left px-3 py-2">股票</th>
            <th class="text-center px-3 py-2">狀態</th>
            <th class="text-left px-3 py-2">建立時間</th>
            <th class="text-right px-5 py-2">操作</th>
          </tr>
        </thead>
        <tbody>
          {#if jobsLoading}
            <tr><td colspan="6" class="px-5 py-8 text-center text-muted">載入中...</td></tr>
          {:else if jobs.length === 0}
            <tr><td colspan="6" class="px-5 py-8 text-center text-muted">尚無回測任務</td></tr>
          {:else}
            {#each jobs as job (job.job_id)}
              <tr
                class="border-b border-border/50 hover:bg-border/20 cursor-pointer transition-colors
                       {selectedJobId === job.job_id ? 'bg-indigo-900/20' : ''}"
                on:click={() => selectJob(job.job_id)}
              >
                <td class="px-5 py-2 font-mono text-xs text-muted truncate max-w-32">{job.job_id}</td>
                <td class="px-3 py-2 text-white text-xs">{job.strategy}</td>
                <td class="px-3 py-2 text-muted text-xs truncate max-w-40">{parseSymbols(job.symbols)}</td>
                <td class="px-3 py-2 text-center">
                  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                    {statusClass[job.status] ?? 'bg-gray-700/60 text-gray-400'}">
                    {statusLabel[job.status] ?? job.status}
                  </span>
                </td>
                <td class="px-3 py-2 text-muted text-xs font-mono">{formatDateTime(job.created_at)}</td>
                <td class="px-5 py-2 text-right">
                  {#if job.status === 'pending'}
                    <button
                      class="text-xs px-2.5 py-1 border border-red-600/40 text-red-400 hover:bg-red-900/20 rounded transition-colors"
                      on:click|stopPropagation={() => cancel(job.job_id)}
                    >取消</button>
                  {/if}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- ── 選中 Job 的結果 ───────────────────────────────────── -->
    {#if selectedJobId}
      <div class="bg-panel border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-3 border-b border-border">
          <h2 class="text-sm font-semibold text-white">回測結果 — {selectedJobId}</h2>
        </div>

        {#if detailLoading}
          <p class="px-5 py-8 text-center text-muted text-sm">載入中...</p>
        {:else if detailError}
          <p class="px-5 py-4 text-rise text-sm">{detailError}</p>
        {:else if selectedJob?.status === 'failed'}
          <div class="px-5 py-4 text-sm" data-testid="job-failed">
            <p class="text-rise font-medium">回測失敗</p>
            <p class="text-muted mt-1">{describeJobFailure(selectedJob.error)}</p>
          </div>
        {:else if !selectedResult}
          <p class="px-5 py-8 text-center text-muted text-sm">尚未完成，稍後會自動刷新</p>
        {:else}
          {#if selectedResult.data_coverage && selectedResult.data_coverage.length > 0}
            <ul class="px-5 pt-4 space-y-1 text-xs" data-testid="data-coverage">
              {#each selectedResult.data_coverage as c (c.symbol)}
                <li class="text-muted">
                  <span class="text-white font-medium">{c.symbol}</span>：
                  {#if c.skipped}
                    <span class="text-amber-400">區間內沒有資料，已略過</span>
                  {:else}
                    {c.first_bar}～{c.last_bar}，{c.bars} 根
                    {#if c.warmup_bars < c.warmup_required}
                      <span class="text-amber-400">，暖身不足（{c.warmup_bars}／{c.warmup_required} 根）</span>
                    {/if}
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 px-5 py-4 text-xs border-b border-border">
            <div>
              <p class="text-muted mb-1">總報酬率</p>
              <p class={selectedResult.total_return >= 0 ? 'text-rise' : 'text-fall'}>{formatPct(selectedResult.total_return)}</p>
            </div>
            <div>
              <p class="text-muted mb-1">年化報酬率</p>
              <p class={selectedResult.annual_return >= 0 ? 'text-rise' : 'text-fall'}>{formatPct(selectedResult.annual_return)}</p>
            </div>
            <div>
              <p class="text-muted mb-1">勝率</p>
              <p class="text-white">{formatPct(selectedResult.win_rate)}</p>
            </div>
            <div>
              <p class="text-muted mb-1">最大回撤</p>
              <p class="text-fall">{formatPct(selectedResult.max_drawdown)}</p>
            </div>
            <div>
              <p class="text-muted mb-1">Sharpe Ratio</p>
              <p class="text-white">{selectedResult.sharpe_ratio.toFixed(2)}</p>
            </div>
            <div>
              <p class="text-muted mb-1">總交易數</p>
              <p class="text-white">{selectedResult.total_trades}</p>
            </div>
            <div>
              <p class="text-muted mb-1">勝 / 敗</p>
              <p class="text-white">{selectedResult.win_trades} / {selectedResult.loss_trades}</p>
            </div>
            <div>
              <p class="text-muted mb-1">平均損益</p>
              <p class={selectedResult.avg_pnl >= 0 ? 'text-rise' : 'text-fall'}>{selectedResult.avg_pnl.toFixed(2)}</p>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-muted text-xs border-b border-border">
                  <th class="text-left px-5 py-2">股票</th>
                  <th class="text-center px-3 py-2">方向</th>
                  <th class="text-left px-3 py-2">進場時間</th>
                  <th class="text-left px-3 py-2">出場時間</th>
                  <th class="text-right px-3 py-2">進場價</th>
                  <th class="text-right px-3 py-2">出場價</th>
                  <th class="text-right px-3 py-2">損益</th>
                  <th class="text-right px-5 py-2">損益%</th>
                </tr>
              </thead>
              <tbody>
                {#if selectedTrades.length === 0}
                  <tr><td colspan="8" class="px-5 py-6 text-center text-muted text-sm">沒有交易紀錄</td></tr>
                {:else}
                  {#each selectedTrades as t (t.id)}
                    <tr class="border-b border-border/50">
                      <td class="px-5 py-2 text-white font-medium">{t.symbol}</td>
                      <td class="px-3 py-2 text-center text-xs">{t.direction}</td>
                      <td class="px-3 py-2 text-muted text-xs font-mono">{formatDateTime(t.entry_time)}</td>
                      <td class="px-3 py-2 text-muted text-xs font-mono">{formatDateTime(t.exit_time)}</td>
                      <td class="px-3 py-2 text-right font-mono">{t.entry_price.toFixed(2)}</td>
                      <td class="px-3 py-2 text-right font-mono">{t.exit_price.toFixed(2)}</td>
                      <td class="px-3 py-2 text-right font-mono {t.pnl >= 0 ? 'text-rise' : 'text-fall'}">{t.pnl.toFixed(2)}</td>
                      <td class="px-5 py-2 text-right font-mono {t.pnl_pct >= 0 ? 'text-rise' : 'text-fall'}">{formatPct(t.pnl_pct)}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}

    <p class="text-muted text-xs">
      回測任務清單每 {POLL_MS / 1000} 秒自動輪詢一次；送出後點選任務列可查看結果，未完成前結果區塊會顯示「尚未完成」。
    </p>
  </div>
</Layout>
