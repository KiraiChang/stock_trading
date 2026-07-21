<script lang="ts">
  import type { SRChipSummary } from '../../lib/api/srZones'

  // SR Zone 專用的整檔籌碼拆解面板（跟 ChipScorePanel 不同：這裡吃的是 -100~100
  // 的子分數，不是張數 net-buy）。分析快照當下對齊的籌碼，非即時最新值。
  export let summary: SRChipSummary | null = null

  const signalText: Record<string, string> = {
    BULLISH: '籌碼偏多',
    WEAK_BULLISH: '籌碼略偏多',
    BEARISH: '籌碼偏空',
    WEAK_BEARISH: '籌碼略偏空',
    NEUTRAL: '中性',
    RISK: '風險升高',
  }
  // 台股慣例：偏多用紅（text-rise），偏空用綠（text-fall）
  const signalClass: Record<string, string> = {
    BULLISH: 'bg-red-900/40 text-rise',
    WEAK_BULLISH: 'bg-red-900/20 text-red-300',
    BEARISH: 'bg-green-900/40 text-fall',
    WEAK_BEARISH: 'bg-green-900/20 text-green-300',
    NEUTRAL: 'bg-gray-700/60 text-gray-300',
    RISK: 'bg-yellow-900/40 text-yellow-400',
  }

  function signedClass(v: number): string {
    if (v > 0) return 'text-rise'
    if (v < 0) return 'text-fall'
    return 'text-muted'
  }

  function fmtScore(v: number | null): string {
    if (v === null || v === undefined) return '—'
    const sign = v > 0 ? '+' : ''
    return `${sign}${v.toFixed(0)}`
  }

  function fmtPct(v: number | null | undefined): string {
    if (v === null || v === undefined) return '—'
    return `${(v * 100).toFixed(0)}%`
  }

  // 分向長條：中心為 0，正值往右（紅）、負值往左（綠），滿格 = |100|。
  function barPct(v: number): number {
    return Math.min(Math.abs(v) / 100, 1) * 50
  }

  $: subScores = summary && !summary.missing
    ? [
        { label: '法人', value: summary.institutional_score },
        { label: '融資', value: summary.margin_score },
        { label: '分點', value: summary.broker_score },
        { label: '集中度', value: summary.concentration_score },
      ]
    : []
</script>

<div class="bg-panel border border-border rounded-xl px-5 py-4">
  <div class="flex items-center justify-between gap-2 mb-1">
    <p class="text-xs font-semibold text-white">籌碼拆解</p>
    <span class="text-[11px] text-muted">整檔一次，非逐區間</span>
  </div>

  {#if !summary || summary.missing}
    <p class="text-muted text-xs py-3">此股票尚無籌碼資料（本項在模型與評分中以中性計）。</p>
  {:else}
    <div class="flex items-center gap-2 mb-4">
      <span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold {signalClass[summary.signal ?? 'NEUTRAL'] ?? 'bg-gray-700/60 text-gray-300'}">
        {signalText[summary.signal ?? 'NEUTRAL'] ?? summary.signal}
      </span>
      <span class="font-mono text-lg {signedClass(summary.score ?? 0)}">{fmtScore(summary.score)}</span>
      <span class="text-muted text-xs">/ 100（−100~100）</span>
    </div>
    <div class="grid grid-cols-2 gap-2 mb-4 text-xs">
      <div class="border border-border/70 rounded p-2">
        <p class="text-muted mb-1">覆蓋率</p>
        <p class="text-white font-mono">{fmtPct(summary.coverage)}</p>
      </div>
      <div class="border border-border/70 rounded p-2">
        <p class="text-muted mb-1">Effective Impact</p>
        <p class="font-mono {signedClass(summary.effective_score ?? 0)}">{fmtScore(summary.effective_score ?? null)}</p>
      </div>
    </div>

    <div class="space-y-2.5">
      {#each subScores as s}
        <div class="grid grid-cols-[3rem_1fr_3rem] items-center gap-2 text-xs">
          <span class="text-muted">{s.label}</span>
          <div class="relative h-2 rounded-full bg-surface/60 overflow-hidden">
            <!-- 中心線 -->
            <div class="absolute top-0 bottom-0 left-1/2 w-px bg-border"></div>
            {#if s.value === null || s.value === undefined}
              <div class="absolute top-0 bottom-0 left-1/2 w-px bg-border"></div>
            {:else if s.value >= 0}
              <div class="absolute top-0 bottom-0 left-1/2 bg-rise/70 rounded-r-full" style="width: {barPct(s.value)}%"></div>
            {:else}
              <div class="absolute top-0 bottom-0 bg-fall/70 rounded-l-full" style="right: 50%; width: {barPct(s.value)}%"></div>
            {/if}
          </div>
          <span class="font-mono text-right {signedClass(s.value ?? 0)}">{fmtScore(s.value)}</span>
        </div>
      {/each}
    </div>

    <p class="text-[11px] text-muted mt-3 leading-relaxed">
      籌碼透過兩條路徑影響評分：直接加權分量（總分的 15%）與 v3 機率模型特徵，兩者不是重複計分。
    </p>
  {/if}
</div>
