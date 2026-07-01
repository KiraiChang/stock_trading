<script lang="ts">
  import { currentRoute, type Route } from '../../lib/stores/router'

  const navItems: { label: string; icon: string; route: Route }[] = [
    { label: 'Dashboard', icon: '◈', route: 'dashboard' },
    { label: '歷史資料回補', icon: '⟳', route: 'backfill' },
    { label: '策略回測', icon: '▶', route: 'backtest' },
    { label: '排程監控', icon: '⏱', route: 'scheduler' },
    { label: '使用者管理', icon: '⊙', route: 'users' },
  ]
</script>

<aside class="w-16 bg-panel border-r border-border flex flex-col items-center py-4 gap-2 shrink-0">
  {#each navItems as item}
    <button
      class="w-10 h-10 rounded-lg flex items-center justify-center text-lg transition-colors
             {$currentRoute === item.route ? 'bg-indigo-600 text-white' : 'text-muted hover:text-white hover:bg-border'}"
      title={item.label}
      on:click={() => currentRoute.set(item.route)}
    >
      {item.icon}
    </button>
  {/each}
</aside>
