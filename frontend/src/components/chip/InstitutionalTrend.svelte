<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { createChart, type IChartApi, type ISeriesApi, ColorType } from 'lightweight-charts'
  import type { ChipScore } from '../../lib/api/chips'

  // scores 為歷史 chip_scores（見 lib/api/chips.ts fetchChipScores），本圖
  // 呈現 institutional_score（法人買賣超正規化後的分數，-100~100），不是
  // 原始買賣超股數——後端目前沒有提供 institutional_trades 的歷史區間查詢
  // API，分數已經反映買賣超方向與強度，足以看出趨勢。
  export let scores: ChipScore[] = []

  let container: HTMLDivElement
  let chart: IChartApi | null = null
  let series: ISeriesApi<'Histogram'> | null = null

  function render() {
    if (!series) return
    const data = [...scores]
      .sort((a, b) => a.trade_date.localeCompare(b.trade_date))
      .map((s) => ({
        time: s.trade_date.substring(0, 10) as `${number}-${number}-${number}`,
        value: s.institutional_score,
        // 台股慣例：買超（正分）用紅色，賣超（負分）用綠色
        color: s.institutional_score >= 0 ? '#e74c3c' : '#2ecc71',
      }))
    series.setData(data)
    chart?.timeScale().fitContent()
  }

  onMount(() => {
    chart = createChart(container, {
      layout: { background: { type: ColorType.Solid, color: '#1a1a2e' }, textColor: '#9ca3af' },
      grid: { vertLines: { color: '#2a2a4a' }, horzLines: { color: '#2a2a4a' } },
      width: container.clientWidth,
      height: 200,
    })
    series = chart.addHistogramSeries({ priceLineVisible: false, lastValueVisible: false })
    render()

    const ro = new ResizeObserver(() => chart?.applyOptions({ width: container.clientWidth }))
    ro.observe(container)
    return () => ro.disconnect()
  })

  $: if (series) render()

  onDestroy(() => {
    chart?.remove()
    chart = null
  })
</script>

<div class="bg-panel rounded-lg border border-border overflow-hidden">
  <div class="px-4 py-2.5 border-b border-border">
    <h3 class="text-sm font-semibold text-white">三大法人買賣超趨勢（分數）</h3>
  </div>
  <div bind:this={container} class="w-full">
    {#if scores.length === 0}
      <div class="flex items-center justify-center h-[200px] text-muted text-xs">尚無資料</div>
    {/if}
  </div>
</div>
