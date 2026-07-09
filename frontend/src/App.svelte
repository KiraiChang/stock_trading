<script lang="ts">
  import { isAuthenticated } from './lib/stores/auth'
  import { currentRoute } from './lib/stores/router'
  import Login from './routes/Login.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  import Users from './routes/Users.svelte'
  import Scheduler from './routes/Scheduler.svelte'
  import Backfill from './routes/Backfill.svelte'
  import Backtest from './routes/Backtest.svelte'
  import Analysis from './routes/Analysis.svelte'
  import SRZones from './routes/SRZones.svelte'
  import Chips from './routes/Chips.svelte'
  import Positions from './routes/Positions.svelte'
</script>

{#if $isAuthenticated}
  {#if $currentRoute === 'users'}
    <Users />
  {:else if $currentRoute === 'scheduler'}
    <Scheduler />
  {:else if $currentRoute === 'backfill'}
    <Backfill />
  {:else if $currentRoute === 'backtest'}
    <Backtest />
  {:else if $currentRoute === 'analysis'}
    <Analysis />
  {:else if $currentRoute === 'sr-zones'}
    <SRZones />
  {:else if $currentRoute === 'positions'}
    <Positions />
  {:else if $currentRoute === 'chips'}
    <Chips />
  {:else}
    <Dashboard />
  {/if}
{:else}
  <Login />
{/if}
