<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { createChart, type IChartApi, type ISeriesApi, ColorType } from 'lightweight-charts'
  import type { ChipScore } from '../../lib/api/chips'

  // 同 InstitutionalTrend.svelte 的說明：呈現 margin_score（融資融券解讀後
  // 正規化的分數，-100~100），不是原始餘額股數。
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
        value: s.margin_score,
        color: s.margin_score >= 0 ? '#e74c3c' : '#2ecc71',
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
    <h3 class="text-sm font-semibold text-white">融資融券變化趨勢（分數）</h3>
  </div>
  <div bind:this={container} class="w-full">
    {#if scores.length === 0}
      <div class="flex items-center justify-center h-[200px] text-muted text-xs">尚無資料</div>
    {/if}
  </div>
</div>
