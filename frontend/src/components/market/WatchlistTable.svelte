<script lang="ts">
  import { watchlist, quotes, selectedSymbol } from '../../lib/stores/market'
  import { formatPrice, formatChangePct, formatVolume } from '../../lib/utils/format'
  import { priceColor, trendColor } from '../../lib/utils/color'

  const trendLabel: Record<string, string> = {
    BULLISH: '多頭',
    BEARISH: '空頭',
    SIDEWAYS: '盤整',
    '': '-',
  }
</script>

<div class="bg-panel rounded-lg overflow-hidden border border-border">
  <div class="px-4 py-3 border-b border-border">
    <h2 class="text-sm font-semibold text-white">監控清單</h2>
  </div>

  <div class="overflow-x-auto">
    <table class="w-full text-sm">
      <thead>
        <tr class="text-muted border-b border-border text-xs">
          <th class="text-left px-4 py-2">代號</th>
          <th class="text-right px-3 py-2">收盤</th>
          <th class="text-right px-3 py-2">漲跌%</th>
          <th class="text-right px-3 py-2">量比</th>
          <th class="text-right px-3 py-2">RSI</th>
          <th class="text-center px-3 py-2">趨勢</th>
          <th class="text-center px-3 py-2">訊號</th>
        </tr>
      </thead>
      <tbody>
        {#each $watchlist as item}
          {@const q = $quotes.get(item.symbol)}
          <tr
            class="border-b border-border/50 hover:bg-border/30 cursor-pointer transition-colors
                   {$selectedSymbol === item.symbol ? 'bg-indigo-900/30' : ''}"
            on:click={() => selectedSymbol.set(item.symbol)}
          >
            <td class="px-4 py-2">
              <div class="font-medium text-white">{item.symbol}</div>
              <div class="text-xs text-muted truncate max-w-20">{item.name}</div>
            </td>
            <td class="px-3 py-2 text-right font-mono">
              {q ? formatPrice(q.close) : '-'}
            </td>
            <td class="px-3 py-2 text-right font-mono {q ? priceColor(q.change) : 'text-flat'}">
              {q ? formatChangePct(q.changePct) : '-'}
            </td>
            <td class="px-3 py-2 text-right font-mono {q && q.volRatio >= 2 ? 'text-rise font-semibold' : 'text-gray-300'}">
              {q ? `${q.volRatio.toFixed(1)}x` : '-'}
            </td>
            <td class="px-3 py-2 text-right font-mono
              {q && q.rsi14 >= 70 ? 'text-rise' : q && q.rsi14 <= 30 ? 'text-fall' : 'text-gray-300'}">
              {q ? q.rsi14.toFixed(1) : '-'}
            </td>
            <td class="px-3 py-2 text-center">
              {#if q}
                <span class="text-xs {trendColor(q.trend)}">{trendLabel[q.trend] ?? '-'}</span>
              {:else}
                <span class="text-muted">-</span>
              {/if}
            </td>
            <td class="px-3 py-2 text-center">
              {#if q?.hasSignal}
                <span class="inline-block w-2 h-2 rounded-full bg-rise animate-pulse"></span>
              {/if}
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" class="px-4 py-8 text-center text-muted text-sm">尚無監控股票</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
