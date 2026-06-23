<script lang="ts">
  import Topbar from './Topbar.svelte'
  import Sidebar from './Sidebar.svelte'
</script>

<div class="flex flex-col h-screen bg-surface overflow-hidden">
  <Topbar />
  <div class="flex flex-1 overflow-hidden">
    <Sidebar />
    <main class="flex-1 overflow-auto p-4">
      <slot />
    </main>
  </div>
</div>
