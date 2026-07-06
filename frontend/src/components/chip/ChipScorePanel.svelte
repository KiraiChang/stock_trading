<script lang="ts">
  import type { ChipSummary } from '../../lib/api/chips'

  export let summary: ChipSummary | null = null
  export let loading = false

  const signalText: Record<string, string> = {
    BULLISH: '籌碼偏多', BEARISH: '籌碼偏空', NEUTRAL: '中性', RISK: '風險升高',
  }
  // 台股慣例：買超/上漲用紅色（text-rise），賣超/下跌用綠色（text-fall）
  const signalClass: Record<string, string> = {
    BULLISH: 'bg-red-900/40 text-rise',
    BEARISH: 'bg-green-900/40 text-fall',
    NEUTRAL: 'bg-gray-700/60 text-gray-300',
    RISK: 'bg-yellow-900/40 text-yellow-400',
  }

  function signedClass(v: number): string {
    if (v > 0) return 'text-rise'
    if (v < 0) return 'text-fall'
    return 'text-muted'
  }

  // 內部單位為股，顯示換算成張（見設計文件單位規則）
  function fmtLots(v: number): string {
    const lots = v / 1000
    const sign = lots > 0 ? '+' : ''
    return `${sign}${lots.toLocaleString('zh-TW', { maximumFractionDigits: 0 })} 張`
  }

  function consecutiveDaysText(days: number): string {
    if (days > 0) return `連續買超 ${days} 日`
    if (days < 0) return `連續賣超 ${-days} 日`
    return '—'
  }
</script>

<div class="bg-panel border border-border rounded-xl px-5 py-4">
  {#if loading}
    <p class="text-muted text-xs">載入中...</p>
  {:else if !summary}
    <p class="text-muted text-xs text-center py-4">尚無籌碼資料，請先執行同步</p>
  {:else}
    <div class="flex items-center justify-between flex-wrap gap-2 mb-4">
      <div class="flex items-center gap-2">
        <span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold {signalClass[summary.signal] ?? 'bg-gray-700/60 text-gray-300'}">
          {signalText[summary.signal] ?? summary.signal}
        </span>
        <span class="text-white font-mono text-lg">{summary.totalScore.toFixed(1)}</span>
        <span class="text-muted text-xs">/ 100</span>
      </div>
      <span class="text-muted text-xs font-mono">{summary.date}</span>
    </div>

    {#if summary.institutional}
      <p class="text-muted text-xs mb-1.5">三大法人</p>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs mb-4">
        <div>
          <p class="text-muted mb-1">外資買賣超</p>
          <p class="font-mono {signedClass(summary.institutional.foreignNetBuy)}">{fmtLots(summary.institutional.foreignNetBuy)}</p>
        </div>
        <div>
          <p class="text-muted mb-1">投信買賣超</p>
          <p class="font-mono {signedClass(summary.institutional.investmentTrustNetBuy)}">{fmtLots(summary.institutional.investmentTrustNetBuy)}</p>
        </div>
        <div>
          <p class="text-muted mb-1">自營商買賣超</p>
          <p class="font-mono {signedClass(summary.institutional.dealerNetBuy)}">{fmtLots(summary.institutional.dealerNetBuy)}</p>
        </div>
        <div>
          <p class="text-muted mb-1">外資連續買賣超</p>
          <p class="font-mono text-white">{consecutiveDaysText(summary.institutional.consecutiveDays)}</p>
        </div>
      </div>
    {/if}

    {#if summary.margin}
      <p class="text-muted text-xs mb-1.5">融資融券</p>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs mb-4">
        <div>
          <p class="text-muted mb-1">融資餘額</p>
          <p class="font-mono text-white">{fmtLots(summary.margin.marginBalance).replace('+', '')}</p>
        </div>
        <div>
          <p class="text-muted mb-1">融資增減</p>
          <p class="font-mono {signedClass(summary.margin.marginChange)}">{fmtLots(summary.margin.marginChange)}</p>
        </div>
        <div>
          <p class="text-muted mb-1">融券餘額</p>
          <p class="font-mono text-white">{fmtLots(summary.margin.shortBalance).replace('+', '')}</p>
        </div>
        <div>
          <p class="text-muted mb-1">融券增減</p>
          <p class="font-mono {signedClass(summary.margin.shortChange)}">{fmtLots(summary.margin.shortChange)}</p>
        </div>
      </div>
    {/if}

    {#if summary.broker}
      <p class="text-muted text-xs mb-1.5">主力分點</p>
      <div class="grid grid-cols-2 gap-3 text-xs mb-4">
        <div>
          <p class="text-muted mb-1">前10大分點買超合計</p>
          <p class="font-mono {signedClass(summary.broker.topNetBuy)}">{fmtLots(summary.broker.topNetBuy)}</p>
        </div>
        <div>
          <p class="text-muted mb-1">籌碼集中度</p>
          <p class="font-mono text-white">{(summary.broker.concentration * 100).toFixed(1)}%</p>
        </div>
      </div>
    {/if}

    {#if summary.reason.length > 0}
      <div class="border-t border-border pt-3">
        <p class="text-muted text-xs mb-1.5">判斷理由</p>
        <ul class="text-xs text-white space-y-1 list-disc list-inside">
          {#each summary.reason as r}
            <li>{r}</li>
          {/each}
        </ul>
      </div>
    {/if}
  {/if}
</div>
