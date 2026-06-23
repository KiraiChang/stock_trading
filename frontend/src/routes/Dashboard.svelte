<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import WatchlistTable from '../components/market/WatchlistTable.svelte'
  import SignalPanel from '../components/signal/SignalPanel.svelte'
  import KLineChart from '../components/chart/KLineChart.svelte'
  import { watchlist, selectedSymbol } from '../lib/stores/market'
  import { signals } from '../lib/stores/signals'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import { fetchSignals } from '../lib/api/signals'
  import { socket } from '../lib/ws/socket'

  let wsUrl = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/market`

  onMount(async () => {
    // 載入初始資料
    const [wl, sigs] = await Promise.all([
      fetchWatchlist().catch(() => []),
      fetchSignals(50).catch(() => []),
    ])
    watchlist.set(wl)
    signals.set(sigs)

    // 連接 WebSocket
    socket.connect(wsUrl)

    // 訂閱所有監控股票
    const symbols = wl.map((w) => w.symbol)
    if (symbols.length > 0) {
      setTimeout(() => socket.subscribe(symbols), 500)
    }
  })
</script>

<Layout>
  <div class="grid grid-cols-12 gap-4 h-full">
    <!-- 左側：監控清單 -->
    <div class="col-span-4 overflow-auto">
      <WatchlistTable />
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
