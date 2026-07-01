<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { createChart, type IChartApi, type ISeriesApi, type LineData, type Time, ColorType } from 'lightweight-charts'
  import { fetchCandles, type Candle } from '../../lib/api/candles'

  export let symbol: string

  let container: HTMLDivElement
  let chart: IChartApi | null = null
  let candleSeries: ISeriesApi<'Candlestick'> | null = null
  let ma5Series: ISeriesApi<'Line'> | null = null
  let ma20Series: ISeriesApi<'Line'> | null = null
  let ma60Series: ISeriesApi<'Line'> | null = null
  let loading = false
  let error = ''

  // ── MA 顯示/隱藏開關 ─────────────────────────────────────────
  let showMA5 = true
  let showMA20 = true
  let showMA60 = false

  $: ma5Series?.applyOptions({ visible: showMA5 })
  $: ma20Series?.applyOptions({ visible: showMA20 })
  $: ma60Series?.applyOptions({ visible: showMA60 })

  // calcMA：跟後端 CalcMA 一致，最後 N 根收盤價的算術平均；資料不足 N 根的
  // 位置回傳 null（不畫線），避免用不足 N 根的資料算出誤導性的平均值
  function calcMA(closes: number[], period: number): (number | null)[] {
    const result: (number | null)[] = new Array(closes.length).fill(null)
    let sum = 0
    for (let i = 0; i < closes.length; i++) {
      sum += closes[i]
      if (i >= period) sum -= closes[i - period]
      if (i >= period - 1) result[i] = sum / period
    }
    return result
  }

  function toLineData(times: Time[], values: (number | null)[]): LineData[] {
    const points: LineData[] = []
    for (let i = 0; i < times.length; i++) {
      const v = values[i]
      if (v !== null) points.push({ time: times[i], value: v })
    }
    return points
  }

  async function loadData(sym: string) {
    if (!chart || !sym) return
    loading = true
    error = ''
    try {
      const candles = await fetchCandles(sym, '1d', 120)
      const times = candles.map((c: Candle) => c.ts.substring(0, 10) as `${number}-${number}-${number}`)
      const chartData = candles.map((c: Candle, i) => ({
        time: times[i],
        open: c.open,
        high: c.high,
        low: c.low,
        close: c.close,
      }))
      candleSeries?.setData(chartData)

      const closes = candles.map((c: Candle) => c.close)
      ma5Series?.setData(toLineData(times, calcMA(closes, 5)))
      ma20Series?.setData(toLineData(times, calcMA(closes, 20)))
      ma60Series?.setData(toLineData(times, calcMA(closes, 60)))

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

    ma5Series = chart.addLineSeries({
      color: '#f59e0b', lineWidth: 1, visible: showMA5,
      priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false,
    })
    ma20Series = chart.addLineSeries({
      color: '#38bdf8', lineWidth: 1, visible: showMA20,
      priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false,
    })
    ma60Series = chart.addLineSeries({
      color: '#a78bfa', lineWidth: 1, visible: showMA60,
      priceLineVisible: false, lastValueVisible: false, crosshairMarkerVisible: false,
    })

    const ro = new ResizeObserver(() => {
      chart?.applyOptions({ width: container.clientWidth })
    })
    ro.observe(container)

    return () => {
      ro.disconnect()
    }
  })

  // 只依賴 chart / symbol，chart 就緒或 symbol 真正變更時才重新載入。
  // 先前用 afterUpdate 會在「任何」元件更新後觸發，而 loadData 本身會
  // 設定 loading/error 觸發更新，造成無限迴圈重複打 /candles API。
  $: if (chart && symbol) {
    loadData(symbol)
  }

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
    <div class="flex items-center gap-3">
      <label class="flex items-center gap-1 text-xs cursor-pointer select-none" style="color: #f59e0b">
        <input type="checkbox" bind:checked={showMA5} class="accent-current w-3.5 h-3.5" />
        MA5
      </label>
      <label class="flex items-center gap-1 text-xs cursor-pointer select-none" style="color: #38bdf8">
        <input type="checkbox" bind:checked={showMA20} class="accent-current w-3.5 h-3.5" />
        MA20
      </label>
      <label class="flex items-center gap-1 text-xs cursor-pointer select-none" style="color: #a78bfa">
        <input type="checkbox" bind:checked={showMA60} class="accent-current w-3.5 h-3.5" />
        MA60
      </label>
      {#if loading}
        <span class="text-xs text-muted">載入中...</span>
      {/if}
    </div>
  </div>

  <div bind:this={container} class="w-full">
    {#if error}
      <div class="flex items-center justify-center h-96 text-muted text-sm">{error}</div>
    {/if}
  </div>
</div>
