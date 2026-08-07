<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { bulkAddToWatchlist } from '../../lib/api/watchlist'
  import { triggerBackfill } from '../../lib/api/market'

  const dispatch = createEventDispatcher<{ done: void; skip: void }>()

  // Step 1: 輸入股票  Step 2: 確認 backfill  Step 3: 完成
  let step = 1
  let stockInput = ''   // 每行一筆：「代號 名稱 產業」或僅「代號」
  let backfillDays = 120
  let loading = false
  let error = ''
  let addResult = { added: 0, failed: 0 }
  // step 1 剛加入的代號，step 2 回補時直接沿用。
  // triggerBackfill 的 symbols 是必填（API 層不認識 watchlist，見 todo.md T-040），
  // 這裡不能只送 days 讓後端自己去撈清單。
  let addedSymbols: string[] = []

  // 解析輸入：每行 "2330 台積電 半導體" 或 "2330"
  function parseStocks(raw: string) {
    return raw
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const parts = line.split(/\s+/)
        return {
          symbol: parts[0],
          name:   parts[1] ?? parts[0],
          sector: parts[2] ?? '',
        }
      })
  }

  async function submitStocks() {
    error = ''
    const items = parseStocks(stockInput)
    if (items.length === 0) {
      error = '請至少輸入一筆股票代號'
      return
    }
    loading = true
    try {
      addResult = await bulkAddToWatchlist(items)
      addedSymbols = items.map((i) => i.symbol)
      step = 2
    } catch {
      error = '新增失敗，請確認格式或稍後再試'
    } finally {
      loading = false
    }
  }

  async function submitBackfill() {
    loading = true
    error = ''
    try {
      await triggerBackfill({ days: backfillDays, symbols: addedSymbols })
      step = 3
    } catch {
      error = 'Backfill 啟動失敗，可稍後在 Dashboard 手動觸發'
      step = 3
    } finally {
      loading = false
    }
  }
</script>

<!-- Backdrop -->
<div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
  <div class="bg-panel border border-border rounded-xl w-full max-w-md shadow-2xl">

    <!-- Header -->
    <div class="px-6 py-4 border-b border-border flex items-center justify-between">
      <div>
        <h2 class="text-white font-semibold">初始設定</h2>
        <p class="text-muted text-xs mt-0.5">步驟 {step} / 3</p>
      </div>
      <!-- Step dots -->
      <div class="flex gap-1.5">
        {#each [1, 2, 3] as s}
          <div class="w-2 h-2 rounded-full {step >= s ? 'bg-indigo-500' : 'bg-border'}"></div>
        {/each}
      </div>
    </div>

    <div class="px-6 py-5">

      <!-- Step 1: 新增股票 -->
      {#if step === 1}
        <p class="text-sm text-gray-300 mb-4">
          輸入要監控的股票，每行一筆。格式：<span class="font-mono text-white">代號 名稱 產業</span>（名稱與產業可省略）
        </p>
        <textarea
          bind:value={stockInput}
          placeholder={"2330 台積電 半導體\n2454 聯發科 半導體\n2317"}
          rows="7"
          class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 font-mono placeholder:text-muted focus:outline-none focus:border-indigo-500
                 resize-none transition-colors"
        ></textarea>
        {#if error}<p class="text-rise text-xs mt-2">{error}</p>{/if}

        <div class="flex gap-3 mt-5">
          <button
            class="flex-1 text-muted hover:text-white text-sm py-2 border border-border rounded-lg transition-colors"
            on:click={() => dispatch('skip')}
          >
            略過
          </button>
          <button
            class="flex-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium py-2 rounded-lg transition-colors"
            disabled={loading}
            on:click={submitStocks}
          >
            {loading ? '新增中...' : '新增並繼續'}
          </button>
        </div>

      <!-- Step 2: Backfill 設定 -->
      {:else if step === 2}
        <p class="text-sm text-gray-300 mb-1">
          成功新增 <span class="text-white font-semibold">{addResult.added}</span> 支股票。
        </p>
        <p class="text-sm text-gray-300 mb-5">
          要從 FinMind 預載歷史 K 棒資料嗎？（在背景執行，不影響使用）
        </p>

        <p class="text-xs text-muted mb-2">補載天數</p>
        <div class="flex gap-2 mb-5">
          {#each [30, 60, 120, 365] as d}
            <button
              class="flex-1 py-1.5 text-sm rounded-lg border transition-colors
                     {backfillDays === d
                       ? 'border-indigo-500 bg-indigo-600/20 text-indigo-300'
                       : 'border-border text-muted hover:text-white hover:border-gray-500'}"
              on:click={() => backfillDays = d}
            >
              {d}天
            </button>
          {/each}
        </div>
        {#if error}<p class="text-rise text-xs mb-3">{error}</p>{/if}

        <div class="flex gap-3">
          <button
            class="flex-1 text-muted hover:text-white text-sm py-2 border border-border rounded-lg transition-colors"
            on:click={() => { step = 3 }}
          >
            略過
          </button>
          <button
            class="flex-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium py-2 rounded-lg transition-colors"
            disabled={loading}
            on:click={submitBackfill}
          >
            {loading ? '啟動中...' : '開始預載'}
          </button>
        </div>

      <!-- Step 3: 完成 -->
      {:else}
        <div class="text-center py-4">
          <div class="text-4xl mb-3">✓</div>
          <p class="text-white font-medium mb-1">設定完成</p>
          <p class="text-muted text-sm">
            資料預載在背景執行中，完成後即可看到 K 棒與指標。
          </p>
        </div>
        <button
          class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-sm
                 py-2 rounded-lg transition-colors mt-4"
          on:click={() => dispatch('done')}
        >
          進入 Dashboard
        </button>
      {/if}

    </div>
  </div>
</div>
