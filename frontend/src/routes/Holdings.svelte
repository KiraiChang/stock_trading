<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { ApiError } from '../lib/api/client'
  import {
    analyzeHolding,
    createHolding,
    deleteHolding,
    listHoldingAnalyses,
    listHoldings,
    updateHolding,
    type Holding,
    type HoldingAnalysis,
  } from '../lib/api/holdings'

  let holdings: Holding[] = []
  let selectedId: number | null = null
  let analyses: HoldingAnalysis[] = []
  let latest: HoldingAnalysis | null = null
  let loading = true
  let saving = false
  let analyzing: number | null = null
  let error = ''
  let form = { symbol: '', shares: '', cost_price: '', note: '' }
  let editingId: number | null = null

  onMount(() => {
    load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      holdings = await listHoldings()
      if (holdings.length > 0 && selectedId === null) selectedId = holdings[0].id
      if (selectedId !== null) await loadAnalyses(selectedId)
    } catch (err) {
      error = err instanceof ApiError ? err.message : '載入持股失敗'
    } finally {
      loading = false
    }
  }

  async function save() {
    const payload = {
      symbol: form.symbol.trim(),
      shares: Number(form.shares),
      cost_price: Number(form.cost_price),
      note: form.note.trim(),
    }
    if (!payload.symbol || payload.shares <= 0 || payload.cost_price <= 0) {
      error = '請輸入代號、股數與持有成本'
      return
    }
    saving = true
    error = ''
    try {
      const holding = editingId ? await updateHolding(editingId, payload) : await createHolding(payload)
      resetForm()
      selectedId = holding.id
      await load()
    } catch (err) {
      error = err instanceof ApiError ? err.message : '儲存持股失敗'
    } finally {
      saving = false
    }
  }

  function edit(h: Holding) {
    editingId = h.id
    form = {
      symbol: h.symbol,
      shares: String(h.shares),
      cost_price: String(h.cost_price),
      note: h.note ?? '',
    }
  }

  function resetForm() {
    editingId = null
    form = { symbol: '', shares: '', cost_price: '', note: '' }
  }

  async function remove(h: Holding) {
    if (!confirm(`刪除 ${h.symbol} 持股設定？歷史分析快照會保留在資料庫中。`)) return
    error = ''
    try {
      await deleteHolding(h.id)
      if (selectedId === h.id) {
        selectedId = null
        analyses = []
        latest = null
      }
      await load()
    } catch (err) {
      error = err instanceof ApiError ? err.message : '刪除持股失敗'
    }
  }

  async function runAnalysis(h: Holding) {
    analyzing = h.id
    error = ''
    try {
      const res = await analyzeHolding(h.id)
      selectedId = h.id
      latest = res.analysis
      await loadAnalyses(h.id)
    } catch (err) {
      error = err instanceof ApiError ? err.message : '分析失敗，請確認 Python service、SR 模型與歷史資料'
    } finally {
      analyzing = null
    }
  }

  async function loadAnalyses(id: number) {
    selectedId = id
    analyses = await listHoldingAnalyses(id, 20).catch(() => [])
    latest = analyses[0] ?? null
  }

  function money(v: number | null | undefined): string {
    if (v === null || v === undefined) return '—'
    return v.toLocaleString('zh-TW', { maximumFractionDigits: 2 })
  }

  function pct(v: number | null | undefined): string {
    if (v === null || v === undefined) return '—'
    return `${(v * 100).toFixed(2)}%`
  }

  function dt(v: string): string {
    return new Date(v).toLocaleString('zh-TW', { hour12: false })
  }

  const actionClass: Record<string, string> = {
    HOLD: 'bg-blue-900/40 text-blue-300',
    STOP_LOSS: 'bg-red-900/50 text-red-300',
    TAKE_PROFIT: 'bg-green-900/50 text-green-300',
    ADD_ON_BREAKOUT: 'bg-emerald-900/40 text-emerald-300',
    REDUCE: 'bg-yellow-900/40 text-yellow-300',
  }
</script>

<Layout>
  <div class="max-w-6xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">持股操作分析</h1>

    {#if error}
      <p class="text-rise text-sm">{error}</p>
    {/if}

    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      <div class="grid grid-cols-1 md:grid-cols-5 gap-3">
        <input bind:value={form.symbol} placeholder="代號" class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white placeholder:text-muted focus:outline-none focus:border-indigo-500" />
        <input bind:value={form.shares} type="number" min="0" step="1" placeholder="股數" class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white placeholder:text-muted focus:outline-none focus:border-indigo-500" />
        <input bind:value={form.cost_price} type="number" min="0" step="0.01" placeholder="持有成本" class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white placeholder:text-muted focus:outline-none focus:border-indigo-500" />
        <input bind:value={form.note} placeholder="備註" class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white placeholder:text-muted focus:outline-none focus:border-indigo-500" />
        <div class="flex gap-2">
          <button class="flex-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium px-4 py-2 rounded-lg" disabled={saving} on:click={save}>
            {saving ? '儲存中' : editingId ? '更新' : '新增'}
          </button>
          {#if editingId}
            <button class="px-3 py-2 border border-border rounded-lg text-muted hover:text-white text-sm" on:click={resetForm}>取消</button>
          {/if}
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-[1.15fr_0.85fr] gap-4">
      <div class="bg-panel border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-3 border-b border-border flex items-center justify-between">
          <h2 class="text-sm font-semibold text-white">目前持股</h2>
          <button class="text-xs text-muted hover:text-white" on:click={load}>重新整理</button>
        </div>
        <table class="w-full text-sm">
          <thead>
            <tr class="text-muted text-xs border-b border-border">
              <th class="text-left px-5 py-3">代號</th>
              <th class="text-right px-3 py-3">股數</th>
              <th class="text-right px-3 py-3">成本</th>
              <th class="text-left px-3 py-3">備註</th>
              <th class="text-center px-5 py-3">操作</th>
            </tr>
          </thead>
          <tbody>
            {#if loading}
              <tr><td colspan="5" class="px-5 py-8 text-center text-muted">載入中...</td></tr>
            {:else if holdings.length === 0}
              <tr><td colspan="5" class="px-5 py-8 text-center text-muted">尚無持股</td></tr>
            {:else}
              {#each holdings as h (h.id)}
                <tr class="border-b border-border/50 hover:bg-border/20">
                  <td class="px-5 py-3">
                    <button class="text-white font-mono hover:text-indigo-300" on:click={() => loadAnalyses(h.id)}>{h.symbol}</button>
                  </td>
                  <td class="px-3 py-3 text-right text-white">{money(h.shares)}</td>
                  <td class="px-3 py-3 text-right text-white">{money(h.cost_price)}</td>
                  <td class="px-3 py-3 text-muted">{h.note || '—'}</td>
                  <td class="px-5 py-3">
                    <div class="flex justify-center gap-2">
                      <button class="text-xs px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-40" disabled={analyzing === h.id} on:click={() => runAnalysis(h)}>
                        {analyzing === h.id ? '分析中' : '分析'}
                      </button>
                      <button class="text-xs px-2 py-1.5 rounded-lg border border-border text-muted hover:text-white" on:click={() => edit(h)}>編輯</button>
                      <button class="text-xs px-2 py-1.5 rounded-lg border border-fall/40 text-fall hover:bg-fall/10" on:click={() => remove(h)}>刪除</button>
                    </div>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      <div class="bg-panel border border-border rounded-xl px-5 py-4 space-y-4">
        <h2 class="text-sm font-semibold text-white">最新分析</h2>
        {#if latest}
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-white font-mono">{latest.symbol}</p>
              <p class="text-xs text-muted">{dt(latest.created_at)}</p>
            </div>
            <span class="px-3 py-1 rounded-full text-xs font-medium {actionClass[latest.action] ?? 'bg-gray-700/60 text-gray-300'}">{latest.action_label}</span>
          </div>
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">現價</p>
              <p class="text-white font-medium">{money(latest.current_price)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">未實現損益</p>
              <p class="text-white font-medium">{money(latest.unrealized_pnl)} ({pct(latest.unrealized_pnl_pct)})</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">停損價</p>
              <p class="text-white font-medium">{money(latest.stop_loss_price)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">停損金額</p>
              <p class="text-white font-medium">{money(latest.stop_loss_amount)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">停利價</p>
              <p class="text-white font-medium">{money(latest.take_profit_price)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">停利金額</p>
              <p class="text-white font-medium">{money(latest.take_profit_amount)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">加碼觸發</p>
              <p class="text-white font-medium">{money(latest.add_on_trigger_price)}</p>
            </div>
            <div class="bg-surface border border-border rounded-lg px-3 py-2">
              <p class="text-xs text-muted">加碼金額</p>
              <p class="text-white font-medium">{money(latest.add_on_amount)}</p>
            </div>
          </div>
          <div class="space-y-1">
            {#each latest.reason as r}
              <p class="text-xs text-muted">• {r}</p>
            {/each}
          </div>
        {:else}
          <p class="text-muted text-sm">選擇持股並按下分析後，這裡會顯示最新操作建議。</p>
        {/if}
      </div>
    </div>

    <div class="bg-panel border border-border rounded-xl overflow-hidden">
      <div class="px-5 py-3 border-b border-border">
        <h2 class="text-sm font-semibold text-white">分析歷史</h2>
      </div>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-muted text-xs border-b border-border">
            <th class="text-left px-5 py-3">時間</th>
            <th class="text-left px-3 py-3">代號</th>
            <th class="text-center px-3 py-3">建議</th>
            <th class="text-right px-3 py-3">現價</th>
            <th class="text-right px-3 py-3">損益</th>
            <th class="text-right px-5 py-3">SR ID</th>
          </tr>
        </thead>
        <tbody>
          {#if analyses.length === 0}
            <tr><td colspan="6" class="px-5 py-8 text-center text-muted">尚無分析歷史</td></tr>
          {:else}
            {#each analyses as a (a.id)}
              <tr class="border-b border-border/50 hover:bg-border/20 cursor-pointer" on:click={() => latest = a}>
                <td class="px-5 py-3 text-muted text-xs">{dt(a.created_at)}</td>
                <td class="px-3 py-3 text-white font-mono">{a.symbol}</td>
                <td class="px-3 py-3 text-center">
                  <span class="px-2 py-0.5 rounded-full text-xs {actionClass[a.action] ?? 'bg-gray-700/60 text-gray-300'}">{a.action_label}</span>
                </td>
                <td class="px-3 py-3 text-right text-white">{money(a.current_price)}</td>
                <td class="px-3 py-3 text-right {a.unrealized_pnl >= 0 ? 'text-rise' : 'text-fall'}">{money(a.unrealized_pnl)}</td>
                <td class="px-5 py-3 text-right text-muted font-mono">{a.sr_zone_analysis_id ?? '—'}</td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</Layout>
