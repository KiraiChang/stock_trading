<script lang="ts">
  import type { BrokerTrade } from '../../lib/api/chips'

  export let title: string
  export let rows: BrokerTrade[] = []

  type SortKey = 'broker_name' | 'buy_volume' | 'sell_volume' | 'net_buy'
  let sortKey: SortKey = 'net_buy'
  let sortDesc = true

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDesc = !sortDesc
    } else {
      sortKey = key
      sortDesc = true
    }
  }

  $: sortedRows = [...rows].sort((a, b) => {
    const dir = sortDesc ? -1 : 1
    if (sortKey === 'broker_name') return a.broker_name.localeCompare(b.broker_name) * dir
    return (a[sortKey] - b[sortKey]) * dir
  })

  // 內部單位為股，顯示換算成張（見設計文件單位規則）
  function fmtLots(v: number): string {
    return (v / 1000).toLocaleString('zh-TW', { maximumFractionDigits: 0 })
  }

  function sortIndicator(key: SortKey): string {
    if (sortKey !== key) return ''
    return sortDesc ? ' ▾' : ' ▴'
  }
</script>

<div class="bg-panel border border-border rounded-xl overflow-hidden">
  <div class="px-4 py-2.5 border-b border-border">
    <h3 class="text-sm font-semibold text-white">{title}</h3>
  </div>
  <table class="w-full text-xs">
    <thead>
      <tr class="text-muted border-b border-border">
        <th class="text-left px-4 py-2 cursor-pointer select-none hover:text-white" on:click={() => toggleSort('broker_name')}>
          分點{sortIndicator('broker_name')}
        </th>
        <th class="text-right px-3 py-2 cursor-pointer select-none hover:text-white" on:click={() => toggleSort('buy_volume')}>
          買量（張）{sortIndicator('buy_volume')}
        </th>
        <th class="text-right px-3 py-2 cursor-pointer select-none hover:text-white" on:click={() => toggleSort('sell_volume')}>
          賣量（張）{sortIndicator('sell_volume')}
        </th>
        <th class="text-right px-4 py-2 cursor-pointer select-none hover:text-white" on:click={() => toggleSort('net_buy')}>
          買賣超（張）{sortIndicator('net_buy')}
        </th>
      </tr>
    </thead>
    <tbody>
      {#if sortedRows.length === 0}
        <tr>
          <td colspan="4" class="px-4 py-6 text-center text-muted">
            目前資料來源不支援或尚無券商分點資料
          </td>
        </tr>
      {:else}
        {#each sortedRows as r (r.broker_name + r.branch_name)}
          <tr class="border-b border-border/50">
            <td class="px-4 py-1.5 text-white">{r.broker_name}{r.branch_name ? ` ${r.branch_name}` : ''}</td>
            <td class="px-3 py-1.5 text-right font-mono text-muted">{fmtLots(r.buy_volume)}</td>
            <td class="px-3 py-1.5 text-right font-mono text-muted">{fmtLots(r.sell_volume)}</td>
            <td class="px-4 py-1.5 text-right font-mono {r.net_buy > 0 ? 'text-rise' : r.net_buy < 0 ? 'text-fall' : 'text-muted'}">
              {r.net_buy > 0 ? '+' : ''}{fmtLots(r.net_buy)}
            </td>
          </tr>
        {/each}
      {/if}
    </tbody>
  </table>
</div>
