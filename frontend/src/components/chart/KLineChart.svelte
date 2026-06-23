<script lang="ts">
  import { onMount, onDestroy, afterUpdate } from 'svelte'
  import { createChart, type IChartApi, type ISeriesApi, ColorType } from 'lightweight-charts'
  import { fetchCandles, type Candle } from '../../lib/api/candles'

  export let symbol: string

  let container: HTMLDivElement
  let chart: IChartApi | null = null
  let candleSeries: ISeriesApi<'Candlestick'> | null = null
  let loading = false
  let error = ''

  async function loadData(sym: string) {
    if (!chart || !sym) return
    loading = true
    error = ''
    try {
      const candles = await fetchCandles(sym, '1d', 120)
      const chartData = candles.map((c: Candle) => ({
        time: c.ts.substring(0, 10) as `${number}-${number}-${number}`,
        open: c.open,
        high: c.high,
        low: c.low,
        close: c.close,
      }))
      candleSeries?.setData(chartData)
      chart?.timeScale().fitContent()
    } catch (e) {
      error = `載入失敗: ${symbol}`
    } finally {
      loading = false
    }
  }

  onMount(() => {
    chart = createChart(container, {
      layout: {
        background: { type: ColorType.Solid, color: '#1a1a2e' },
        textColor: '#9ca3af',
      },
      grid: {
        vertLines: { color: '#2a2a4a' },
        horzLines: { color: '#2a2a4a' },
      },
      crosshair: { mode: 1 },
      width: container.clientWidth,
      height: 380,
    })

    candleSeries = chart.addCandlestickSeries({
      upColor: '#e74c3c',
      downColor: '#2ecc71',
      borderUpColor: '#e74c3c',
      borderDownColor: '#2ecc71',
      wickUpColor: '#e74c3c',
      wickDownColor: '#2ecc71',
    })

    const ro = new ResizeObserver(() => {
      chart?.applyOptions({ width: container.clientWidth })
    })
    ro.observe(container)

    loadData(symbol)

    return () => {
      ro.disconnect()
    }
  })

  afterUpdate(() => {
    loadData(symbol)
  })

  onDestroy(() => {
    chart?.remove()
    chart = null
  })
</script>

<div class="bg-panel rounded-lg border border-border overflow-hidden">
  <div class="px-4 py-3 border-b border-border flex items-center justify-between">
    <h2 class="text-sm font-semibold text-white">
      {symbol ? `${symbol} 日K` : 'K 線圖'}
    </h2>
    {#if loading}
      <span class="text-xs text-muted">載入中...</span>
    {/if}
  </div>

  <div bind:this={container} class="w-full">
    {#if error}
      <div class="flex items-center justify-center h-96 text-muted text-sm">{error}</div>
    {/if}
  </div>
</div>
