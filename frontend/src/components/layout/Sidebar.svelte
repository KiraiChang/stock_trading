<script lang="ts">
  import { selectedSymbol } from '../../lib/stores/market'

  const navItems = [
    { label: 'Dashboard', icon: '◈', route: 'dashboard' },
    { label: '監控清單', icon: '≡', route: 'watchlist' },
    { label: '訊號', icon: '◉', route: 'signals' },
  ]

  let currentRoute = 'dashboard'
</script>

<aside class="w-16 bg-panel border-r border-border flex flex-col items-center py-4 gap-2 shrink-0">
  {#each navItems as item}
    <button
      class="w-10 h-10 rounded-lg flex items-center justify-center text-lg transition-colors
             {currentRoute === item.route ? 'bg-indigo-600 text-white' : 'text-muted hover:text-white hover:bg-border'}"
      title={item.label}
      on:click={() => { currentRoute = item.route }}
    >
      {item.icon}
    </button>
  {/each}
</aside>
