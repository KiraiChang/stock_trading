<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { currentRoute } from '../lib/stores/router'
  import {
    fetchSymbolCandidates,
    fetchSymbolFacets,
    type SymbolCandidatesResult,
    type SymbolFacets,
  } from '../lib/api/stockSymbols'
  import {
    triggerBackfill,
    getBackfillJob,
    type MarketBackfillJob,
    type MarketBackfillStatus,
  } from '../lib/api/market'
  import { ApiError } from '../lib/api/client'
  import { pollUntilTerminal, stalledMessage, DEFAULT_STALL_MINUTES } from '../lib/utils/jobPolling'

  // ── ① 產生候選清單 ────────────────────────────────────────────
  // 選項全部來自 /stock-symbols/facets，不寫死也不要使用者手打——
  // 值是 TWSE ISIN 的原始中文分類，打錯的後果是 HTTP 200 + 0 筆，
  // 與「條件真的沒匹配」在畫面上無法區分。
  let facets: SymbolFacets | null = null
  let facetsError = ''
  // 預設勾股票與 ETF：主檔 94% 是認購（售）權證，而回補是 5 req/min 的長任務。
  // 權證仍然選得到（選單完整列出並標示筆數），但要使用者自己勾。
  let selectedTypes: string[] = ['股票', 'ETF']
  let selectedIndustries: string[] = []
  // 預設 9：實測 股票,ETF + per_industry=9 得到 293 檔股票 + 354 檔 ETF = 647 檔，
  // 對上 T-040 計畫書的「≈650 檔」。ETF 的 industry 是空字串、不受此上限約束。
  let perIndustry = 9
  // **預設留空**：「上市滿 N 年」是 Step 3 的選取規則，不是 Step 1 的。
  // Step 1 只量 ATR% 分佈、每檔抓 130 天，帶上 5 年會讓 ETF 從 354 檔掉到 199 檔，
  // 漏掉四成母體——而 ETF 是目前唯一填得進 LOW bucket 的類型。
  let listedYears: number | null = null
  let candidateLimit = 1000

  let generating = false
  let candidateError = ''
  let candidates: SymbolCandidatesResult | null = null
  let showAllIndustries = false
  let copyMessage = ''

  // ── ② 回補 ────────────────────────────────────────────────────
  let symbolsInput = ''
  // 130 天 = VOLATILITY_PROFILE_LOOKBACK（60 個交易日）＋ 假日 buffer，即 Step 1 的需求。
  let days = 130
  let submitting = false
  let backfillError = ''
  let backfillJob: MarketBackfillJob | null = null
  let stopPolling: (() => void) | null = null

  const statusText: Record<MarketBackfillStatus, string> = {
    pending: '排隊中', running: '回補中', done: '完成', partial: '部分成功', failed: '失敗',
  }
  const statusClass: Record<MarketBackfillStatus, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    partial: 'bg-yellow-900/40 text-yellow-400',
    failed: 'bg-red-900/40 text-red-400',
  }

  onMount(async () => {
    try {
      // **帶著已選類型抓**：不帶的話產業清單會混入只存在於創新板（30 檔）與
      // 特別股（26 檔）的產業，使用者在「股票+ETF」之下點了它必然 0 筆，
      // 卻會看到「主檔存的是中文」這種怪罪他打錯字的訊息。
      // security_types 清單不受此參數影響（後端只縮放 industries），選單仍然完整。
      facets = await fetchSymbolFacets(selectedTypes)
    } catch (err) {
      facetsError = err instanceof ApiError ? err.message : '載入篩選選項失敗'
    }
  })

  // 證券類型改變時重抓產業清單：ETF 與權證的 industry 全是空字串，
  // 不縮放的話產業選單會列出一堆對當前類型根本不適用的選項。
  // 請求序號：快速連點類型按鈕時會有多個 in-flight 請求，較舊的回應若後到，
  // 會把產業清單換成錯誤的縮放結果，並據此把使用者已選的產業默默剔除。
  // 這與 jobPolling 裡處理的是同一類競態。
  let facetsRequestId = 0
  $: void refreshIndustries(selectedTypes)
  async function refreshIndustries(types: string[]) {
    if (!facets) return
    const requestId = ++facetsRequestId
    try {
      const next = await fetchSymbolFacets(types)
      if (requestId !== facetsRequestId) return // 已有更新的請求，這份結果作廢
      facets = { security_types: facets.security_types, industries: next.industries }
      // 縮放後不存在的產業要從已選清單移除，否則會送出永遠 0 筆的條件
      const available = new Set(next.industries.map((f) => f.value))
      selectedIndustries = selectedIndustries.filter((v) => available.has(v))
    } catch {
      // 產業清單刷新失敗不阻斷主流程，沿用上一次的清單
    }
  }

  function toggle(list: string[], value: string): string[] {
    return list.includes(value) ? list.filter((v) => v !== value) : [...list, value]
  }

  // 元件在 await triggerBackfill 期間被銷毀時，onDestroy 已經跑過，
  // 之後才建立的 interval 沒有人會清掉——650 檔的任務會讓它每 3 秒打一次 API 打上數小時。
  let destroyed = false
  onDestroy(() => {
    destroyed = true
    stopPolling?.()
  })

  $: industryEntries = candidates
    ? Object.entries(candidates.by_industry)
        .filter(([name]) => name !== '')
        .sort((a, b) => b[1] - a[1])
    : []
  $: visibleIndustries = showAllIndustries ? industryEntries : industryEntries.slice(0, 10)
  $: unclassifiedCount = candidates?.by_industry[''] ?? 0

  $: pendingSymbols = parseSymbols(symbolsInput)
  // FinMind 節流是 5 req/min，一檔一個請求。按下去要跑一小時的事，
  // 使用者有權在按之前就知道。
  $: estimatedMinutes = Math.ceil(pendingSymbols.length / 5)
  $: progressPct =
    backfillJob && backfillJob.symbols_total > 0
      ? Math.round((backfillJob.symbols_done / backfillJob.symbols_total) * 100)
      : 0

  function parseSymbols(raw: string): string[] {
    return raw
      .split(/[,\s]+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
  }

  function formatDuration(minutes: number): string {
    if (minutes < 60) return `約 ${minutes} 分鐘`
    const h = Math.floor(minutes / 60)
    const m = minutes % 60
    return m === 0 ? `約 ${h} 小時` : `約 ${h} 小時 ${m} 分鐘`
  }

  async function generate() {
    // 全部取消勾選時後端會落回預設的 股票,ETF——那與旁邊「產業不選 = 不限」的
    // 語意相反，使用者會以為自己在查全市場，實際拿到的是 647 檔。擋在這裡講清楚。
    if (selectedTypes.length === 0) {
      candidateError = '請至少選一種證券類型。全部不選不等於「不限」——證券類型必須明確指定。'
      return
    }
    generating = true
    candidateError = ''
    try {
      candidates = await fetchSymbolCandidates({
        securityTypes: selectedTypes,
        industries: selectedIndustries,
        listedYears: listedYears ?? undefined,
        perIndustry: perIndustry || undefined,
        limit: candidateLimit || undefined,
      })
      if (candidates.count === 0) {
        candidateError =
          '沒有符合條件的標的。最常見原因是證券類型或產業名稱與主檔的實際值不符——' +
          '主檔存的是中文（例如「股票」「ETF」「半導體業」）。'
      }
    } catch (err) {
      candidates = null
      candidateError = err instanceof ApiError ? err.message : '產生候選清單失敗，請稍後再試'
    } finally {
      generating = false
    }
  }

  function useAsBackfillList() {
    if (candidates) symbolsInput = candidates.symbols.join(',')
  }

  // 這個頁面由後端的 embed dist 提供，實務上多半是區網的純 HTTP，
  // 而 navigator.clipboard 只在 secure context 存在——存取 .writeText 會直接丟 TypeError。
  // **不能默默吞掉**：代號沒有顯示在畫面上（要按「加入下方回補清單」才會進 textarea），
  // 失敗又無訊息的話按鈕就是個什麼都不做的按鈕。
  async function copySymbols() {
    if (!candidates) return
    copyMessage = ''
    try {
      await navigator.clipboard.writeText(candidates.symbols.join(','))
      copyMessage = `已複製 ${candidates.symbols.length} 個代號`
    } catch {
      copyMessage = '無法存取剪貼簿（非 HTTPS 或權限被拒）。請改按「加入下方回補清單」，再從文字框自行複製。'
    }
  }

  async function submitBackfill() {
    const symbols = pendingSymbols
    if (symbols.length === 0) {
      backfillError = '請先產生候選清單，或直接在下方輸入股票代號'
      return
    }
    if (!Number.isFinite(days) || days <= 0) {
      backfillError = '回補天數需大於 0'
      return
    }

    submitting = true
    backfillError = ''
    backfillJob = null
    try {
      const created = await triggerBackfill({ days, symbols })
      if (destroyed) return
      backfillJob = created
      startPolling(created.job_id)
    } catch (err) {
      backfillError = err instanceof ApiError ? err.message : '回補請求送出失敗，請稍後再試'
      submitting = false
    }
  }

  function startPolling(jobId: string) {
    stopPolling?.()
    stopPolling = pollUntilTerminal<MarketBackfillJob>({
      fetch: () => getBackfillJob(jobId),
      isTerminal: (j) => j.status === 'done' || j.status === 'partial' || j.status === 'failed',
      progressOf: (j) => j.symbols_done,
      onUpdate: (j) => { backfillJob = j },
      onSettled: (reason) => {
        // 三種收尾都要解鎖，否則按鈕會鎖死到重新整理為止。
        submitting = false
        if (reason === 'stalled') backfillError = stalledMessage(jobId, DEFAULT_STALL_MINUTES)
        if (reason === 'error') backfillError = '查詢回補狀態失敗（後端可能仍在執行）'
      },
    })
  }
</script>

<Layout>
  <div class="max-w-3xl mx-auto">
    <div class="mb-4">
      <h1 class="text-white font-semibold">評估標的池</h1>
      <p class="text-muted text-xs mt-1">
        產生研究用的候選標的並補齊日 K，供 SR Zone 的 evaluation／sweep 有足夠寬的取樣母體。
        <strong class="text-white">與監控清單分離</strong>——這裡的標的不會進盤中掃描、籌碼同步或訊號評估。
      </p>
    </div>

    <!-- ── ① 產生候選清單 ──────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl mb-6">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">① 產生候選清單</h2>
        <p class="text-muted text-xs mt-1">
          量分佈階段建議用全部 ETF ＋ 各產業分層抽樣的股票。每產業上限是
          <strong class="text-white">在該產業代號區間內等距取樣</strong>，不是取代號最小的前 N 檔
          （台股代號大致依上市資歷編配，取前 N 會讓每個產業只拿到最老最大的那幾檔）。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        {#if candidateError}
          <p class="text-rise text-sm">{candidateError}</p>
        {/if}

        {#if facetsError}
          <p class="text-rise text-sm">{facetsError}</p>
        {/if}

        <!-- 證券類型：全部列出並標示母體筆數。看到「上市認購(售)權證 31,090」
             就知道不該勾——用資訊而不是隱藏來防止誤選。 -->
        <div>
          <span class="text-muted text-xs block mb-1">證券類型（可複選）</span>
          {#if facets}
            <div class="flex flex-wrap gap-2">
              {#each facets.security_types as t}
                <button
                  type="button"
                  class="px-3 py-1.5 rounded-lg text-xs border transition-colors
                         {selectedTypes.includes(t.value)
                           ? 'bg-indigo-600 border-indigo-500 text-white'
                           : 'bg-surface border-border text-muted hover:text-white'}"
                  on:click={() => (selectedTypes = toggle(selectedTypes, t.value))}
                >
                  {t.value}
                  <span class="opacity-70 font-mono ml-1">{t.count.toLocaleString()}</span>
                </button>
              {/each}
            </div>
          {:else}
            <p class="text-muted text-xs">載入中...</p>
          {/if}
        </div>

        <div>
          <span class="text-muted text-xs block mb-1">
            產業（可複選，不選 = 不限）
            {#if facets && facets.industries.length > 0}
              <span class="opacity-70">— 共 {facets.industries.length} 個</span>
            {/if}
          </span>
          {#if facets && facets.industries.length > 0}
            <div class="flex flex-wrap gap-2 max-h-40 overflow-y-auto">
              {#each facets.industries as ind}
                <button
                  type="button"
                  class="px-3 py-1.5 rounded-lg text-xs border transition-colors
                         {selectedIndustries.includes(ind.value)
                           ? 'bg-indigo-600 border-indigo-500 text-white'
                           : 'bg-surface border-border text-muted hover:text-white'}"
                  on:click={() => (selectedIndustries = toggle(selectedIndustries, ind.value))}
                >
                  {ind.value}
                  <span class="opacity-70 font-mono ml-1">{ind.count}</span>
                </button>
              {/each}
            </div>
          {:else if facets}
            <p class="text-muted text-xs">目前選取的證券類型沒有產業分類（ETF 與權證皆無）。</p>
          {/if}
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <label class="block">
            <span class="text-muted text-xs block mb-1">每產業上限（0 = 不限）</span>
            <input
              type="number"
              min="0"
              bind:value={perIndustry}
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </label>

          <label class="block">
            <span class="text-muted text-xs block mb-1">總筆數上限（留空 = 3000）</span>
            <input
              type="number"
              min="1"
              bind:value={candidateLimit}
              class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </label>
        </div>

        <div class="flex gap-3 flex-wrap items-end">
          <label class="block">
            <span class="text-muted text-xs block mb-1">上市滿幾年（留空 = 不限）</span>
            <input
              type="number"
              min="0"
              bind:value={listedYears}
              placeholder="留空"
              class="w-40 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </label>
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors"
            disabled={generating}
            on:click={generate}
          >
            {generating ? '產生中...' : '產生候選清單'}
          </button>
        </div>

        <p class="text-muted text-xs">
          「上市滿 N 年」是最終選股（Step 3）的規則，<strong class="text-white">量分佈階段不要帶</strong>——
          帶 5 年會讓 ETF 從 354 檔掉到 199 檔，而 ETF 是目前唯一落在低波動 bucket 的類型。
        </p>

        {#if candidates && candidates.count > 0}
          <div class="pt-2 space-y-3 border-t border-border">
            <p class="text-white text-sm">
              共 <span class="font-mono">{candidates.count}</span> 檔，涵蓋
              <span class="font-mono">{industryEntries.length}</span> 個產業
              {#if unclassifiedCount > 0}
                <span class="text-muted">（另有 {unclassifiedCount} 檔無產業分類，多為 ETF）</span>
              {/if}
            </p>

            {#if candidates.truncated}
              <p class="text-rise text-xs">
                清單已達總筆數上限而被截斷。<strong>截斷依代號順序</strong>，會整批砍掉高代號的產業，
                正是「每產業上限」要消除的偏斜——請調高上限或收緊條件，不要直接使用這份清單。
              </p>
            {/if}

            {#if industryEntries.length > 0}
              <div class="space-y-1">
                {#each visibleIndustries as [name, n]}
                  <div class="flex items-center gap-2 text-xs">
                    <span class="text-muted w-32 shrink-0 truncate" title={name}>{name}</span>
                    <span class="text-white font-mono w-8 text-right">{n}</span>
                    <div class="flex-1 bg-surface rounded-full h-1.5 overflow-hidden">
                      <div
                        class="bg-indigo-500 h-full"
                        style="width: {Math.round((n / industryEntries[0][1]) * 100)}%"
                      ></div>
                    </div>
                  </div>
                {/each}
                {#if industryEntries.length > 10}
                  <button
                    class="text-xs text-muted hover:text-white transition-colors"
                    on:click={() => (showAllIndustries = !showAllIndustries)}
                  >
                    {showAllIndustries ? '收合' : `顯示全部 ${industryEntries.length} 個產業`}
                  </button>
                {/if}
              </div>
            {/if}

            <div class="flex gap-3">
              <button
                class="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium
                       px-4 py-2 rounded-lg transition-colors"
                on:click={useAsBackfillList}
              >
                加入下方回補清單
              </button>
              <button
                class="text-muted hover:text-white text-sm px-4 py-2 border border-border
                       rounded-lg transition-colors"
                on:click={copySymbols}
              >
                複製代號
              </button>
            </div>
            {#if copyMessage}
              <p class="text-muted text-xs">{copyMessage}</p>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <!-- ── ② 回補這批標的 ──────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl mb-6">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">② 回補這批標的</h2>
        <p class="text-muted text-xs mt-1">
          代號可自行增刪。回補在後端背景執行，離開頁面不影響它繼續跑，但此頁的進度追蹤會中斷——
          屆時可用 job_id 查詢。
        </p>
      </div>

      <div class="px-5 py-4 space-y-3">
        {#if backfillError}
          <p class="text-rise text-sm">{backfillError}</p>
        {/if}

        <textarea
          bind:value={symbolsInput}
          rows="4"
          placeholder="股票代號，逗號分隔"
          class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white font-mono
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        ></textarea>

        <div class="flex gap-3 flex-wrap items-center">
          <label class="text-muted text-xs">
            天數
            <input
              type="number"
              min="1"
              step="10"
              bind:value={days}
              class="ml-2 w-24 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                     focus:outline-none focus:border-indigo-500 transition-colors"
            />
          </label>
          <span class="text-muted text-xs">
            {pendingSymbols.length} 檔，預估 {formatDuration(estimatedMinutes)}
            <span class="opacity-70">（FinMind 節流 5 檔/分）</span>
          </span>
          <button
            class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                   font-medium px-5 py-2 rounded-lg transition-colors ml-auto"
            disabled={submitting || pendingSymbols.length === 0}
            on:click={submitBackfill}
          >
            {submitting ? '回補中...' : '開始回補'}
          </button>
        </div>

        {#if backfillJob}
          <div class="px-3 py-2 bg-surface/60 rounded-lg text-xs space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-muted font-mono">{backfillJob.job_id}</span>
              <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {statusClass[backfillJob.status]}">
                {statusText[backfillJob.status]}
              </span>
              <span class="text-muted">
                {backfillJob.symbols_done}/{backfillJob.symbols_total} 檔，失敗 {backfillJob.symbols_failed}
              </span>
              <span class="text-white font-mono ml-auto">{progressPct}%</span>
            </div>
            <div class="bg-surface rounded-full h-2 overflow-hidden">
              <div class="bg-indigo-500 h-full transition-all" style="width: {progressPct}%"></div>
            </div>
            {#if backfillJob.status === 'failed' || backfillJob.status === 'partial'}
              {#each backfillJob.failures as f}
                <p class="text-rise">{f.symbol}: {f.error}</p>
              {/each}
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <!-- ── ③ 下一步 ────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl">
      <div class="px-5 py-4 border-b border-border">
        <h2 class="text-sm font-semibold text-white">③ 下一步：看波動分佈</h2>
      </div>
      <div class="px-5 py-4 space-y-3">
        <p class="text-muted text-xs">
          回補完成後到「支撐/壓力機率」頁面跑一次 evaluation，在報告的
          <strong class="text-white">波動側寫</strong>區看三個 bucket（LOW / NORMAL / HIGH）的實際分佈。
          判讀入口在那裡，本頁不重做。
        </p>
        <button
          class="text-sm text-indigo-400 hover:text-indigo-300 transition-colors"
          on:click={() => currentRoute.set('sr-zones')}
        >
          前往支撐/壓力機率 →
        </button>
      </div>
    </div>
  </div>
</Layout>
