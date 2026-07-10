<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { ApiError } from '../lib/api/client'
  import {
    addPositionTransaction, adjustPosition, getPosition,
    listPositionTransactions, listPositions,
    type Position, type PositionAnalysis, type PositionTransaction,
  } from '../lib/api/positions'
  import { analyzeTrade, listTradeAnalyses } from '../lib/api/tradeAnalysis'

  let symbol = ''
  let positions: Position[] = []
  let current: Position | null = null
  let transactions: PositionTransaction[] = []
  let analyses: PositionAnalysis[] = []
  let latest: PositionAnalysis | null = null
  let error = ''
  let busy = false
  let eventType: 'BUY' | 'SELL' = 'BUY'
  let trade = { shares: '', price: '', fee: '0', tax: '0', note: '' }
  let adjustment = { shares: '', avgCost: '', reason: '' }

  onMount(loadPositions)

  async function loadPositions() {
    positions = await listPositions().catch(() => [])
  }

  async function select(raw: string) {
    symbol = raw.trim().toUpperCase()
    if (!symbol) return
    error = ''
    try {
      current = await getPosition(symbol)
      transactions = await listPositionTransactions(symbol)
      analyses = await listTradeAnalyses(symbol)
      latest = analyses[0] ?? null
      adjustment = {
        shares: String(current.shares),
        avgCost: String(current.avg_cost),
        reason: '',
      }
    } catch (err) {
      error = err instanceof ApiError ? err.message : '載入 Position 失敗'
    }
  }

  async function saveTrade() {
    if (!current) await select(symbol)
    if (!current) return
    busy = true
    error = ''
    try {
      current = await addPositionTransaction(symbol, {
        event_type: eventType,
        shares: Number(trade.shares),
        price: Number(trade.price),
        fee: Number(trade.fee || 0),
        tax: Number(trade.tax || 0),
        expected_version: current.version,
        note: trade.note.trim(),
      })
      trade = { shares: '', price: '', fee: '0', tax: '0', note: '' }
      await select(symbol)
      await loadPositions()
    } catch (err) {
      error = err instanceof ApiError ? err.message : '交易寫入失敗'
    } finally {
      busy = false
    }
  }

  async function saveAdjustment() {
    if (!current || !adjustment.reason.trim()) {
      error = 'ADJUSTMENT 必須填寫更正原因'
      return
    }
    busy = true
    try {
      current = await adjustPosition(symbol, {
        target_shares: Number(adjustment.shares),
        target_avg_cost: Number(adjustment.avgCost),
        expected_version: current.version,
        reason: adjustment.reason.trim(),
      })
      await select(symbol)
      await loadPositions()
    } catch (err) {
      error = err instanceof ApiError ? err.message : '更正失敗'
    } finally {
      busy = false
    }
  }

  async function runAnalysis(forceRefresh = false) {
    symbol = symbol.trim().toUpperCase()
    if (!symbol) return
    busy = true
    error = ''
    try {
      const response = await analyzeTrade(symbol, forceRefresh)
      latest = response.analysis
      await select(symbol)
    } catch (err) {
      error = err instanceof ApiError ? err.message : '分析失敗'
    } finally {
      busy = false
    }
  }

  function num(value?: number | null): string {
    return value == null ? '—' : value.toLocaleString('zh-TW', { maximumFractionDigits: 2 })
  }
  function pct(value?: number | null): string {
    return value == null ? '—' : `${(value * 100).toFixed(2)}%`
  }
  function dt(value: string): string {
    return new Date(value).toLocaleString('zh-TW', { hour12: false })
  }
  const actionClass: Record<string, string> = {
    ENTER: 'bg-green-900/50 text-green-300', ENTER_SMALL: 'bg-emerald-900/40 text-emerald-300',
    WAIT: 'bg-gray-700/60 text-gray-300', AVOID: 'bg-red-900/40 text-red-300',
    HOLD: 'bg-blue-900/40 text-blue-300', ADD: 'bg-green-900/50 text-green-300',
    REDUCE: 'bg-yellow-900/40 text-yellow-300', TAKE_PROFIT: 'bg-emerald-900/40 text-emerald-300',
    EXIT_STOP: 'bg-red-900/50 text-red-300',
  }
</script>

<Layout>
  <div class="max-w-6xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">交易分析</h1>
    {#if error}<p class="text-rise text-sm">{error}</p>{/if}

    <div class="bg-panel border border-border rounded-xl p-4 flex gap-3 flex-wrap">
      <input bind:value={symbol} placeholder="股票代號，例如 2330" on:keydown={(e) => e.key === 'Enter' && select(symbol)}
        class="flex-1 min-w-[180px] bg-surface border border-border rounded-lg px-3 py-2 text-white" />
      <button class="px-4 py-2 bg-indigo-600 rounded-lg text-white" on:click={() => select(symbol)}>載入</button>
      <button class="px-4 py-2 bg-green-700 rounded-lg text-white disabled:opacity-50" disabled={busy} on:click={() => runAnalysis(false)}>分析</button>
      <button class="px-4 py-2 border border-border rounded-lg text-muted disabled:opacity-50" disabled={busy} on:click={() => runAnalysis(true)}>強制刷新 SR</button>
    </div>

    <div class="grid lg:grid-cols-3 gap-4">
      <div class="bg-panel border border-border rounded-xl p-4">
        <h2 class="text-white text-sm font-semibold mb-3">目前持股狀態</h2>
        {#if current}
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div><p class="text-muted text-xs">狀態</p><p class="text-white">{current.shares > 0 ? 'LONG' : 'FLAT'}</p></div>
            <div><p class="text-muted text-xs">版本</p><p class="text-white">{current.version}</p></div>
            <div><p class="text-muted text-xs">股數</p><p class="text-white">{num(current.shares)}</p></div>
            <div><p class="text-muted text-xs">AVG 成本</p><p class="text-white">{num(current.avg_cost)}</p></div>
            <div class="col-span-2"><p class="text-muted text-xs">已實現損益</p><p class={current.realized_pnl >= 0 ? 'text-rise' : 'text-fall'}>{num(current.realized_pnl)}</p></div>
          </div>
        {:else}<p class="text-muted text-sm">輸入股票代號載入；沒有交易時視為 FLAT。</p>{/if}
      </div>

      <div class="bg-panel border border-border rounded-xl p-4 space-y-2">
        <h2 class="text-white text-sm font-semibold">新增交易</h2>
        <div class="grid grid-cols-2 gap-2">
          <select bind:value={eventType} class="bg-surface border border-border rounded px-2 py-2 text-white"><option>BUY</option><option>SELL</option></select>
          <input bind:value={trade.shares} type="number" placeholder="股數" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.price} type="number" placeholder="價格" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.fee} type="number" placeholder="手續費" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.tax} type="number" placeholder="交易稅" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.note} placeholder="備註" class="bg-surface border border-border rounded px-2 py-2 text-white" />
        </div>
        <button class="w-full bg-indigo-600 text-white rounded py-2 disabled:opacity-50" disabled={busy || !current} on:click={saveTrade}>寫入不可變交易</button>
      </div>

      <div class="bg-panel border border-border rounded-xl p-4 space-y-2">
        <h2 class="text-white text-sm font-semibold">ADJUSTMENT</h2>
        <input bind:value={adjustment.shares} type="number" placeholder="更正後股數" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.avgCost} type="number" placeholder="更正後 AVG" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.reason} placeholder="必填：更正原因" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <button class="w-full border border-yellow-700 text-yellow-300 rounded py-2 disabled:opacity-50" disabled={busy || !current} on:click={saveAdjustment}>新增更正事件</button>
      </div>
    </div>

    <div class="bg-panel border border-border rounded-xl p-4">
      <h2 class="text-white text-sm font-semibold mb-3">最新分析</h2>
      {#if latest}
        <div class="flex justify-between gap-3 mb-4">
          <div><p class="text-white font-mono">{latest.symbol} · {latest.position_state}</p><p class="text-muted text-xs">{dt(latest.created_at)}</p></div>
          <span class="px-3 py-1 rounded-full text-xs {actionClass[latest.action]}">{latest.action_label}</span>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
          <div><p class="text-muted text-xs">目前 → 目標</p><p class="text-white">{num(latest.shares)} → {num(latest.target_shares)}</p></div>
          <div><p class="text-muted text-xs">本次調整</p><p class="text-white">{latest.adjustment_side} {num(Math.abs(latest.adjustment_shares))}</p></div>
          <div><p class="text-muted text-xs">預估金額</p><p class="text-white">{num(latest.adjustment_amount)}</p></div>
          <div><p class="text-muted text-xs">RR</p><p class="text-white">{num(latest.risk_reward_ratio)}</p></div>
          <div><p class="text-muted text-xs">停損</p><p class="text-white">{num(latest.stop_loss_price)}</p></div>
          <div><p class="text-muted text-xs">停利</p><p class="text-white">{num(latest.take_profit_price)}</p></div>
          <div><p class="text-muted text-xs">風險金額</p><p class="text-white">{num(latest.risk_amount)}</p></div>
          <div><p class="text-muted text-xs">未實現損益</p><p class={latest.unrealized_pnl >= 0 ? 'text-rise' : 'text-fall'}>{num(latest.unrealized_pnl)} ({pct(latest.unrealized_pnl_pct)})</p></div>
        </div>
        <div class="mt-3">{#each latest.reason as reason}<p class="text-muted text-xs">• {reason}</p>{/each}</div>
      {:else}<p class="text-muted text-sm">尚無分析；空手股票也可直接執行分析。</p>{/if}
    </div>

    <div class="grid lg:grid-cols-2 gap-4">
      <div class="bg-panel border border-border rounded-xl p-4">
        <h2 class="text-white text-sm font-semibold mb-3">不可變交易流水</h2>
        <div class="space-y-2 max-h-80 overflow-auto">
          {#each transactions as tx}
            <div class="border border-border rounded p-2 text-xs flex justify-between">
              <span class="text-white">{tx.event_type} {num(tx.shares ?? tx.target_shares)} @ {num(tx.price ?? tx.target_avg_cost)}</span>
              <span class="text-muted">{dt(tx.occurred_at)}</span>
            </div>
          {/each}
        </div>
      </div>
      <div class="bg-panel border border-border rounded-xl p-4">
        <h2 class="text-white text-sm font-semibold mb-3">分析歷史</h2>
        <div class="space-y-2 max-h-80 overflow-auto">
          {#each analyses as item}
            <button class="w-full border border-border rounded p-2 text-xs flex justify-between" on:click={() => latest = item}>
              <span class="text-white">{item.action_label} · 目標 {num(item.target_shares)}</span>
              <span class="text-muted">{dt(item.created_at)}</span>
            </button>
          {/each}
        </div>
      </div>
    </div>

    {#if positions.length > 0}
      <div class="text-xs text-muted">現有 LONG：{positions.map((p) => p.symbol).join('、')}</div>
    {/if}
  </div>
</Layout>
