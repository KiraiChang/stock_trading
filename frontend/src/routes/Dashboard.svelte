<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import WatchlistTable from '../components/market/WatchlistTable.svelte'
  import SignalPanel from '../components/signal/SignalPanel.svelte'
  import KLineChart from '../components/chart/KLineChart.svelte'
  import SetupWizard from '../components/setup/SetupWizard.svelte'
  import { watchlist, quotes, selectedSymbol, type WatchlistItem, type StockQuote } from '../lib/stores/market'
  import { signals, type Signal } from '../lib/stores/signals'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import { fetchSignals } from '../lib/api/signals'
  import { fetchCandles } from '../lib/api/candles'
  import { fetchIndicators } from '../lib/api/indicators'
  import { deriveTrendFromMA } from '../lib/utils/trend'
  import { socket } from '../lib/ws/socket'

  let wsUrl = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/market`
  let showSetup = false

  onMount(async () => {
    const [wl, sigs] = await Promise.all([
      fetchWatchlist().catch(() => []),
      fetchSignals(50).catch(() => []),
    ])
    watchlist.set(wl)
    signals.set(sigs)
    hydrateQuotes(wl, sigs)

    // 首次登入且監控清單為空時，顯示初始設定精靈
    if (wl.length === 0) {
      showSetup = true
    }

    socket.connect(wsUrl)

    const symbols = wl.map((w) => w.symbol)
    if (symbols.length > 0) {
      setTimeout(() => socket.subscribe(symbols), 500)
    }
  })

  // 監控清單的收盤/漲跌%/量比/RSI/趨勢/訊號欄位平常靠 WebSocket 推播更新，
  // 但後端只在「產生訊號」時才會推播，一般情況下（沒觸發突破/爆量）欄位
  // 會一直是空的。這裡在載入 watchlist 後用既有的 /candles、/indicators、
  // /signals REST API 主動撈一次最新資料，讓清單一開啟就有內容可看。
  async function hydrateQuotes(items: WatchlistItem[], recentSignals: Signal[]) {
    const entries = await Promise.all(
      items.map(async (item): Promise<[string, StockQuote]> => {
        const [candles, ind] = await Promise.all([
          fetchCandles(item.symbol, '1d', 2).catch(() => []),
          fetchIndicators(item.symbol, '1d'),
        ])

        const latest = candles[candles.length - 1]
        const prev = candles[candles.length - 2]
        const change = latest && prev ? latest.close - prev.close : 0
        const changePct = latest && prev && prev.close ? (change / prev.close) * 100 : 0

        // 優先用該股票最新一筆訊號附帶的趨勢（跟後端 HH/HL 結構判斷一致），
        // 沒有訊號紀錄可查時才退回用 MA5/MA20 排列位置概略推算
        const latestSignal = recentSignals
          .filter((s) => s.symbol === item.symbol)
          .sort((a, b) => (a.ts < b.ts ? 1 : -1))[0]

        const quote: StockQuote = {
          symbol: item.symbol,
          name: item.name,
          close: latest?.close ?? 0,
          open: latest?.open ?? 0,
          high: latest?.high ?? 0,
          low: latest?.low ?? 0,
          change,
          changePct,
          volume: latest?.volume ?? 0,
          volRatio: ind?.vol_ratio ?? 0,
          ma5: ind?.ma5 ?? 0,
          ma20: ind?.ma20 ?? 0,
          rsi14: ind?.rsi14 ?? 0,
          trend: (latestSignal?.trend as StockQuote['trend']) ?? deriveTrendFromMA(ind?.ma5 ?? 0, ind?.ma20 ?? 0),
          hasSignal: recentSignals.some((s) => s.symbol === item.symbol),
        }
        return [item.symbol, quote]
      }),
    )

    quotes.update((map) => {
      const next = new Map(map)
      for (const [symbol, quote] of entries) next.set(symbol, quote)
      return next
    })
  }

  async function onSetupDone() {
    showSetup = false
    // 重新載入 watchlist（精靈已新增股票）
    const wl = await fetchWatchlist().catch(() => [])
    watchlist.set(wl)
    hydrateQuotes(wl, [])
    const symbols = wl.map((w) => w.symbol)
    if (symbols.length > 0) {
      setTimeout(() => socket.subscribe(symbols), 500)
    }
  }
</script>

{#if showSetup}
  <SetupWizard
    on:done={onSetupDone}
    on:skip={() => { showSetup = false }}
  />
{/if}

<Layout>
  <div class="grid grid-cols-12 gap-4 h-full">
    <!-- 左側：監控清單 -->
    <div class="col-span-4 overflow-auto">
      <WatchlistTable on:symbolAdded={(e) => socket.subscribe([e.detail])} />
    </div>

    <!-- 右側：訊號 + K 線圖 -->
    <div class="col-span-8 flex flex-col gap-4 overflow-auto">
      <SignalPanel />
      {#if $selectedSymbol}
        <KLineChart symbol={$selectedSymbol} />
      {:else}
        <div class="flex-1 flex items-center justify-center text-muted text-sm bg-panel rounded-lg border border-border">
          點擊左側股票查看 K 線圖
        </div>
      {/if}
    </div>
  </div>
</Layout>
