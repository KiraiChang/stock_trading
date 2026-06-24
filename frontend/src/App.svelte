<script lang="ts">
  import { isAuthenticated } from './lib/stores/auth'
  import { currentRoute } from './lib/stores/router'
  import Login from './routes/Login.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  import Users from './routes/Users.svelte'
</script>

{#if $isAuthenticated}
  {#if $currentRoute === 'users'}
    <Users />
  {:else}
    <Dashboard />
  {/if}
{:else}
  <Login />
{/if}
