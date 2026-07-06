<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { selectedSymbol } from '../lib/stores/market'
  import { ApiError } from '../lib/api/client'
  import {
    fetchChipSummary,
    fetchChipScores,
    fetchChipBrokers,
    triggerChipSync,
    getChipSyncJob,
    type ChipSummary,
    type ChipScore,
    type BrokerTrade,
    type ChipSyncJob,
  } from '../lib/api/chips'
  import ChipScorePanel from '../components/chip/ChipScorePanel.svelte'
  import InstitutionalTrend from '../components/chip/InstitutionalTrend.svelte'
  import MarginTrend from '../components/chip/MarginTrend.svelte'
  import BrokerRankingTable from '../components/chip/BrokerRankingTable.svelte'

  let symbol = $selectedSymbol || ''
  let date = ''

  let summary: ChipSummary | null = null
  let loadingSummary = false
  let summaryError = ''

  let scores: ChipScore[] = []

  let topBuy: BrokerTrade[] = []
  let topSell: BrokerTrade[] = []

  let syncing = false
  let syncError = ''
  let syncJob: ChipSyncJob | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null

  function todayStr(): string {
    const d = new Date()
    return d.toISOString().substring(0, 10)
  }
  function daysAgo(n: number): string {
    const d = new Date()
    d.setDate(d.getDate() - n)
    return d.toISOString().substring(0, 10)
  }

  async function loadSummary() {
    loadingSummary = true
    summaryError = ''
    try {
      summary = await fetchChipSummary(symbol.trim(), date || undefined)
    } catch (err) {
      summary = null
      if (err instanceof ApiError) {
        summaryError = err.status === 404 ? '尚無籌碼資料，請先執行手動同步' : err.message
      } else {
        summaryError = '載入失敗，請確認後端服務是否正常'
      }
    } finally {
      loadingSummary = false
    }
  }

  async function loadScores() {
    try {
      scores = await fetchChipScores(symbol.trim(), daysAgo(60), date || todayStr())
    } catch {
      scores = [] // 沉默失敗，不影響分數面板的呈現
    }
  }

  async function loadBrokers() {
    try {
      const res = await fetchChipBrokers(symbol.trim(), date || todayStr(), 20)
      topBuy = res.topBuy ?? []
      topSell = res.topSell ?? []
    } catch {
      topBuy = []
      topSell = []
    }
  }

  async function loadAll() {
    if (!symbol.trim()) return
    await Promise.all([loadSummary(), loadScores(), loadBrokers()])
  }

  async function runSync() {
    if (!symbol.trim()) return
    syncing = true
    syncError = ''
    try {
      const job = await triggerChipSync({
        mode: 'manual',
        symbols: [symbol.trim()],
        from: daysAgo(30),
        to: date || todayStr(),
        dataTypes: ['institutional', 'margin', 'broker', 'scores'],
      })
      syncJob = job
      pollSyncJob(job.job_id)
    } catch (err) {
      syncError = err instanceof ApiError ? err.message : '同步觸發失敗，請確認後端服務是否正常'
      syncing = false
    }
  }

  // 每 2 秒查一次同步任務狀態，直到 done/partial/failed 才停止，完成後重新
  // 載入頁面資料（比照 SRZones.svelte 訓練任務輪詢的做法）。
  function pollSyncJob(jobId: string) {
    stopPolling()
    pollTimer = setInterval(async () => {
      try {
        const job = await getChipSyncJob(jobId)
        syncJob = job
        if (job.status === 'done' || job.status === 'partial' || job.status === 'failed') {
          stopPolling()
          syncing = false
          await loadAll()
        }
      } catch {
        syncError = '查詢同步狀態失敗'
        stopPolling()
        syncing = false
      }
    }, 2000)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  onMount(() => {
    if (symbol.trim()) loadAll()
    return stopPolling
  })

  // 與既有股票選擇同步：切換全域選中股票時自動載入（比照其他頁面重用
  // selectedSymbol store 的慣例）
  $: if ($selectedSymbol && $selectedSymbol !== symbol) {
    symbol = $selectedSymbol
    loadAll()
  }

  const syncStatusText: Record<string, string> = {
    pending: '排隊中', running: '同步中', done: '完成', partial: '部分成功', failed: '失敗',
  }
  const syncStatusClass: Record<string, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    partial: 'bg-yellow-900/40 text-yellow-400',
    failed: 'bg-red-900/40 text-red-400',
  }
</script>

<Layout>
  <div class="max-w-5xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">籌碼分析</h1>

    <!-- ── 輸入表單 ──────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      <div class="flex gap-3 flex-wrap items-center">
        <input
          bind:value={symbol}
          placeholder="輸入股票代號，例如 2330"
          on:keydown={(e) => e.key === 'Enter' && loadAll()}
          class="flex-1 min-w-[160px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <input
          type="date"
          bind:value={date}
          on:change={loadAll}
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={!symbol.trim()}
          on:click={loadAll}
        >
          查詢
        </button>
        <button
          class="border border-border text-muted hover:text-white text-sm font-medium px-4 py-2 rounded-lg
                 transition-colors disabled:opacity-50"
          disabled={syncing || !symbol.trim()}
          on:click={runSync}
        >
          {syncing ? '同步中...' : '手動同步'}
        </button>
      </div>
      <p class="text-muted text-xs mt-2">
        手動同步會抓取最近 30 天的三大法人、融資融券、券商分點資料並重新計算籌碼分數（分點資料若來源不支援會自動略過，不影響其他分數）。
      </p>
      {#if syncError}
        <p class="text-rise text-xs mt-2">{syncError}</p>
      {/if}
      {#if syncJob}
        <p class="text-xs mt-2 flex items-center gap-2 flex-wrap">
          <span class="text-muted">同步任務 <span class="font-mono">{syncJob.job_id}</span>：</span>
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {syncStatusClass[syncJob.status] ?? ''}">
            {syncStatusText[syncJob.status] ?? syncJob.status}
          </span>
          <span class="text-muted">（{syncJob.symbols_done}/{syncJob.symbols_total}，失敗 {syncJob.symbols_failed}）</span>
        </p>
      {/if}
    </div>

    {#if summaryError}
      <p class="text-rise text-sm">{summaryError}</p>
    {/if}

    <!-- ── 籌碼總分與訊號 ────────────────────────────────────── -->
    <ChipScorePanel {summary} loading={loadingSummary} />

    <!-- ── 法人/融資融券趨勢圖 ──────────────────────────────── -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <InstitutionalTrend {scores} />
      <MarginTrend {scores} />
    </div>

    <!-- ── 主力分點排行 ─────────────────────────────────────── -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <BrokerRankingTable title="主力買超排行" rows={topBuy} />
      <BrokerRankingTable title="主力賣超排行" rows={topSell} />
    </div>
  </div>
</Layout>
