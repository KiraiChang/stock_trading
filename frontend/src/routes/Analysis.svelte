<script lang="ts">
  import Layout from '../components/layout/Layout.svelte'
  import {
    createAnalysis,
    listAnalyses,
    getAnalysis,
    verifyAnalysis,
    parseTradeVerification,
    type StockAnalysis,
    type AnalysisLevel,
  } from '../lib/api/analysis'

  let symbol = ''
  let submitting = false
  let submitError = ''

  let current: StockAnalysis | null = null
  let currentLevels: AnalysisLevel[] = []

  let history: StockAnalysis[] = []
  let historyLoading = false
  let verifyingId: number | null = null

  const trendLabel: Record<string, string> = { BULLISH: '多頭', BEARISH: '空頭', SIDEWAYS: '盤整' }
  const trendClass: Record<string, string> = {
    BULLISH: 'text-rise', BEARISH: 'text-fall', SIDEWAYS: 'text-muted',
  }
  const directionLabel: Record<string, string> = { LONG: '做多', SHORT: '放空', NONE: '無方向' }
  const entryStatusLabel: Record<string, string> = { ACTIVE: '已觸發', WATCHING: '觀察中' }
  const entryStatusClass: Record<string, string> = {
    ACTIVE: 'bg-green-900/40 text-green-400',
    WATCHING: 'bg-blue-900/40 text-blue-400',
  }
  const levelStatusLabel: Record<string, string> = {
    PENDING: '尚未驗證', HELD_SO_FAR: '目前守住', BROKEN: '已被突破',
  }
  const levelStatusClass: Record<string, string> = {
    PENDING: 'bg-gray-700/60 text-gray-400',
    HELD_SO_FAR: 'bg-green-900/40 text-green-400',
    BROKEN: 'bg-red-900/40 text-red-400',
  }

  async function submit() {
    if (!symbol.trim()) {
      submitError = '請輸入股票代號'
      return
    }
    submitting = true
    submitError = ''
    try {
      const { analysis, levels } = await createAnalysis(symbol.trim(), '1d')
      current = analysis
      currentLevels = levels
      await loadHistory()
    } catch {
      submitError = '分析失敗，請確認股票代號是否有歷史資料、Python service 是否已啟動'
    } finally {
      submitting = false
    }
  }

  async function loadHistory() {
    if (!symbol.trim()) return
    historyLoading = true
    try {
      history = await listAnalyses(symbol.trim(), 20)
    } catch {
      // 沉默失敗，不影響主要分析結果的呈現
    } finally {
      historyLoading = false
    }
  }

  async function selectHistory(id: number) {
    try {
      const { analysis, levels } = await getAnalysis(id)
      current = analysis
      currentLevels = levels
    } catch {
      // ignore
    }
  }

  async function reVerify(id: number) {
    verifyingId = id
    try {
      const { analysis, levels } = await verifyAnalysis(id)
      if (current?.id === id) {
        current = analysis
        currentLevels = levels
      }
      await loadHistory()
    } catch {
      // ignore
    } finally {
      verifyingId = null
    }
  }

  function formatDateTime(ts?: string): string {
    if (!ts) return '—'
    return new Date(ts).toLocaleString('zh-TW', { hour12: false })
  }

  function fmt(v?: number | null): string {
    return v === undefined || v === null ? '—' : v.toFixed(2)
  }

  $: verification = current ? parseTradeVerification(current.trade_verification) : null
  $: supports = currentLevels.filter((lv) => lv.type === 'SUPPORT')
  $: resistances = currentLevels.filter((lv) => lv.type === 'RESISTANCE')
</script>

<Layout>
  <div class="max-w-4xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">個股分析</h1>

    <!-- ── 輸入表單 ──────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      {#if submitError}
        <p class="text-rise text-sm mb-3">{submitError}</p>
      {/if}
      <div class="flex gap-3">
        <input
          bind:value={symbol}
          placeholder="輸入股票代號，例如 2330"
          on:keydown={(e) => e.key === 'Enter' && submit()}
          class="flex-1 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={submitting}
          on:click={submit}
        >
          {submitting ? '分析中...' : '分析'}
        </button>
      </div>
      <p class="text-muted text-xs mt-2">
        會呼叫 Python 分析服務即時計算，結果會存入資料庫供之後驗證使用。
      </p>
    </div>

    <!-- ── 目前分析結果 ──────────────────────────────────────── -->
    {#if current}
      <div class="bg-panel border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 class="text-white font-semibold">{current.symbol}</h2>
            <p class="text-muted text-xs mt-0.5">分析時間：{formatDateTime(current.analyzed_at)}</p>
          </div>
          <div class="text-right">
            <p class="text-white font-mono text-lg">{fmt(current.current_price)}</p>
            <p class="text-xs {trendClass[current.trend] ?? 'text-muted'}">{trendLabel[current.trend] ?? current.trend}</p>
          </div>
        </div>

        <!-- 進場建議 -->
        <div class="px-5 py-4 border-b border-border">
          <div class="flex items-center gap-2 mb-2">
            <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
              {entryStatusClass[current.entry_status] ?? 'bg-gray-700/60 text-gray-400'}">
              {entryStatusLabel[current.entry_status] ?? current.entry_status}
            </span>
            <span class="text-sm text-white font-medium">{directionLabel[current.entry_direction] ?? current.entry_direction}</span>
            <span class="text-sm text-muted font-mono">@ {fmt(current.entry_price)}</span>
          </div>
          {#if current.entry_reason}
            <p class="text-muted text-xs">{current.entry_reason}</p>
          {/if}
        </div>

        <!-- 停損比較 -->
        <div class="px-5 py-4 border-b border-border">
          <h3 class="text-xs font-semibold text-muted mb-2">停損比較</h3>
          <div class="grid grid-cols-3 gap-3 text-sm">
            <div><p class="text-muted text-xs mb-1">ATR</p><p class="text-fall font-mono">{fmt(current.stop_loss_atr)}</p></div>
            <div><p class="text-muted text-xs mb-1">結構</p><p class="text-fall font-mono">{fmt(current.stop_loss_structural)}</p></div>
            <div><p class="text-muted text-xs mb-1">複合（較保守）</p><p class="text-fall font-mono">{fmt(current.stop_loss_composite)}</p></div>
          </div>
        </div>

        <!-- 停利比較 -->
        <div class="px-5 py-4 border-b border-border">
          <h3 class="text-xs font-semibold text-muted mb-2">停利比較</h3>
          <div class="grid grid-cols-3 gap-3 text-sm">
            <div><p class="text-muted text-xs mb-1">下一關卡</p><p class="text-rise font-mono">{fmt(current.take_profit_next_level)}</p></div>
            <div><p class="text-muted text-xs mb-1">風險報酬比 2R</p><p class="text-rise font-mono">{fmt(current.take_profit_risk_reward)}</p></div>
            <div><p class="text-muted text-xs mb-1">ATR 倍數</p><p class="text-rise font-mono">{fmt(current.take_profit_atr)}</p></div>
          </div>
        </div>

        <!-- 驗證結果（若已觸發過進場且驗證過） -->
        {#if verification?.applicable}
          <div class="px-5 py-4 border-b border-border">
            <h3 class="text-xs font-semibold text-muted mb-2">
              進場後驗證結果{current.verified_at ? `（${formatDateTime(current.verified_at)}）` : ''}
            </h3>
            <div class="grid grid-cols-2 gap-4 text-xs">
              <div>
                <p class="text-muted mb-1">停損觸及狀況</p>
                {#each Object.entries(verification.stop_loss ?? {}) as [key, r]}
                  <p class="{r.hit ? 'text-fall' : 'text-muted'}">
                    {key}：{r.hit ? `已觸及 @ ${formatDateTime(r.hit_at)}` : '尚未觸及'}
                  </p>
                {/each}
              </div>
              <div>
                <p class="text-muted mb-1">停利觸及狀況</p>
                {#each Object.entries(verification.take_profit ?? {}) as [key, r]}
                  <p class="{r.hit ? 'text-rise' : 'text-muted'}">
                    {key}：{r.hit ? `已觸及 @ ${formatDateTime(r.hit_at)}` : '尚未觸及'}
                  </p>
                {/each}
              </div>
            </div>
          </div>
        {/if}

        <!-- 支撐/壓力清單 -->
        <div class="px-5 py-4 flex items-center justify-between">
          <h3 class="text-xs font-semibold text-muted">支撐 / 壓力（含驗證狀態）</h3>
          <button
            class="text-xs px-3 py-1.5 border border-border text-muted hover:text-white rounded-lg transition-colors disabled:opacity-50"
            disabled={verifyingId === current.id}
            on:click={() => current && reVerify(current.id)}
          >
            {verifyingId === current.id ? '驗證中...' : '重新驗證'}
          </button>
        </div>
        <div class="grid grid-cols-2 gap-px bg-border">
          <div class="bg-panel px-5 py-3">
            <p class="text-xs text-muted mb-2">壓力（由高到低）</p>
            {#each resistances as lv (lv.id)}
              <div class="flex items-center justify-between py-1.5 border-b border-border/50 last:border-0 text-sm">
                <span class="font-mono text-fall">{fmt(lv.price)}</span>
                <span class="text-muted text-xs">{lv.method}</span>
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs {levelStatusClass[lv.status]}">
                  {levelStatusLabel[lv.status]}
                </span>
              </div>
            {:else}
              <p class="text-muted text-xs py-2">無資料</p>
            {/each}
          </div>
          <div class="bg-panel px-5 py-3">
            <p class="text-xs text-muted mb-2">支撐（由高到低）</p>
            {#each supports as lv (lv.id)}
              <div class="flex items-center justify-between py-1.5 border-b border-border/50 last:border-0 text-sm">
                <span class="font-mono text-rise">{fmt(lv.price)}</span>
                <span class="text-muted text-xs">{lv.method}</span>
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs {levelStatusClass[lv.status]}">
                  {levelStatusLabel[lv.status]}
                </span>
              </div>
            {:else}
              <p class="text-muted text-xs py-2">無資料</p>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    <!-- ── 歷史分析紀錄 ──────────────────────────────────────── -->
    {#if symbol.trim()}
      <div class="bg-panel border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-3 border-b border-border">
          <h2 class="text-sm font-semibold text-white">{symbol} 歷史分析紀錄</h2>
        </div>
        <table class="w-full text-sm">
          <thead>
            <tr class="text-muted text-xs border-b border-border">
              <th class="text-left px-5 py-2">分析時間</th>
              <th class="text-center px-3 py-2">趨勢</th>
              <th class="text-center px-3 py-2">進場</th>
              <th class="text-right px-3 py-2">進場價</th>
              <th class="text-right px-5 py-2">操作</th>
            </tr>
          </thead>
          <tbody>
            {#if historyLoading}
              <tr><td colspan="5" class="px-5 py-6 text-center text-muted">載入中...</td></tr>
            {:else if history.length === 0}
              <tr><td colspan="5" class="px-5 py-6 text-center text-muted">尚無歷史紀錄</td></tr>
            {:else}
              {#each history as h (h.id)}
                <tr
                  class="border-b border-border/50 hover:bg-border/20 cursor-pointer transition-colors
                         {current?.id === h.id ? 'bg-indigo-900/20' : ''}"
                  on:click={() => selectHistory(h.id)}
                >
                  <td class="px-5 py-2 text-muted text-xs font-mono">{formatDateTime(h.analyzed_at)}</td>
                  <td class="px-3 py-2 text-center text-xs {trendClass[h.trend] ?? 'text-muted'}">{trendLabel[h.trend] ?? h.trend}</td>
                  <td class="px-3 py-2 text-center">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs {entryStatusClass[h.entry_status]}">
                      {entryStatusLabel[h.entry_status]}
                    </span>
                  </td>
                  <td class="px-3 py-2 text-right font-mono">{fmt(h.entry_price)}</td>
                  <td class="px-5 py-2 text-right">
                    <button
                      class="text-xs px-2.5 py-1 border border-border text-muted hover:text-white rounded transition-colors disabled:opacity-50"
                      disabled={verifyingId === h.id}
                      on:click|stopPropagation={() => reVerify(h.id)}
                    >
                      {verifyingId === h.id ? '...' : '重新驗證'}
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</Layout>
