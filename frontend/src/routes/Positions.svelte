<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { ApiError } from '../lib/api/client'
  import {
    addPositionTransaction, adjustPosition, getPosition,
    listPositionTransactions, listPositions,
    type Position, type PositionAnalysis, type PositionTransaction,
  } from '../lib/api/positions'
  import { createPortfolio, listPortfolios, type Portfolio } from '../lib/api/portfolios'
  import { analyzeTrade, listTradeAnalyses } from '../lib/api/tradeAnalysis'
  import { derivedReasonLabel } from '../lib/api/srZones'
  import { selectedPortfolioID } from '../lib/stores/portfolio'

  let symbol = ''
  let portfolios: Portfolio[] = []
  let creatingPortfolio = false
  let newPortfolioName = ''
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

  $: selectedPortfolio = portfolios.find((p) => p.id === $selectedPortfolioID) ?? null
  // portfolio 尚未 resolve（清單未載入、或選到無權存取的 id）時保守停用寫入，
  // 交由 server 的 requirePortfolioAccess 作最終真相源。
  $: canWritePortfolio = selectedPortfolio?.can_write ?? false

  onMount(async () => {
    await loadPortfolioOptions()
    await loadPositions()
  })

  async function loadPortfolioOptions() {
    try {
      portfolios = await listPortfolios()
      if (portfolios.length > 0 && !portfolios.some((p) => p.id === $selectedPortfolioID)) {
        selectedPortfolioID.set(portfolios[0].id)
      }
    } catch (err) {
      error = err instanceof ApiError ? err.message : '載入 Portfolio 失敗'
    }
  }

  async function loadPositions() {
    positions = await listPositions($selectedPortfolioID).catch(() => [])
  }

  async function changePortfolio(raw: string) {
    const nextID = Number(raw)
    if (!nextID || nextID === $selectedPortfolioID) return
    selectedPortfolioID.set(nextID)
    current = null
    transactions = []
    analyses = []
    latest = null
    await loadPositions()
    if (symbol.trim()) await select(symbol)
  }

  function changePortfolioFromEvent(event: Event) {
    const target = event.currentTarget as HTMLSelectElement
    void changePortfolio(target.value)
  }

  async function savePortfolio() {
    const name = newPortfolioName.trim()
    if (!name) return
    creatingPortfolio = true
    error = ''
    try {
      const created = await createPortfolio({ name })
      newPortfolioName = ''
      await loadPortfolioOptions()
      selectedPortfolioID.set(created.id)
      await loadPositions()
      if (symbol.trim()) await select(symbol)
    } catch (err) {
      error = err instanceof ApiError ? err.message : '建立 Portfolio 失敗'
    } finally {
      creatingPortfolio = false
    }
  }

  async function select(raw: string) {
    symbol = raw.trim().toUpperCase()
    if (!symbol) return
    error = ''
    try {
      current = await getPosition(symbol, $selectedPortfolioID)
      transactions = await listPositionTransactions(symbol, $selectedPortfolioID)
      analyses = await listTradeAnalyses(symbol, $selectedPortfolioID)
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
    if (!canWritePortfolio) {
      error = '目前 Portfolio 沒有寫入權限'
      return
    }
    if (!current) await select(symbol)
    if (!current) return
    busy = true
    error = ''
    try {
      current = await addPositionTransaction(symbol, {
        portfolio_id: $selectedPortfolioID,
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
    if (!canWritePortfolio) {
      error = '目前 Portfolio 沒有寫入權限'
      return
    }
    if (!current || !adjustment.reason.trim()) {
      error = 'ADJUSTMENT 必須填寫更正原因'
      return
    }
    busy = true
    try {
      current = await adjustPosition(symbol, {
        portfolio_id: $selectedPortfolioID,
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
    if (!canWritePortfolio) {
      error = '目前 Portfolio 沒有寫入權限'
      return
    }
    symbol = symbol.trim().toUpperCase()
    if (!symbol) return
    busy = true
    error = ''
    try {
      const response = await analyzeTrade(symbol, $selectedPortfolioID, forceRefresh)
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
  function conditionLabel(latest: PositionAnalysis): string {
    if (latest.action === 'HOLD' && latest.evidence?.position_action_condition) return '條件式持有'
    return latest.action_label
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

    <div class="bg-panel border border-border rounded-xl p-4 space-y-3">
      <div class="flex gap-3 flex-wrap">
        <select value={$selectedPortfolioID} on:change={changePortfolioFromEvent}
          class="min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-white">
          {#each portfolios as portfolio}
            <option value={portfolio.id}>{portfolio.name} · {portfolio.owner_type}{portfolio.can_write ? '' : ' · READ'}</option>
          {/each}
        </select>
        <input bind:value={newPortfolioName} placeholder="新增個人 Portfolio"
          class="flex-1 min-w-[180px] bg-surface border border-border rounded-lg px-3 py-2 text-white" />
        <button class="px-4 py-2 border border-border rounded-lg text-muted disabled:opacity-50"
          disabled={creatingPortfolio || !newPortfolioName.trim()} on:click={savePortfolio}>建立</button>
      </div>
      {#if selectedPortfolio && !selectedPortfolio.can_write}
        <p class="text-yellow-300 text-xs">目前 Portfolio 為唯讀，已停用交易、更正與分析寫入。</p>
      {/if}
    </div>

    <div class="bg-panel border border-border rounded-xl p-4 flex gap-3 flex-wrap">
      <input bind:value={symbol} placeholder="股票代號，例如 2330" on:keydown={(e) => e.key === 'Enter' && select(symbol)}
        class="flex-1 min-w-[180px] bg-surface border border-border rounded-lg px-3 py-2 text-white" />
      <button class="px-4 py-2 bg-indigo-600 rounded-lg text-white" on:click={() => select(symbol)}>載入</button>
      <button class="px-4 py-2 bg-green-700 rounded-lg text-white disabled:opacity-50" disabled={busy || !canWritePortfolio} on:click={() => runAnalysis(false)}>分析</button>
      <button class="px-4 py-2 border border-border rounded-lg text-muted disabled:opacity-50" disabled={busy || !canWritePortfolio} on:click={() => runAnalysis(true)}>強制刷新 SR</button>
    </div>

    <div class="grid lg:grid-cols-3 gap-4">
      <div class="bg-panel border border-border rounded-xl p-4">
        <h2 class="text-white text-sm font-semibold mb-3">目前持股狀態</h2>
        {#if current}
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div><p class="text-muted text-xs">狀態</p><p class="text-white">{current.shares > 0 ? 'LONG' : 'FLAT'}</p></div>
            <div><p class="text-muted text-xs">Portfolio</p><p class="text-white">{current.portfolio_id}</p></div>
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
        <button class="w-full bg-indigo-600 text-white rounded py-2 disabled:opacity-50" disabled={busy || !current || !canWritePortfolio} on:click={saveTrade}>寫入不可變交易</button>
      </div>

      <div class="bg-panel border border-border rounded-xl p-4 space-y-2">
        <h2 class="text-white text-sm font-semibold">ADJUSTMENT</h2>
        <input bind:value={adjustment.shares} type="number" placeholder="更正後股數" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.avgCost} type="number" placeholder="更正後 AVG" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.reason} placeholder="必填：更正原因" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <button class="w-full border border-yellow-700 text-yellow-300 rounded py-2 disabled:opacity-50" disabled={busy || !current || !canWritePortfolio} on:click={saveAdjustment}>新增更正事件</button>
      </div>
    </div>

    <div class="bg-panel border border-border rounded-xl p-4">
      <h2 class="text-white text-sm font-semibold mb-3">最新分析</h2>
      {#if latest}
        <div class="flex justify-between gap-3 mb-4">
          <div><p class="text-white font-mono">{latest.symbol} · {latest.position_state}</p><p class="text-muted text-xs">Portfolio {latest.portfolio_id} · {dt(latest.created_at)}</p></div>
          <span class="px-3 py-1 rounded-full text-xs {actionClass[latest.action]}">{conditionLabel(latest)}</span>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
          <div><p class="text-muted text-xs">目前 → 目標</p><p class="text-white">{num(latest.shares)} → {num(latest.target_shares)}</p></div>
          <div><p class="text-muted text-xs">本次調整</p><p class="text-white">{latest.adjustment_side} {num(Math.abs(latest.adjustment_shares))}</p></div>
          <div><p class="text-muted text-xs">預估金額</p><p class="text-white">{num(latest.adjustment_amount)}</p></div>
          <div><p class="text-muted text-xs">Market RR</p><p class="text-white">{num(latest.evidence?.rr?.market_rr ?? latest.risk_reward_ratio)}</p></div>
          <div><p class="text-muted text-xs">Position RR</p><p class="text-white">{num(latest.evidence?.rr?.position_rr)}</p></div>
          <div><p class="text-muted text-xs">Defense Price</p><p class="text-white">{num(latest.evidence?.stops?.defense_price ?? latest.stop_loss_price)}</p></div>
          <div><p class="text-muted text-xs">Structural Stop</p><p class="text-white">{num(latest.evidence?.stops?.structural_stop ?? latest.stop_loss_price)}</p></div>
          <div><p class="text-muted text-xs">停利</p><p class="text-white">{num(latest.take_profit_price)}</p></div>
          <div><p class="text-muted text-xs">風險金額</p><p class="text-white">{num(latest.risk_amount)}</p></div>
          <div><p class="text-muted text-xs">未實現損益</p><p class={latest.unrealized_pnl >= 0 ? 'text-rise' : 'text-fall'}>{num(latest.unrealized_pnl)} ({pct(latest.unrealized_pnl_pct)})</p></div>
        </div>
        <div class="mt-4 grid md:grid-cols-4 gap-3 text-xs">
          <div class="border border-border rounded p-2">
            <p class="text-muted">Risk Budget</p>
            <p class="text-white font-mono">{num(latest.evidence?.risk_sizing?.risk_budget)}</p>
          </div>
          <div class="border border-border rounded p-2">
            <p class="text-muted">Per Share Risk</p>
            <p class="text-white font-mono">{num(latest.evidence?.risk_sizing?.per_share_risk)}</p>
          </div>
          <div class="border border-border rounded p-2">
            <p class="text-muted">Max Shares</p>
            <p class="text-white font-mono">{num(latest.evidence?.risk_sizing?.max_shares)}</p>
          </div>
          <div class="border border-border rounded p-2">
            <p class="text-muted">Excess Shares</p>
            <p class="text-white font-mono">{num(latest.evidence?.risk_sizing?.excess_shares)}</p>
          </div>
        </div>
        <div class="mt-4 grid md:grid-cols-2 gap-3 text-xs">
          <div class="border border-border rounded p-3">
            <div class="flex items-center justify-between gap-2 mb-2">
              <p class="text-muted">空手決策</p>
              <span class="px-2 py-0.5 rounded {latest.evidence?.entry_decision?.applicable ? 'bg-green-900/40 text-green-300' : 'bg-gray-700/60 text-gray-300'}">
                {latest.evidence?.entry_decision?.applicable ? '適用' : '不適用'}
              </span>
            </div>
            <p class="text-white font-semibold">{latest.evidence?.entry_decision?.label ?? '—'}</p>
            <div class="grid grid-cols-2 gap-2 mt-2">
              <div><p class="text-muted">State</p><p class="text-white font-mono">{latest.evidence?.entry_decision?.state ?? '—'}</p></div>
              <div><p class="text-muted">Target</p><p class="text-white font-mono">{num(latest.evidence?.entry_decision?.target_shares)}</p></div>
              <div><p class="text-muted">Entry RR</p><p class="text-white font-mono">{num(latest.evidence?.entry_decision?.market_rr)}</p></div>
              <div><p class="text-muted">Stop</p><p class="text-white font-mono">{num(latest.evidence?.entry_decision?.stop_loss_price)}</p></div>
            </div>
            <p class="text-muted mt-2 break-words">{latest.evidence?.entry_decision?.reason_codes?.join(' / ') || '—'}</p>
          </div>
          <div class="border border-border rounded p-3">
            <div class="flex items-center justify-between gap-2 mb-2">
              <p class="text-muted">持有決策</p>
              <span class="px-2 py-0.5 rounded {latest.evidence?.position_decision?.applicable ? 'bg-blue-900/40 text-blue-300' : 'bg-gray-700/60 text-gray-300'}">
                {latest.evidence?.position_decision?.applicable ? '適用' : '不適用'}
              </span>
            </div>
            <p class="text-white font-semibold">{latest.evidence?.position_decision?.label ?? '—'}</p>
            <div class="grid grid-cols-2 gap-2 mt-2">
              <div><p class="text-muted">State</p><p class="text-white font-mono">{latest.evidence?.position_decision?.state ?? '—'}</p></div>
              <div><p class="text-muted">Target</p><p class="text-white font-mono">{num(latest.evidence?.position_decision?.target_shares)}</p></div>
              <div><p class="text-muted">Position RR</p><p class="text-white font-mono">{num(latest.evidence?.position_decision?.position_rr)}</p></div>
              <div><p class="text-muted">Defense</p><p class="text-white font-mono">{num(latest.evidence?.position_decision?.defense_price)}</p></div>
            </div>
            <p class="text-muted mt-2 break-words">{latest.evidence?.position_decision?.reason_codes?.join(' / ') || '—'}</p>
          </div>
        </div>
        {#if latest.evidence?.position_action_condition}
          <div class="mt-3 grid md:grid-cols-3 gap-3 text-xs">
            <div><p class="text-muted">防守線</p><p class="text-fall font-mono">{num(latest.evidence.position_action_condition.invalidation_price)}</p></div>
            <div><p class="text-muted">回穩線</p><p class="text-rise font-mono">{num(latest.evidence.position_action_condition.recovery_price)}</p></div>
            <div><p class="text-muted">Reason Codes</p><p class="text-white font-mono break-words">{latest.evidence.position_action_condition.reason_codes?.map(derivedReasonLabel).join(' / ') || '—'}</p></div>
          </div>
        {/if}
        <div class="mt-3 grid md:grid-cols-3 gap-3 text-xs">
          <div><p class="text-muted">Realized P&L Impact</p><p class={(latest.evidence?.pnl_impact?.realized_delta ?? 0) >= 0 ? 'text-rise' : 'text-fall'}>{num(latest.evidence?.pnl_impact?.realized_delta)}</p></div>
          <div><p class="text-muted">Unrealized Before</p><p class={(latest.evidence?.pnl_impact?.unrealized_before ?? 0) >= 0 ? 'text-rise' : 'text-fall'}>{num(latest.evidence?.pnl_impact?.unrealized_before)}</p></div>
          <div><p class="text-muted">Unrealized After</p><p class={(latest.evidence?.pnl_impact?.unrealized_after ?? 0) >= 0 ? 'text-rise' : 'text-fall'}>{num(latest.evidence?.pnl_impact?.unrealized_after)}</p></div>
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
