<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import WatchlistTable from '../components/market/WatchlistTable.svelte'
  import SignalPanel from '../components/signal/SignalPanel.svelte'
  import KLineChart from '../components/chart/KLineChart.svelte'
  import SetupWizard from '../components/setup/SetupWizard.svelte'
  import { watchlist, selectedSymbol } from '../lib/stores/market'
  import { signals } from '../lib/stores/signals'
  import { fetchWatchlist } from '../lib/api/watchlist'
  import { fetchSignals } from '../lib/api/signals'
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

  async function onSetupDone() {
    showSetup = false
    // 重新載入 watchlist（精靈已新增股票）
    const wl = await fetchWatchlist().catch(() => [])
    watchlist.set(wl)
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
