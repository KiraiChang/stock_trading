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
  {:else}
    <Dashboard />
  {/if}
{:else}
  <Login />
{/if}
