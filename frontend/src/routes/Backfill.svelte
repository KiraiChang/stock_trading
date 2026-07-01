<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import { triggerBackfill } from '../lib/api/market'
  import type { WatchlistItem } from '../lib/stores/market'

  let items: WatchlistItem[] = []
  let selected = new Set<string>()
  let days = 120
  let loading = true
  let submitting = false
  let error = ''
  let result: { symbols: number; days: number } | null = null

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
  </div>
</Layout>
