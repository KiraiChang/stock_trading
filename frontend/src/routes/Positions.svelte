<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { ApiError } from '../lib/api/client'
  import {
    addPositionTransaction, adjustPosition, getPosition,
    listPositionTransactions, listPositions,
    type Position, type PositionTransaction,
  } from '../lib/api/positions'
  import { createPortfolio, listPortfolios, type Portfolio } from '../lib/api/portfolios'
  import { analyzeTrade, listTradeAnalyses, type TradeDataStatus, type TradePlan } from '../lib/api/tradeAnalysis'
  import { derivedReasonLabel } from '../lib/api/srZones'
  import { selectedPortfolioID } from '../lib/stores/portfolio'
  import { selectedSymbol } from '../lib/stores/market'
  import {
    mdhm, pickSummaryPlan, shortDate, tradePlanStatus, type TradeAnalysisOutcome,
  } from '../lib/utils/tradePlanStatus'

  // ── T-089 D5：請求鏈固定選擇 ─────────────────────────────────
  // 每條請求鏈（載入：持倉 → 流水 → 歷史；分析；買賣與更正）在開始時取一份「選擇」（投組 id＋代號）與遞增的序號，
  // 鏈裡每一步都用這份選擇發請求（⛔ 不再讀全域的 symbol／$selectedPortfolioID）。回應要套用到畫面之前先比對：
  // 這份選擇等於目前的選擇、而且序號是這個選擇最新的；⛔ 不符合就丟掉。busy、錯誤、分析結果都依鍵記錄。
  type Selection = { portfolioID: number; symbol: string }
  type Busy = 'analyzing' | 'writing'
  const keyOf = (s: Selection) => `${s.portfolioID}|${s.symbol}`

  let symbolInput = ''
  let sel: Selection | null = null
  let seq = 0
  let latestLoadSeq: Record<string, number> = {}
  let loadingKey = ''
  let busyByKey: Record<string, Busy | undefined> = {}
  // 錯誤分兩種，都依鍵記錄：載入錯誤（重新載入成功就清掉）、動作錯誤（分析、買賣、更正；同一個鍵下一次動作開始時才清掉，
  // ⛔ 不因為回到這個鍵重新載入就消失——「回到 A 才顯示更新失敗／更正失敗」）
  let loadErrorByKey: Record<string, string | undefined> = {}
  let actionErrorByKey: Record<string, string | undefined> = {}
  let outcomeByKey: Record<string, TradeAnalysisOutcome | undefined> = {}
  // 買賣或更正寫入成功時的序號（review 2026-11-26 [P1]）：之後開始的載入成功才清掉；在那之前摘要⛔ 不能是「最新」
  let writtenSeqByKey: Record<string, number | undefined> = {}
  let positionsSeq = 0

  let portfolios: Portfolio[] = []
  let portfoliosLoaded = false
  let creatingPortfolio = false
  let newPortfolioName = ''
  let positions: Position[] = []
  let pageError = ''

  // 目前選擇的畫面內容（切換時先清掉，⛔ 不保留上一檔的內容）
  let current: Position | null = null
  let transactions: PositionTransaction[] = []
  let analyses: TradePlan[] = []
  let dataStatus: TradeDataStatus | null = null
  let viewing: TradePlan | null = null // 使用者從分析歷史點選的那一筆

  let eventType: 'BUY' | 'SELL' = 'BUY'
  const emptyTrade = () => ({ shares: '', price: '', fee: '0', tax: '0', note: '' })
  const emptyAdjustment = () => ({ shares: '', avgCost: '', reason: '' })
  let trade = emptyTrade()
  let adjustment = emptyAdjustment()

  $: selectedPortfolio = portfolios.find((p) => p.id === $selectedPortfolioID) ?? null
  $: hasSelectedPortfolio = $selectedPortfolioID > 0 && selectedPortfolio !== null
  // portfolio 尚未 resolve（清單未載入、或選到無權存取的 id）時保守停用寫入，
  // 交由 server 的 requirePortfolioAccess 作最終真相源。
  $: canWritePortfolio = hasSelectedPortfolio && (selectedPortfolio?.can_write ?? false)

  $: curKey = sel ? keyOf(sel) : ''
  $: curBusy = curKey ? busyByKey[curKey] : undefined
  $: curError = curKey ? [actionErrorByKey[curKey], loadErrorByKey[curKey]].filter(Boolean).join('；') : ''
  $: curOutcome = curKey ? outcomeByKey[curKey] ?? null : null
  $: loading = curKey !== '' && loadingKey === curKey
  $: summaryPlan = pickSummaryPlan(analyses)
  $: shownPlan = viewing ?? summaryPlan
  $: status = sel && !loading
    ? tradePlanStatus({
      plan: shownPlan, dataStatus, lastAnalysis: curOutcome,
      viewingHistory: viewing !== null && viewing.id !== summaryPlan?.id,
      positionWrittenSinceLoad: writtenSeqByKey[curKey] !== undefined,
    })
    : null
  // 細節區塊照原樣顯示目前的那一筆；資料不足時⛔ 不顯示判斷（除非使用者自己從歷史點選）
  $: latest = status && (status.showJudgement || viewing) ? shownPlan : null

  function isCurrent(s: Selection): boolean {
    return sel !== null && keyOf(sel) === keyOf(s)
  }
  function setBusy(k: string, v: Busy | undefined) { busyByKey = { ...busyByKey, [k]: v } }
  function setError(k: string, v: string | undefined) { actionErrorByKey = { ...actionErrorByKey, [k]: v } }
  function setLoadError(k: string, v: string | undefined) { loadErrorByKey = { ...loadErrorByKey, [k]: v } }
  function setOutcome(k: string, v: TradeAnalysisOutcome | undefined) { outcomeByKey = { ...outcomeByKey, [k]: v } }
  function markWritten(k: string) { writtenSeqByKey = { ...writtenSeqByKey, [k]: ++seq } }

  onMount(async () => {
    await loadPortfolioOptions()
    await loadPositions()
    // T-089 D4：從觀察清單按「計畫」進來時帶入那一檔，自動載入持倉與分析歷史（只讀、⛔ 不自動分析，Q1）
    const preset = $selectedSymbol.trim().toUpperCase()
    // 開頁期間使用者已經自己選了別檔時⛔ 不覆蓋（review 2026-11-26 [P2]）
    if (preset && !sel) {
      symbolInput = preset
      if ($selectedPortfolioID > 0) await select(preset)
    }
  })

  async function loadPortfolioOptions() {
    try {
      portfolios = await listPortfolios()
      if (portfolios.length > 0 && !portfolios.some((p) => p.id === $selectedPortfolioID)) {
        selectedPortfolioID.set(portfolios[0].id)
      } else if (portfolios.length === 0) {
        selectedPortfolioID.set(0)
      }
    } catch (err) {
      pageError = err instanceof ApiError ? err.message : '載入 Portfolio 失敗'
    } finally {
      portfoliosLoaded = true
    }
  }

  // 持倉清單也帶投組與序號：晚回來的舊投組清單⛔ 不蓋掉新的（review 2026-11-26 [P2]）
  // 頁尾清單屬於哪一個投組：換投組時立即清空，⛔ 不在新清單回來前暫時顯示舊投組的股票（review 2026-11-26 [P3]）
  let positionsPortfolioID = 0
  async function loadPositions() {
    const portfolioID = $selectedPortfolioID
    const mySeq = ++positionsSeq
    if (portfolioID !== positionsPortfolioID) {
      positions = []
      positionsPortfolioID = portfolioID
    }
    if (portfolioID <= 0) return
    const rows = await listPositions(portfolioID).catch(() => [] as Position[])
    if (mySeq !== positionsSeq || portfolioID !== $selectedPortfolioID) return
    positions = rows
  }

  // 切換選擇：先清掉畫面上的計畫、持倉、流水與兩張表單，再載入新的
  function switchTo(next: Selection | null) {
    sel = next
    current = null
    transactions = []
    analyses = []
    dataStatus = null
    viewing = null
    trade = emptyTrade()
    adjustment = emptyAdjustment()
  }

  async function changePortfolio(raw: string) {
    const nextID = Number(raw)
    if (!nextID || nextID === $selectedPortfolioID) return
    selectedPortfolioID.set(nextID)
    const symbol = sel?.symbol ?? symbolInput.trim().toUpperCase()
    // 投組切換立即確定選擇（review 2026-11-26 [P2]）：⛔ 不等持倉清單回來才選，否則期間使用者選的別檔會被跳回
    const loading = symbol ? select(symbol) : (switchTo(null), Promise.resolve())
    await Promise.all([loading, loadPositions()])
  }

  function changePortfolioFromEvent(event: Event) {
    const target = event.currentTarget as HTMLSelectElement
    void changePortfolio(target.value)
  }

  async function savePortfolio() {
    const name = newPortfolioName.trim()
    if (!name) return
    creatingPortfolio = true
    pageError = ''
    try {
      const created = await createPortfolio({ name })
      newPortfolioName = ''
      await loadPortfolioOptions()
      selectedPortfolioID.set(created.id)
      // 建立後直接回到原本那一檔（同樣立即確定選擇，再等持倉清單）
      const symbol = symbolInput.trim().toUpperCase()
      await Promise.all([symbol ? select(symbol) : Promise.resolve(), loadPositions()])
    } catch (err) {
      pageError = err instanceof ApiError ? err.message : '建立 Portfolio 失敗'
    } finally {
      creatingPortfolio = false
    }
  }

  // 選一檔：固定這份選擇後載入
  async function select(raw: string) {
    const symbol = raw.trim().toUpperCase()
    symbolInput = symbol
    if (!symbol) return
    const portfolioID = $selectedPortfolioID
    if (portfolioID <= 0) {
      switchTo(null)
      return
    }
    const next = { portfolioID, symbol }
    if (!sel || keyOf(sel) !== keyOf(next)) switchTo(next)
    await load(next)
  }

  // 載入鏈：持倉 → 流水 → 歷史，三步都用同一份選擇；最後一次比對通過才一起套用
  async function load(s: Selection) {
    const k = keyOf(s)
    const mySeq = ++seq
    latestLoadSeq = { ...latestLoadSeq, [k]: mySeq }
    if (isCurrent(s)) loadingKey = k
    try {
      const pos = await getPosition(s.symbol, s.portfolioID)
      const txs = await listPositionTransactions(s.symbol, s.portfolioID)
      const hist = await listTradeAnalyses(s.symbol, s.portfolioID)
      if (!isCurrent(s) || latestLoadSeq[k] !== mySeq) return
      // 這次載入在寫入成功之後才開始 → 持倉與行情已重新確認，撤下「寫入後待確認」
      const w = writtenSeqByKey[k]
      if (w !== undefined && mySeq > w) writtenSeqByKey = { ...writtenSeqByKey, [k]: undefined }
      current = pos
      transactions = txs
      analyses = hist.analyses
      dataStatus = hist.data_status
      viewing = null
      adjustment = { shares: String(pos.shares), avgCost: String(pos.avg_cost), reason: '' }
      setLoadError(k, undefined)
    } catch (err) {
      if (!isCurrent(s) || latestLoadSeq[k] !== mySeq) return
      setLoadError(k, err instanceof ApiError ? err.message : '載入 Position 失敗')
    } finally {
      if (latestLoadSeq[k] === mySeq && loadingKey === k) loadingKey = ''
    }
  }

  async function runAnalysis(forceRefresh = false) {
    if (!sel) return
    if (!canWritePortfolio) {
      setError(keyOf(sel), '目前 Portfolio 沒有寫入權限')
      return
    }
    const s = sel
    const k = keyOf(s)
    if (busyByKey[k]) return
    setBusy(k, 'analyzing')
    setError(k, undefined)
    try {
      await analyzeTrade(s.symbol, s.portfolioID, forceRefresh)
      // 結果記在發出請求的那個鍵上；成功時下次載入那個鍵，歷史自然有新的一筆
      setOutcome(k, { kind: 'ok' })
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setOutcome(k, { kind: 'not_found' })
      } else {
        const message = err instanceof ApiError ? err.message : '分析失敗'
        setOutcome(k, { kind: 'failed', at: new Date().toISOString(), message })
        setError(k, `分析失敗：${message}`)
      }
    } finally {
      setBusy(k, undefined)
    }
    // ⛔ 不用當下的選擇重新載入：只有選擇沒變時才重新載入發出請求的那一檔
    if (isCurrent(s)) await load(s)
  }

  async function saveTrade() {
    if (!sel || !current) return
    if (!canWritePortfolio) {
      setError(keyOf(sel), '目前 Portfolio 沒有寫入權限')
      return
    }
    const s = sel
    const k = keyOf(s)
    if (busyByKey[k]) return
    // expected_version 用這份選擇當時的持倉版本
    const input = {
      portfolio_id: s.portfolioID,
      event_type: eventType,
      shares: Number(trade.shares),
      price: Number(trade.price),
      fee: Number(trade.fee || 0),
      tax: Number(trade.tax || 0),
      expected_version: current.version,
      note: trade.note.trim(),
    }
    const label = eventType === 'BUY' ? '買進' : '賣出'
    setBusy(k, 'writing')
    setError(k, undefined)
    try {
      await addPositionTransaction(s.symbol, input)
      markWritten(k)
      if (isCurrent(s)) {
        trade = emptyTrade()
        await load(s)
        await loadPositions()
      }
    } catch (err) {
      // 選擇已經換掉時⛔ 不顯示在別檔；錯誤記在發出請求的那個鍵，回到那個鍵才看得到
      setError(k, `${label}寫入失敗：${err instanceof ApiError ? err.message : '交易寫入失敗'}`)
    } finally {
      setBusy(k, undefined)
    }
  }

  async function saveAdjustment() {
    if (!sel || !current) return
    if (!canWritePortfolio) {
      setError(keyOf(sel), '目前 Portfolio 沒有寫入權限')
      return
    }
    if (!adjustment.reason.trim()) {
      setError(keyOf(sel), 'ADJUSTMENT 必須填寫更正原因')
      return
    }
    const s = sel
    const k = keyOf(s)
    if (busyByKey[k]) return
    const input = {
      portfolio_id: s.portfolioID,
      target_shares: Number(adjustment.shares),
      target_avg_cost: Number(adjustment.avgCost),
      expected_version: current.version,
      reason: adjustment.reason.trim(),
    }
    setBusy(k, 'writing')
    setError(k, undefined)
    try {
      await adjustPosition(s.symbol, input)
      markWritten(k)
      if (isCurrent(s)) {
        await load(s)
        await loadPositions()
      }
    } catch (err) {
      setError(k, `更正失敗：${err instanceof ApiError ? err.message : '更正失敗'}`)
    } finally {
      setBusy(k, undefined)
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
  function conditionLabel(latest: TradePlan): string {
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
  const statusClass: Record<string, string> = {
    latest: 'border-green-700/60 text-green-300',
    insufficient: 'border-yellow-700/60 text-yellow-300',
    no_plan: 'border-border text-muted',
  }
</script>

<Layout>
  <div class="max-w-6xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">交易分析</h1>
    {#if pageError}<p class="text-rise text-sm">{pageError}</p>{/if}
    {#if curError}<p class="text-rise text-sm" data-testid="plan-error">{curError}</p>{/if}

    <div class="bg-panel border border-border rounded-xl p-4 space-y-3">
      <div class="flex gap-3 flex-wrap">
        <select value={$selectedPortfolioID} on:change={changePortfolioFromEvent} data-testid="portfolio-select"
          class="min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-white">
          {#if portfolios.length === 0}
            <option value="0">尚無 Portfolio</option>
          {/if}
          {#each portfolios as portfolio}
            <option value={portfolio.id}>{portfolio.name} · {portfolio.owner_type}{portfolio.can_write ? '' : ' · READ'}</option>
          {/each}
        </select>
        <input bind:value={newPortfolioName} placeholder="新增個人 Portfolio" data-testid="new-portfolio-name"
          class="flex-1 min-w-[180px] bg-surface border border-border rounded-lg px-3 py-2 text-white" />
        <button class="px-4 py-2 border border-border rounded-lg text-muted disabled:opacity-50" data-testid="create-portfolio"
          disabled={creatingPortfolio || !newPortfolioName.trim()} on:click={savePortfolio}>建立</button>
      </div>
      {#if portfoliosLoaded && portfolios.length === 0}
        <p class="text-yellow-300 text-xs" data-testid="no-portfolio-hint">
          請先建立 Portfolio，才能載入持倉、寫入交易或執行分析。{#if symbolInput.trim()}建立後會開啟 {symbolInput.trim().toUpperCase()} 的計畫。{/if}
        </p>
      {:else if selectedPortfolio && !selectedPortfolio.can_write}
        <p class="text-yellow-300 text-xs">目前 Portfolio 為唯讀，已停用交易、更正與分析寫入；歷史照樣可看。</p>
      {/if}
    </div>

    <div class="bg-panel border border-border rounded-xl p-4 flex gap-3 flex-wrap">
      <input bind:value={symbolInput} placeholder="股票代號，例如 2330" on:keydown={(e) => e.key === 'Enter' && select(symbolInput)} data-testid="symbol-input"
        class="flex-1 min-w-[180px] bg-surface border border-border rounded-lg px-3 py-2 text-white" />
      <button class="px-4 py-2 bg-indigo-600 rounded-lg text-white disabled:opacity-50" disabled={!hasSelectedPortfolio} on:click={() => select(symbolInput)}>載入</button>
      <button class="px-4 py-2 bg-green-700 rounded-lg text-white disabled:opacity-50" data-testid="analyze"
        disabled={!sel || !!curBusy || !canWritePortfolio} on:click={() => runAnalysis(false)}>分析</button>
      <button class="px-4 py-2 border border-border rounded-lg text-muted disabled:opacity-50" data-testid="force-refresh"
        disabled={!sel || !!curBusy || !canWritePortfolio} on:click={() => runAnalysis(true)}>強制刷新 SR</button>
    </div>

    <!-- T-089 D4：計畫摘要卡 -->
    <div class="bg-panel border border-border rounded-xl p-4 space-y-3" data-testid="plan-summary">
      <div class="flex items-center justify-between gap-3 flex-wrap">
        <h2 class="text-white text-sm font-semibold">計畫摘要{#if sel}<span class="text-muted font-mono ml-2">{sel.symbol}</span>{/if}</h2>
        {#if curBusy === 'analyzing'}<span class="text-xs text-muted">分析中…</span>{/if}
        {#if curBusy === 'writing'}<span class="text-xs text-muted">寫入中…</span>{/if}
      </div>
      {#if !sel}
        {#if portfoliosLoaded && portfolios.length === 0}
          <p class="text-muted text-sm">先在上方建立 Portfolio{#if symbolInput.trim()}，建立後會開啟 {symbolInput.trim().toUpperCase()} 的計畫{/if}。</p>
        {:else}
          <p class="text-muted text-sm">從觀察清單按「計畫」，或輸入股票代號載入。</p>
        {/if}
      {:else if loading}
        <p class="text-muted text-sm" data-testid="plan-loading">載入 {sel.symbol}…</p>
      {:else if status}
        <p class="text-sm border rounded px-3 py-2 {statusClass[status.main] ?? 'border-yellow-700/60 text-yellow-200'}" data-testid="plan-status" data-state={status.main}>
          {status.label}
        </p>
        {#each status.hints as hint}<p class="text-xs text-muted" data-testid="plan-hint">{hint}</p>{/each}
        {#if shownPlan && status.showJudgement}
          <div class="grid md:grid-cols-4 gap-3 text-sm">
            <div>
              <p class="text-muted text-xs">判斷</p>
              <span class="inline-block mt-1 px-3 py-1 rounded-full text-xs {actionClass[shownPlan.action]}" data-testid="plan-judgement">{conditionLabel(shownPlan)}</span>
            </div>
            <div class="md:col-span-3">
              <p class="text-muted text-xs">主要理由</p>
              <p class="text-white text-xs mt-1">{shownPlan.reason?.[0] ?? '—'}</p>
            </div>
            <div class="md:col-span-2">
              <p class="text-muted text-xs">下一個觀察條件</p>
              {#if shownPlan.trigger_conditions?.length}
                {#each shownPlan.trigger_conditions as c}<p class="text-white text-xs mt-1">• {c}</p>{/each}
              {:else}<p class="text-white text-xs mt-1">—</p>{/if}
            </div>
            <div>
              <p class="text-muted text-xs">行情日期</p>
              <p class="text-white font-mono" data-testid="plan-market-date">{shownPlan.market_date ? shortDate(shownPlan.market_date) : '無法確認'}</p>
            </div>
            <div>
              <p class="text-muted text-xs">分析紀錄時間</p>
              <p class="text-white font-mono">{mdhm(shownPlan.created_at)}</p>
            </div>
          </div>
        {/if}
        {#if !status.current}
          <div class="flex items-center gap-3">
            <button class="px-4 py-2 bg-green-700 rounded-lg text-white text-sm disabled:opacity-50" data-testid="summary-analyze"
              disabled={!!curBusy || !canWritePortfolio} on:click={() => runAnalysis(false)}>分析</button>
            {#if !canWritePortfolio}<span class="text-xs text-muted">目前 Portfolio 為唯讀，無法執行分析</span>{/if}
          </div>
        {/if}
      {/if}
    </div>

    <div class="grid lg:grid-cols-3 gap-4">
      <div class="bg-panel border border-border rounded-xl p-4">
        <h2 class="text-white text-sm font-semibold mb-3">目前持股狀態</h2>
        {#if current}
          <div class="grid grid-cols-2 gap-3 text-sm" data-testid="position-card">
            <div><p class="text-muted text-xs">狀態</p><p class="text-white">{current.shares > 0 ? 'LONG' : 'FLAT'}</p></div>
            <div><p class="text-muted text-xs">Portfolio</p><p class="text-white">{current.portfolio_id}</p></div>
            <div><p class="text-muted text-xs">版本</p><p class="text-white">{current.version}</p></div>
            <div><p class="text-muted text-xs">股數</p><p class="text-white" data-testid="position-shares">{num(current.shares)}</p></div>
            <div><p class="text-muted text-xs">AVG 成本</p><p class="text-white">{num(current.avg_cost)}</p></div>
            <div class="col-span-2"><p class="text-muted text-xs">已實現損益</p><p class={current.realized_pnl >= 0 ? 'text-rise' : 'text-fall'}>{num(current.realized_pnl)}</p></div>
          </div>
        {:else}<p class="text-muted text-sm">輸入股票代號載入；沒有交易時視為 FLAT。</p>{/if}
      </div>

      <div class="bg-panel border border-border rounded-xl p-4 space-y-2">
        <h2 class="text-white text-sm font-semibold">新增交易</h2>
        <div class="grid grid-cols-2 gap-2">
          <select bind:value={eventType} class="bg-surface border border-border rounded px-2 py-2 text-white"><option>BUY</option><option>SELL</option></select>
          <input bind:value={trade.shares} type="number" placeholder="股數" data-testid="trade-shares" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.price} type="number" placeholder="價格" data-testid="trade-price" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.fee} type="number" placeholder="手續費" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.tax} type="number" placeholder="交易稅" class="bg-surface border border-border rounded px-2 py-2 text-white" />
          <input bind:value={trade.note} placeholder="備註" data-testid="trade-note" class="bg-surface border border-border rounded px-2 py-2 text-white" />
        </div>
        <button class="w-full bg-indigo-600 text-white rounded py-2 disabled:opacity-50" data-testid="save-trade" disabled={!!curBusy || !current || !canWritePortfolio} on:click={saveTrade}>寫入不可變交易</button>
      </div>

      <div class="bg-panel border border-border rounded-xl p-4 space-y-2">
        <h2 class="text-white text-sm font-semibold">ADJUSTMENT</h2>
        <input bind:value={adjustment.shares} type="number" placeholder="更正後股數" data-testid="adjust-shares" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.avgCost} type="number" placeholder="更正後 AVG" data-testid="adjust-avg" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <input bind:value={adjustment.reason} placeholder="必填：更正原因" data-testid="adjust-reason" class="w-full bg-surface border border-border rounded px-2 py-2 text-white" />
        <button class="w-full border border-yellow-700 text-yellow-300 rounded py-2 disabled:opacity-50" data-testid="save-adjustment" disabled={!!curBusy || !current || !canWritePortfolio} on:click={saveAdjustment}>新增更正事件</button>
      </div>
    </div>

    <div class="bg-panel border border-border rounded-xl p-4">
      <h2 class="text-white text-sm font-semibold mb-3">計畫細節</h2>
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
        <div class="space-y-2 max-h-80 overflow-auto" data-testid="plan-history">
          {#each analyses as item}
            <button class="w-full border rounded p-2 text-xs flex justify-between gap-2 {shownPlan?.id === item.id ? 'border-indigo-500/60' : 'border-border'}"
              data-testid="history-item-{item.id}" on:click={() => (viewing = item)}>
              <span class="text-white">{item.action_label} · 目標 {num(item.target_shares)}</span>
              <span class="text-muted">
                行情 {item.market_date ? shortDate(item.market_date) : '無法確認'} · 分析 {dt(item.created_at)}
              </span>
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
