<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import {
    createSRZoneAnalysis,
    listSRZoneAnalyses,
    getSRZoneAnalysis,
    deleteSRZoneAnalysis,
    triggerSRScoringTrain,
    type SRZoneAnalysis,
    type SRZone,
  } from '../lib/api/srZones'

  let symbol = ''
  let fetchLimit = 250
  let submitting = false
  let submitError = ''

  let current: SRZoneAnalysis | null = null
  let currentZones: SRZone[] = []

  let history: SRZoneAnalysis[] = []
  let historyLoading = true
  let confirmDeleteId: number | null = null
  let deletingId: number | null = null

  // ── 訓練/更新機率模型 ──────────────────────────────────────
  let trainSymbols = ''
  let trainLimit = 1500
  let trainModelType: 'gradient_boosting' | 'logistic_regression' = 'gradient_boosting'
  let training = false
  let trainMessage = ''
  let trainError = ''

  onMount(loadHistory)

  const roleLabel: Record<string, string> = {
    SUPPORT: '支撐',
    RESISTANCE: '壓力',
    AT_ZONE: '現價在區間內',
  }
  const roleClass: Record<string, string> = {
    SUPPORT: 'bg-green-900/40 text-rise',
    RESISTANCE: 'bg-red-900/40 text-fall',
    AT_ZONE: 'bg-gray-700/60 text-gray-300',
  }
  const roleTextClass: Record<string, string> = {
    SUPPORT: 'text-rise',
    RESISTANCE: 'text-fall',
    AT_ZONE: 'text-muted',
  }
  const methodLabel: Record<string, string> = {
    atr: 'ATR 通道',
    volume_profile: '成交量分布',
  }
  const statusLabel: Record<string, string> = {
    PENDING: '尚未驗證', HELD_SO_FAR: '目前守住', BROKEN: '已被突破',
  }
  const statusClass: Record<string, string> = {
    PENDING: 'bg-gray-700/60 text-gray-400',
    HELD_SO_FAR: 'bg-green-900/40 text-green-400',
    BROKEN: 'bg-red-900/40 text-red-400',
  }

  async function submit() {
    if (!symbol.trim()) {
      submitError = '請輸入股票代號'
      return
    }
    if (fetchLimit < 35) {
      submitError = '抓取根數至少要 35 根，分析才有足夠資料可用'
      return
    }
    submitting = true
    submitError = ''
    try {
      const { analysis, zones } = await createSRZoneAnalysis(symbol.trim(), '1d', fetchLimit)
      current = analysis
      currentZones = sortZones(zones)
      await loadHistory()
    } catch {
      submitError = '分析失敗，請確認股票代號是否有歷史資料、Python service 是否已啟動，或機率模型尚未訓練（見下方「訓練/更新機率模型」）'
    } finally {
      submitting = false
    }
  }

  async function runTrain() {
    training = true
    trainError = ''
    trainMessage = ''
    try {
      const symbols = trainSymbols
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.length > 0)
      const res = await triggerSRScoringTrain({
        symbols: symbols.length > 0 ? symbols : undefined,
        limit: trainLimit,
        modelType: trainModelType,
      })
      trainMessage = `${res.message}（${res.symbols} 檔股票）`
    } catch {
      trainError = '觸發失敗，請確認 Python service 是否已啟動'
    } finally {
      training = false
    }
  }

  function sortZones(zones: SRZone[]): SRZone[] {
    return [...zones].sort((a, b) => Math.max(b.support_score, b.resistance_score) - Math.max(a.support_score, a.resistance_score))
  }

  // symbol 留空時列出「所有股票」最近的分析紀錄，方便一進頁面就有內容可看；
  // symbol 有值時才篩選成該股票的歷史紀錄
  async function loadHistory() {
    historyLoading = true
    try {
      history = await listSRZoneAnalyses(symbol.trim() || undefined, 20)
    } catch {
      // 沉默失敗，不影響主要分析結果的呈現
    } finally {
      historyLoading = false
    }
  }

  async function selectHistory(h: SRZoneAnalysis) {
    try {
      const { analysis, zones } = await getSRZoneAnalysis(h.id)
      current = analysis
      currentZones = sortZones(zones)
      if (symbol.trim() !== h.symbol) {
        symbol = h.symbol
        await loadHistory()
      }
    } catch {
      // ignore
    }
  }

  async function doDelete(id: number) {
    deletingId = id
    try {
      await deleteSRZoneAnalysis(id)
      history = history.filter((h) => h.id !== id)
      if (current?.id === id) {
        current = null
        currentZones = []
      }
    } catch {
      // ignore，列表維持原狀讓使用者可以重試
    } finally {
      deletingId = null
      confirmDeleteId = null
    }
  }

  function formatDateTime(ts?: string): string {
    if (!ts) return '—'
    return new Date(ts).toLocaleString('zh-TW', { hour12: false })
  }

  function fmt(v?: number | null): string {
    return v === undefined || v === null ? '—' : v.toFixed(2)
  }

  function fmtPct(v?: number | null): string {
    return v === undefined || v === null ? '—' : `${(v * 100).toFixed(1)}%`
  }
</script>

<Layout>
  <div class="max-w-4xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">支撐/壓力機率分析</h1>

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
        <input
          type="number"
          min="35"
          step="10"
          bind:value={fetchLimit}
          title="抓取的歷史K棒根數"
          on:keydown={(e) => e.key === 'Enter' && submit()}
          class="w-28 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
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
        用 ATR 通道與成交量分布兩種方法建立價格區間（zone），對每個區間算出規則式的支撐/壓力強度分數，
        以及依歷史觸碰事件訓練的機率模型預測反彈/跌破機率。抓取根數指分析用的歷史K棒數量（預設 250，至少
        35 根）。機率欄位需要下方先訓練過模型才會有值。
      </p>
    </div>

    <!-- ── 訓練/更新機率模型 ────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      <h2 class="text-sm font-semibold text-white mb-1">訓練/更新機率模型</h2>
      <p class="text-muted text-xs mb-3">
        用歷史觸碰事件重新訓練 bounce_probability / break_probability 模型，在背景執行（視股票數與資料長度可能耗時數十秒到數分鐘）。
      </p>
      {#if trainError}
        <p class="text-rise text-sm mb-3">{trainError}</p>
      {/if}
      {#if trainMessage}
        <p class="text-green-400 text-sm mb-3">{trainMessage}</p>
      {/if}
      <div class="flex flex-wrap gap-3 items-center">
        <input
          bind:value={trainSymbols}
          placeholder="股票代號，逗號分隔（留空 = 整個監控清單）"
          class="flex-1 min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <select
          bind:value={trainModelType}
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        >
          <option value="gradient_boosting">Gradient Boosting</option>
          <option value="logistic_regression">Logistic Regression</option>
        </select>
        <input
          type="number"
          min="35"
          step="100"
          bind:value={trainLimit}
          title="訓練用的歷史K棒根數（每檔股票）"
          class="w-32 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={training}
          on:click={runTrain}
        >
          {training ? '觸發中...' : '開始訓練'}
        </button>
      </div>
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
            <p class="text-muted text-xs">{currentZones.length} 個區間</p>
          </div>
        </div>

        <div class="divide-y divide-border">
          {#each currentZones as z (z.id)}
            <div class="px-5 py-4">
              <div class="flex items-center justify-between mb-3">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-white text-sm">{fmt(z.price_low)} ~ {fmt(z.price_high)}</span>
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {roleClass[z.role] ?? 'bg-gray-700/60 text-gray-400'}">
                    {roleLabel[z.role] ?? z.role}
                  </span>
                  <span class="text-muted text-xs">{methodLabel[z.method] ?? z.method}</span>
                </div>
                <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs {statusClass[z.status] ?? 'bg-gray-700/60 text-gray-400'}">
                  {statusLabel[z.status] ?? z.status}
                </span>
              </div>

              <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs mb-3">
                <div>
                  <p class="text-muted mb-1">支撐強度分數</p>
                  <p class="text-rise font-mono">{fmtPct(z.support_score)}</p>
                </div>
                <div>
                  <p class="text-muted mb-1">壓力強度分數</p>
                  <p class="text-fall font-mono">{fmtPct(z.resistance_score)}</p>
                </div>
                <div>
                  <p class="text-muted mb-1">反彈機率</p>
                  <p class="text-white font-mono">{fmtPct(z.bounce_probability)}</p>
                </div>
                <div>
                  <p class="text-muted mb-1">跌破機率</p>
                  <p class="text-white font-mono">{fmtPct(z.break_probability)}</p>
                </div>
              </div>

              <div class="grid grid-cols-3 sm:grid-cols-7 gap-3 text-xs text-muted">
                <div><p class="mb-1">觸碰次數</p><p class="text-white">{z.touch_count}</p></div>
                <div><p class="mb-1">拒絕次數</p><p class="text-white">{z.rejection_count}</p></div>
                <div><p class="mb-1">突破次數</p><p class="text-white">{z.breakout_count}</p></div>
                <div><p class="mb-1">觸碰後平均報酬</p><p class="{roleTextClass[z.role] ?? 'text-white'}">{fmtPct(z.avg_return_after_touch)}</p></div>
                <div><p class="mb-1">相對量能</p><p class="text-white">{z.relative_volume.toFixed(2)}x</p></div>
                <div><p class="mb-1">波動率</p><p class="text-white">{fmtPct(z.volatility)}</p></div>
                <div><p class="mb-1">趨勢強度</p><p class="{z.trend_strength >= 0 ? 'text-rise' : 'text-fall'}">{fmtPct(z.trend_strength)}</p></div>
              </div>
            </div>
          {:else}
            <p class="text-muted text-xs px-5 py-6 text-center">這次分析沒有偵測到任何價格區間</p>
          {/each}
        </div>
      </div>
    {/if}

    <!-- ── 歷史分析紀錄：一進頁面就顯示（未指定股票時顯示所有股票最近紀錄）── -->
    <div class="bg-panel border border-border rounded-xl overflow-hidden">
      <div class="px-5 py-3 border-b border-border flex items-center justify-between">
        <h2 class="text-sm font-semibold text-white">
          {symbol.trim() ? `${symbol} 歷史分析紀錄` : '最近分析紀錄（所有股票）'}
        </h2>
        {#if symbol.trim()}
          <button
            class="text-xs text-muted hover:text-white transition-colors"
            on:click={() => { symbol = ''; loadHistory() }}
          >清除篩選</button>
        {/if}
      </div>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-muted text-xs border-b border-border">
            <th class="text-left px-5 py-2">股票</th>
            <th class="text-left px-3 py-2">分析時間</th>
            <th class="text-right px-3 py-2">現價</th>
            <th class="text-right px-5 py-2">操作</th>
          </tr>
        </thead>
        <tbody>
          {#if historyLoading}
            <tr><td colspan="4" class="px-5 py-6 text-center text-muted">載入中...</td></tr>
          {:else if history.length === 0}
            <tr><td colspan="4" class="px-5 py-6 text-center text-muted">尚無歷史紀錄，輸入股票代號分析看看</td></tr>
          {:else}
            {#each history as h (h.id)}
              {#if confirmDeleteId === h.id}
                <tr class="border-b border-border/50 bg-red-900/20">
                  <td class="px-5 py-2 text-xs text-gray-300" colspan="2">
                    確定刪除 <span class="font-semibold text-white">{h.symbol}（{formatDateTime(h.analyzed_at)}）</span> 這筆分析嗎？
                  </td>
                  <td class="px-5 py-2 text-right" colspan="2">
                    <div class="flex gap-2 justify-end">
                      <button
                        class="text-xs px-2.5 py-1 border border-border text-muted hover:text-white rounded transition-colors"
                        on:click={() => (confirmDeleteId = null)}
                      >取消</button>
                      <button
                        class="text-xs px-2.5 py-1 bg-red-600 hover:bg-red-500 text-white rounded transition-colors disabled:opacity-50"
                        disabled={deletingId === h.id}
                        on:click={() => doDelete(h.id)}
                      >{deletingId === h.id ? '刪除中...' : '刪除'}</button>
                    </div>
                  </td>
                </tr>
              {:else}
                <tr
                  class="border-b border-border/50 hover:bg-border/20 cursor-pointer transition-colors group
                         {current?.id === h.id ? 'bg-indigo-900/20' : ''}"
                  on:click={() => selectHistory(h)}
                >
                  <td class="px-5 py-2 text-white font-medium">{h.symbol}</td>
                  <td class="px-3 py-2 text-muted text-xs font-mono">{formatDateTime(h.analyzed_at)}</td>
                  <td class="px-3 py-2 text-right font-mono">{fmt(h.current_price)}</td>
                  <td class="px-5 py-2 text-right">
                    <div class="flex gap-2 justify-end">
                      <button
                        class="text-xs px-2.5 py-1 border border-fall/40 text-fall hover:bg-fall/10 rounded transition-colors"
                        on:click|stopPropagation={() => (confirmDeleteId = h.id)}
                      >刪除</button>
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</Layout>
