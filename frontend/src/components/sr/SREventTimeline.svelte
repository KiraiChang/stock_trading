<script lang="ts">
  import { getEventTimeline } from '../../lib/api/srZones'
  import type { SREventTimeline } from '../../lib/api/srZones'
  import { chainEndNote, chainZoneLabel, isDecisionVisible, maxGapDays, splitChains } from '../../lib/utils/eventTimeline'

  export let symbol: string
  export let timeframe = '1d'
  /**
   * 目前顯示的那一筆分析。
   *
   * **要納入快取鍵**：重跑分析時 `current` 是被新分析直接覆寫、中間不會變成 null，
   * 元件實例不會重建。只用 symbol/timeframe 當鍵的話，新的 transitions 已經落地，
   * 面板卻還停在舊的鏈，而且畫面上沒有任何過期提示。
   */
  export let analysisId: number | null = null

  let open = false
  let loading = false
  let error = ''
  let timeline: SREventTimeline | null = null
  // 記住**已載入過的是哪一組**（成功或失敗都記）：切換股票或重跑分析時要重抓，
  // 而不是顯示上一組的鏈；失敗時不記的話，第一次就失敗的面板會卡住不再重抓。
  let loadedFor = ''

  $: wanted = `${symbol}:${timeframe}:${analysisId ?? ''}`

  async function toggle() {
    open = !open
    if (open) await ensureLoaded()
  }

  async function ensureLoaded() {
    if (loading) return
    const key = wanted
    if (timeline && loadedFor === key) return
    loading = true
    error = ''
    try {
      timeline = await getEventTimeline(symbol, timeframe)
    } catch (e) {
      error = e instanceof Error ? e.message : '載入失敗'
      timeline = null
    } finally {
      // **失敗也要記**：否則 loadedFor 永遠是空字串，下方的作廢判斷會因 falsy 跳過，
      // 換股票後既不清錯誤訊息也不重抓，面板會一直掛著上一檔的錯誤。
      loadedFor = key
      loading = false
    }
  }

  // 換標的或換分析時把已載入的內容作廢；展開狀態保留，重新抓新的那一組。
  $: if (loadedFor && loadedFor !== wanted) {
    timeline = null
    error = ''
    if (open) ensureLoaded()
  }

  $: split = timeline ? splitChains(timeline.chains) : { open: [], closed: [] }
  $: gap = timeline ? maxGapDays(timeline.snapshots) : 0
  // 只寫不讀的事實紀錄有幾條。**不從列表裡濾掉**，只是讓人知道總數裡有多少不算訊號。
  $: factOnly = timeline ? timeline.chains.filter((c) => !isDecisionVisible(c)).length : 0

  function fmtDate(ts: string): string {
    return new Date(ts).toLocaleDateString('en-CA', { timeZone: 'Asia/Taipei' })
  }

  const stateClass: Record<string, string> = {
    CANDIDATE: 'bg-gray-700/60 text-gray-300 border-border',
    ACTIVE: 'bg-amber-900/40 text-amber-300 border-amber-700/50',
    CONFIRMED: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/50',
    RESOLVED: 'bg-sky-900/40 text-sky-300 border-sky-700/50',
    EXPIRED: 'bg-gray-800/60 text-muted border-border',
  }
  const directionClass: Record<string, string> = {
    BULLISH: 'text-emerald-300',
    BEARISH: 'text-rose-300',
    NEUTRAL: 'text-muted',
  }
</script>

<div class="border border-border/70 rounded-lg p-3 bg-panel/50 mb-4 text-xs">
  <button class="flex items-center gap-2 text-muted hover:text-white transition-colors" on:click={toggle}>
    <span>{open ? '▾' : '▸'}</span>
    <span>Event Timeline（跨分析事件鏈）</span>
    {#if timeline}
      <span class="text-[10px]">
        {split.open.length} 條進行中 / {timeline.chains.length} 條{#if factOnly > 0}（{factOnly} 條不參與決策）{/if}
      </span>
    {/if}
  </button>

  {#if open}
    {#if loading}
      <p class="text-muted mt-3">載入中...</p>
    {:else if error}
      <p class="text-rise mt-3">{error}</p>
    {:else if timeline}
      <!-- **空白不等於沒事發生**：timeline 的解析度等於 SR 分析的執行頻率。 -->
      <div class="mt-3 mb-3 text-[11px] text-muted space-y-0.5">
        {#if timeline.identity_since}
          <p>身分層自 {fmtDate(timeline.identity_since)} 起有紀錄；更早的分析沒有事件鏈（刻意不回填）。</p>
        {/if}
        <p>
          這段期間共 {timeline.snapshots.length} 次分析{#if gap > 1}，最大間隔
            <span class="text-amber-300">{gap} 天</span>——鏈上的空白代表那幾天沒有分析，不代表沒有事件{/if}。
        </p>
      </div>

      {#if timeline.chains.length === 0}
        <p class="text-muted">這段期間沒有任何事件鏈。</p>
      {:else}
        <!-- **全部終結是常態**（實測 2330 的 28 條鏈全已終結）。少了這一行，展開後
             上半部會是空的，看起來像載入失敗而不是「現在沒有進行中的鏈」。 -->
        {#if split.open.length === 0}
          <p class="text-muted mb-2">目前沒有進行中的鏈——{split.closed.length} 條都已終結，展開下方查看。</p>
        {/if}
        {#each split.open as c (c.event_uid)}
          {@const note = chainEndNote(c)}
          <div class="border-l-2 border-amber-700/50 pl-3 py-2 mb-2">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="{isDecisionVisible(c) ? 'text-white' : 'text-gray-400'} font-medium">{c.event_family}</span>
              <!-- **只寫不讀的事實紀錄**（階段 D 的隔離旗標）：這種鏈不進任何決策桶，
                   不影響 Bias 或進場。少了這個標記，RESISTANCE_BREAKOUT / CONFIRMED /
                   BULLISH 會被讀成突破買訊。標記而不隱藏——它們是人工判讀的依據。 -->
              {#if !isDecisionVisible(c)}
                <span class="px-1.5 py-0.5 rounded bg-gray-800/60 border border-border text-[10px] text-gray-400"
                      title="事實紀錄：這個事件不進任何決策桶，不影響 Bias 或進場">事實紀錄・不參與決策</span>
              {/if}
              {#if c.seq > 1}
                <!-- seq > 1 是**新的一條鏈**，不是舊鏈復活 -->
                <span class="px-1.5 py-0.5 rounded bg-surface border border-border text-[10px]">第 {c.seq} 條</span>
              {/if}
              <span class="px-2 py-0.5 rounded-full border text-[10px] {stateClass[c.final_state] ?? 'bg-surface text-white border-border'}">{c.final_state}</span>
              <span class="text-[10px] {directionClass[c.direction] ?? 'text-muted'}">{c.direction}</span>
              <span class="text-muted text-[10px]" title={c.zone_key ?? ''}>{chainZoneLabel(c)}</span>
              {#if note}
                <span class="text-[10px] {note.emphasise ? 'text-amber-300' : 'text-muted'}">{note.label}</span>
              {/if}
            </div>
            <p class="text-muted text-[10px] mt-0.5">{fmtDate(c.first_seen_at)} ~ {fmtDate(c.last_seen_at)}</p>
            <div class="mt-1.5 space-y-1">
              {#each c.transitions as t}
                <div class="flex items-baseline gap-2">
                  <span class="text-muted font-mono text-[10px] w-[68px] shrink-0">{fmtDate(t.occurred_at)}</span>
                  <!-- from_state 留白＝鏈的誕生 -->
                  <span class="text-white">{#if t.from_state}{t.from_state} → {/if}{t.state}</span>
                  {#if t.event_type}<span class="text-muted text-[10px]">{t.event_type}</span>{/if}
                </div>
              {/each}
            </div>
          </div>
        {/each}

        {#if split.closed.length > 0}
          <details class="mt-2">
            <summary class="cursor-pointer text-muted hover:text-white transition-colors">
              已終結的鏈（{split.closed.length} 條）
            </summary>
            <div class="mt-2 space-y-2">
              {#each split.closed as c (c.event_uid)}
                {@const note = chainEndNote(c)}
                <div class="border-l-2 border-border pl-3 py-1.5">
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="{isDecisionVisible(c) ? 'text-gray-300' : 'text-gray-500'}">{c.event_family}</span>
                    {#if !isDecisionVisible(c)}
                      <span class="px-1.5 py-0.5 rounded bg-gray-800/60 border border-border text-[10px] text-gray-400"
                            title="事實紀錄：這個事件不進任何決策桶，不影響 Bias 或進場">事實紀錄・不參與決策</span>
                    {/if}
                    {#if c.seq > 1}
                      <span class="px-1.5 py-0.5 rounded bg-surface border border-border text-[10px]">第 {c.seq} 條</span>
                    {/if}
                    <span class="px-2 py-0.5 rounded-full border text-[10px] {stateClass[c.final_state] ?? 'bg-surface text-white border-border'}">{c.final_state}</span>
                    <span class="text-muted text-[10px]" title={c.zone_key ?? ''}>{chainZoneLabel(c)}</span>
                    {#if note}
                      <span class="text-[10px] {note.emphasise ? 'text-amber-300' : 'text-muted'}">{note.label}</span>
                    {/if}
                  </div>
                  <p class="text-muted text-[10px] mt-0.5">
                    {fmtDate(c.first_seen_at)} ~ {fmtDate(c.last_seen_at)}・{c.transitions.length} 步
                  </p>
                </div>
              {/each}
            </div>
          </details>
        {/if}
      {/if}
    {/if}
  {/if}
</div>
