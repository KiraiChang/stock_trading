<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { watchlist, quotes, selectedSymbol } from '../../lib/stores/market'
  import type { WatchlistItem } from '../../lib/stores/market'
  import { formatPrice, formatChangePct } from '../../lib/utils/format'
  import { priceColor, trendColor } from '../../lib/utils/color'
  import {
    fetchWatchlist,
    addToWatchlist,
    updateWatchlist,
    removeFromWatchlist,
    setWatched,
    searchStockSymbols,
  } from '../../lib/api/watchlist'
  import type { StockSymbol } from '../../lib/api/watchlist'

  const dispatch = createEventDispatcher<{
    symbolWatched: string
    symbolUnwatched: string
  }>()

  // 即時監聽（WebSocket 訂閱）最多 3 檔，超過上限後端回 409
  let watchError = ''

  async function toggleWatch(item: WatchlistItem) {
    const next = !item.watched
    watchError = ''
    try {
      await setWatched(item.symbol, next)
      watchlist.set(await fetchWatchlist())
      dispatch(next ? 'symbolWatched' : 'symbolUnwatched', item.symbol)
    } catch (e) {
      watchError = e instanceof Error && e.message.includes('409')
        ? '已達監聽上限（3 檔），請先取消其他股票的監聽'
        : '設定監聽失敗，請稍後再試'
    }
  }

  const trendLabel: Record<string, string> = {
    BULLISH: '多頭',
    BEARISH: '空頭',
    SIDEWAYS: '盤整',
    '': '-',
  }

  // ── Modal 狀態 ──────────────────────────────────────────────
  let showModal = false
  let modalMode: 'add' | 'edit' = 'add'
  let form = { symbol: '', name: '', sector: '' }
  let selectedStock: StockSymbol | null = null
  let suggestions: StockSymbol[] = []
  let searching = false
  let searchTimer: ReturnType<typeof setTimeout> | null = null
  let submitting = false
  let formError = ''
  type StatusFilter = 'all' | 'listed' | 'delisted' | 'unknown'
  let statusFilter: StatusFilter = 'all'
  const statusOptions: { value: StatusFilter; label: string }[] = [
    { value: 'all', label: '全部' },
    { value: 'listed', label: '上市' },
    { value: 'delisted', label: '已下架' },
    { value: 'unknown', label: '未知' },
  ]

  $: filteredWatchlist = $watchlist.filter((item) => {
    if (statusFilter === 'listed') return item.stock_symbol?.exists && item.stock_symbol.is_listed
    if (statusFilter === 'delisted') return item.stock_symbol?.exists && !item.stock_symbol.is_listed
    if (statusFilter === 'unknown') return !item.stock_symbol?.exists
    return true
  })

  // ── Delete 確認狀態 ─────────────────────────────────────────
  let confirmDeleteSymbol = ''

  // ── 開啟 Modal ───────────────────────────────────────────────
  function openAdd() {
    form = { symbol: '', name: '', sector: '' }
    selectedStock = null
    suggestions = []
    modalMode = 'add'
    formError = ''
    showModal = true
  }

  function openEdit(item: WatchlistItem) {
    form = { symbol: item.symbol, name: item.name, sector: item.sector }
    selectedStock = null
    suggestions = []
    modalMode = 'edit'
    formError = ''
    showModal = true
  }

  function closeModal() {
    showModal = false
    submitting = false
    formError = ''
  }

  function stockStatusLabel(item: WatchlistItem): string {
    if (!item.stock_symbol?.exists) return '主檔未知'
    return item.stock_symbol.is_listed ? '上市' : '已下架'
  }

  function stockStatusClass(item: WatchlistItem): string {
    if (!item.stock_symbol?.exists) return 'text-muted border-border'
    return item.stock_symbol.is_listed
      ? 'text-emerald-300 border-emerald-500/30'
      : 'text-amber-300 border-amber-500/40'
  }

  function scheduleSymbolSearch() {
    selectedStock = null
    form.symbol = form.symbol.trim().toUpperCase()
    form.name = ''
    form.sector = ''
    if (searchTimer) clearTimeout(searchTimer)
    const query = form.symbol.trim()
    if (query.length < 2) {
      suggestions = []
      searching = false
      return
    }
    searching = true
    searchTimer = setTimeout(async () => {
      try {
        suggestions = await searchStockSymbols(query, { listed: true, limit: 8 })
      } catch {
        suggestions = []
      } finally {
        searching = false
      }
    }, 180)
  }

  function selectStock(stock: StockSymbol) {
    selectedStock = stock
    suggestions = []
    form.symbol = stock.symbol
    form.name = stock.name
    form.sector = stock.industry
  }

  // ── 送出 Add / Edit ─────────────────────────────────────────
  async function submitForm() {
    if (!form.symbol.trim()) { formError = '請輸入股票代號'; return }
    if (modalMode === 'edit' && !form.name.trim()) { formError = '請輸入股票名稱'; return }

    submitting = true
    formError = ''
    try {
      if (modalMode === 'add') {
        await addToWatchlist(form.symbol.trim(), form.name.trim(), form.sector.trim())
      } else {
        await updateWatchlist(form.symbol, form.name.trim(), form.sector.trim())
      }
      watchlist.set(await fetchWatchlist())
      closeModal()
    } catch (e) {
      const msg = e instanceof Error ? e.message : ''
      formError = msg.includes('不在主檔')
        ? '主檔找不到這個代號，請改用搜尋結果或手動輸入名稱'
        : modalMode === 'add' ? '新增失敗，請稍後再試' : '更新失敗，請稍後再試'
    } finally {
      submitting = false
    }
  }

  // ── Delete ───────────────────────────────────────────────────
  async function confirmDelete(symbol: string) {
    try {
      await removeFromWatchlist(symbol)
      watchlist.set(await fetchWatchlist())
    } catch {
      // silent — row will remain
    } finally {
      confirmDeleteSymbol = ''
    }
  }
</script>

<!-- ── Add / Edit Modal ────────────────────────────────────── -->
{#if showModal}
  <div role="presentation"
       class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4"
       on:click|self={closeModal}
       on:keydown|self={(e) => e.key === 'Escape' && closeModal()}>
    <div class="bg-panel border border-border rounded-xl w-full max-w-sm shadow-2xl">
      <div class="px-5 py-4 border-b border-border flex items-center justify-between">
        <h3 class="text-white font-semibold text-sm">
          {modalMode === 'add' ? '新增監控股票' : '編輯監控股票'}
        </h3>
        <button class="text-muted hover:text-white transition-colors text-lg leading-none"
                on:click={closeModal}>×</button>
      </div>

      <div class="px-5 py-4 space-y-3">
        <div>
          <label for="wl-symbol" class="block text-xs text-muted mb-1">代號 <span class="text-rise">*</span></label>
          <div class="relative">
            <input
              id="wl-symbol"
              bind:value={form.symbol}
              disabled={modalMode === 'edit'}
              placeholder="搜尋代號或名稱"
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors
                     disabled:opacity-50 disabled:cursor-not-allowed"
              on:input={modalMode === 'add' ? scheduleSymbolSearch : undefined}
            />
            {#if modalMode === 'add' && (suggestions.length > 0 || searching)}
              <div class="absolute z-10 mt-1 w-full bg-panel border border-border rounded-lg shadow-xl overflow-hidden">
                {#if searching}
                  <div class="px-3 py-2 text-xs text-muted">搜尋中...</div>
                {:else}
                  {#each suggestions as stock (stock.symbol)}
                    <button
                      class="w-full px-3 py-2 text-left hover:bg-border/40 transition-colors"
                      on:click={() => selectStock(stock)}
                    >
                      <div class="flex items-center justify-between gap-3">
                        <span class="text-sm text-white font-medium">{stock.symbol} {stock.name}</span>
                        <span class="text-xs text-muted">{stock.security_type}</span>
                      </div>
                      <div class="text-xs text-muted truncate">{stock.market} · {stock.industry || '未分類'}</div>
                    </button>
                  {/each}
                {/if}
              </div>
            {/if}
          </div>
        </div>
        <div>
          <label for="wl-name" class="block text-xs text-muted mb-1">名稱 <span class="text-rise">*</span></label>
          <input
            id="wl-name"
            bind:value={form.name}
            placeholder="例：台積電"
            readonly={modalMode === 'add' && selectedStock !== null}
            class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors
                   read-only:opacity-70"
          />
        </div>
        <div>
          <label for="wl-sector" class="block text-xs text-muted mb-1">產業</label>
          <input
            id="wl-sector"
            bind:value={form.sector}
            placeholder="例：半導體"
            readonly={modalMode === 'add' && selectedStock !== null}
            class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors
                   read-only:opacity-70"
          />
        </div>

        {#if formError}
          <p class="text-rise text-xs">{formError}</p>
        {/if}

        <div class="flex gap-3 pt-1">
          <button
            class="flex-1 text-muted hover:text-white text-sm py-2 border border-border rounded-lg transition-colors"
            on:click={closeModal}
          >取消</button>
          <button
            class="flex-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium py-2 rounded-lg transition-colors"
            disabled={submitting}
            on:click={submitForm}
          >{submitting ? '儲存中...' : '儲存'}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- ── Table ─────────────────────────────────────────────────── -->
<div class="bg-panel rounded-lg overflow-hidden border border-border">
  <div class="px-4 py-3 border-b border-border flex items-center justify-between">
    <h2 class="text-sm font-semibold text-white">監控清單</h2>
    <div class="flex items-center gap-2">
      <div class="flex rounded-lg border border-border overflow-hidden">
        {#each statusOptions as option}
          <button
            class="text-xs px-2.5 py-1 transition-colors {statusFilter === option.value ? 'bg-indigo-600 text-white' : 'text-muted hover:text-white'}"
            on:click={() => statusFilter = option.value}
          >{option.label}</button>
        {/each}
      </div>
      <button
        class="text-xs px-2.5 py-1 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition-colors"
        on:click={openAdd}
      >+ 新增</button>
    </div>
  </div>

  {#if watchError}
    <p class="px-4 py-2 text-rise text-xs border-b border-border">{watchError}</p>
  {/if}

  <div class="overflow-x-auto">
    <table class="w-full text-sm">
      <thead>
        <tr class="text-muted border-b border-border text-xs">
          <th class="text-left px-4 py-2">代號</th>
          <th class="text-right px-3 py-2">收盤</th>
          <th class="text-right px-3 py-2">漲跌%</th>
          <th class="text-right px-3 py-2">量比</th>
          <th class="text-right px-3 py-2">RSI</th>
          <th class="text-center px-3 py-2">趨勢</th>
          <th class="text-center px-3 py-2">訊號</th>
          <th class="text-center px-3 py-2" title="即時監聽（WebSocket），最多 3 檔">監聽</th>
          <th class="px-2 py-2"></th>
        </tr>
      </thead>
      <tbody>
        {#each filteredWatchlist as item (item.symbol)}
          {@const q = $quotes.get(item.symbol)}

          {#if confirmDeleteSymbol === item.symbol}
            <!-- 刪除確認列 -->
            <tr class="border-b border-border/50 bg-red-900/20">
              <td class="px-4 py-2 text-xs text-gray-300" colspan="7">
                確定刪除 <span class="font-semibold text-white">{item.symbol} {item.name}</span>？
              </td>
              <td class="px-2 py-2 text-right" colspan="2">
                <div class="flex gap-2 justify-end">
                  <button
                    class="text-xs px-2.5 py-1 border border-border text-muted hover:text-white rounded transition-colors"
                    on:click={() => confirmDeleteSymbol = ''}
                  >取消</button>
                  <button
                    class="text-xs px-2.5 py-1 bg-red-600 hover:bg-red-500 text-white rounded transition-colors"
                    on:click={() => confirmDelete(item.symbol)}
                  >刪除</button>
                </div>
              </td>
            </tr>
          {:else}
            <!-- 一般資料列 -->
            <tr
              class="border-b border-border/50 hover:bg-border/30 cursor-pointer transition-colors group
                     {$selectedSymbol === item.symbol ? 'bg-indigo-900/30' : ''}"
              on:click={() => selectedSymbol.set(item.symbol)}
            >
              <td class="px-4 py-2">
                <div class="font-medium text-white">{item.symbol}</div>
                <div class="text-xs text-muted truncate max-w-28">{item.name}</div>
                <div class="mt-1 flex items-center gap-1 flex-wrap">
                  <span class="text-[10px] px-1.5 py-0.5 rounded border {stockStatusClass(item)}">{stockStatusLabel(item)}</span>
                  {#if item.stock_symbol?.security_type}
                    <span class="text-[10px] text-muted">{item.stock_symbol.security_type}</span>
                  {/if}
                  {#if item.stock_symbol?.industry || item.sector}
                    <span class="text-[10px] text-muted truncate max-w-24">{item.stock_symbol?.industry || item.sector}</span>
                  {/if}
                </div>
              </td>
              <td class="px-3 py-2 text-right font-mono">
                {q ? formatPrice(q.close) : '-'}
              </td>
              <td class="px-3 py-2 text-right font-mono {q ? priceColor(q.change) : 'text-flat'}">
                {q ? formatChangePct(q.changePct) : '-'}
              </td>
              <td class="px-3 py-2 text-right font-mono {q && q.volRatio >= 2 ? 'text-rise font-semibold' : 'text-gray-300'}">
                {q ? `${q.volRatio.toFixed(1)}x` : '-'}
              </td>
              <td class="px-3 py-2 text-right font-mono
                {q && q.rsi14 >= 70 ? 'text-rise' : q && q.rsi14 <= 30 ? 'text-fall' : 'text-gray-300'}">
                {q ? q.rsi14.toFixed(1) : '-'}
              </td>
              <td class="px-3 py-2 text-center">
                {#if q}
                  <span class="text-xs {trendColor(q.trend)}">{trendLabel[q.trend] ?? '-'}</span>
                {:else}
                  <span class="text-muted">-</span>
                {/if}
              </td>
              <td class="px-3 py-2 text-center">
                {#if q?.hasSignal}
                  <span class="inline-block w-2 h-2 rounded-full bg-rise animate-pulse"></span>
                {/if}
              </td>
              <td class="px-3 py-2 text-center">
                <button
                  class="text-base leading-none transition-colors
                         {item.watched ? 'text-amber-400 hover:text-amber-300' : 'text-muted hover:text-white'}"
                  title={item.watched ? '取消即時監聽' : '設定即時監聽（最多 3 檔）'}
                  on:click|stopPropagation={() => toggleWatch(item)}
                >{item.watched ? '★' : '☆'}</button>
              </td>
              <!-- 操作按鈕：hover 才顯示 -->
              <td class="px-2 py-2">
                <div class="flex gap-1 justify-end opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    class="p-1 text-muted hover:text-indigo-400 transition-colors rounded"
                    title="編輯"
                    on:click|stopPropagation={() => openEdit(item)}
                  >✏</button>
                  <button
                    class="p-1 text-muted hover:text-red-400 transition-colors rounded"
                    title="刪除"
                    on:click|stopPropagation={() => confirmDeleteSymbol = item.symbol}
                  >✕</button>
                </div>
              </td>
            </tr>
          {/if}
        {:else}
          <tr>
            <td colspan="9" class="px-4 py-8 text-center text-muted text-sm">沒有符合條件的監控股票</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
