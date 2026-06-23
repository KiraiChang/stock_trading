<script lang="ts">
  import { signals } from '../../lib/stores/signals'
  import { formatPrice, formatTime } from '../../lib/utils/format'
  import { directionColor } from '../../lib/utils/color'

  const typeLabel: Record<string, string> = {
    BREAKOUT: '突破',
    BREAKDOWN: '跌破',
    VOLUME_SPIKE: '爆量',
  }
</script>

<div class="bg-panel rounded-lg border border-border overflow-hidden">
  <div class="px-4 py-3 border-b border-border flex items-center justify-between">
    <h2 class="text-sm font-semibold text-white">訊號記錄</h2>
    {#if $signals.length > 0}
      <span class="text-xs text-muted">{$signals.length} 筆</span>
    {/if}
  </div>

  <div class="overflow-y-auto max-h-52">
    {#each $signals as sig (sig.id)}
      <div class="flex items-start gap-3 px-4 py-2.5 border-b border-border/50 hover:bg-border/20">
        <span class="shrink-0 mt-0.5 px-2 py-0.5 rounded text-xs font-bold text-white {directionColor(sig.direction)}">
          {sig.direction}
        </span>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="font-semibold text-white text-sm">{sig.symbol}</span>
            <span class="text-xs text-muted">{typeLabel[sig.signal_type] ?? sig.signal_type}</span>
            <span class="text-xs font-mono text-gray-300">{formatPrice(sig.price)}</span>
            {#if sig.vol_ratio > 0}
              <span class="text-xs text-rise">{sig.vol_ratio.toFixed(1)}x</span>
            {/if}
          </div>
          {#if sig.note}
            <div class="text-xs text-muted mt-0.5 truncate">{sig.note}</div>
          {/if}
        </div>
        <span class="text-xs text-muted shrink-0">{formatTime(sig.ts)}</span>
      </div>
    {:else}
      <div class="px-4 py-8 text-center text-muted text-sm">尚無訊號</div>
    {/each}
  </div>
</div>
