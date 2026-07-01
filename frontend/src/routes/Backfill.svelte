<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import { triggerBackfill } from '../lib/api/market'
  import { computeIndicators, type IndicatorSnapshot } from '../lib/api/indicators'
  import type { WatchlistItem } from '../lib/stores/market'

  let items: WatchlistItem[] = []
  let selected = new Set<string>()
  let days = 120
  let loading = true
  let submitting = false
  let error = ''
  let result: { symbols: number; days: number } | null = null

  // ── 手動計算指標：不限監控清單，任意股票代號都可以 ──────────
  let computeSymbol = ''
  let computeTimeframe = '1d'
  let computing = false
  let computeError = ''
  let computeResult: IndicatorSnapshot | null = null

  onMount(load)

  async function load() {
    loading = true
    error = ''
    try {
      items = await fetchWatchlist()
      selected = new Set(items.map((i) => i.symbol)) // 預設全選
    } catch {
      error = '載入監控清單失敗'
    } finally {
      loading = false
    }
  }

  function toggle(symbol: string) {
    if (selected.has(symbol)) selected.delete(symbol)
    else selected.add(symbol)
    selected = selected // 觸發 Svelte reactivity
  }

  function toggleAll() {
    selected = selected.size === items.length ? new Set() : new Set(items.map((i) => i.symbol))
  }

  async function submit() {
    if (selected.size === 0) { error = '請至少勾選一檔股票'; return }
    if (!Number.isFinite(days) || days <= 0) { error = '回補天數需大於 0'; return }

    submitting = true
    error = ''
    result = null
    try {
      result = await triggerBackfill({ days, symbols: Array.from(selected) })
    } catch {
      error = '回補請求送出失敗，請稍後再試'
    } finally {
      submitting = false
    }
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

    {#if result}
      <p class="text-green-400 text-sm mb-4">
        已在背景啟動：{result.symbols} 檔股票、回補 {result.days} 天歷史資料，完成進度不會即時回報。
      </p>
    {/if}

    <div class="bg-panel border border-border rounded-xl mb-4">
      <div class="px-5 py-4 border-b border-border flex items-center gap-3">
        <label for="bf-days" class="text-sm text-muted">回補天數</label>
        <input
          id="bf-days"
          type="number"
          min="1"
          bind:value={days}
          class="w-24 bg-surface border border-border rounded-lg px-3 py-1.5 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
      </div>

      <div class="px-5 py-3 border-b border-border flex items-center justify-between">
        <h2 class="text-sm font-semibold text-white">選擇股票</h2>
        <button
          class="text-xs text-muted hover:text-white transition-colors"
          on:click={toggleAll}
        >
          {selected.size === items.length && items.length > 0 ? '取消全選' : '全選'}
        </button>
      </div>

      <div class="max-h-96 overflow-y-auto">
        {#if loading}
          <p class="px-5 py-8 text-center text-muted text-sm">載入中...</p>
        {:else if items.length === 0}
          <p class="px-5 py-8 text-center text-muted text-sm">尚無監控股票，請先至 Dashboard 新增</p>
        {:else}
          {#each items as item (item.symbol)}
            <label
              class="flex items-center gap-3 px-5 py-2.5 border-b border-border/50 last:border-0
                     hover:bg-border/20 cursor-pointer transition-colors"
            >
              <input
                type="checkbox"
                checked={selected.has(item.symbol)}
                on:change={() => toggle(item.symbol)}
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
      disabled={submitting || loading}
      on:click={submit}
    >
      {submitting ? '送出中...' : `開始回補（已選 ${selected.size} 檔）`}
    </button>

    <p class="text-muted text-xs mt-3">
      回補會在後端背景執行（呼叫 POST /market/backfill），不會即時回報每檔股票的完成進度；
      可稍後至排程監控頁面或後端 log 確認執行結果。
    </p>

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
  </div>
</Layout>
