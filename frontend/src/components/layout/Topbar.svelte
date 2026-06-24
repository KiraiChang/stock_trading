<script lang="ts">
  import { unreadCount } from '../../lib/stores/signals'
  import { authEmail, authLogout } from '../../lib/stores/auth'
</script>

<header class="flex items-center justify-between px-6 py-3 bg-panel border-b border-border shrink-0">
  <div class="flex items-center gap-3">
    <span class="text-lg font-bold text-white">台股技術分析</span>
    <span class="text-xs text-muted">Trading Assistant</span>
  </div>

  <div class="flex items-center gap-4">
    {#if $unreadCount > 0}
      <div class="flex items-center gap-1.5 bg-rise/20 text-rise px-3 py-1 rounded-full text-sm font-medium">
        <span class="w-2 h-2 rounded-full bg-rise animate-pulse"></span>
        {$unreadCount} 訊號
      </div>
    {/if}

    <div class="w-2 h-2 rounded-full bg-green-400" title="系統運作中"></div>

    <!-- 帳號 + 登出 -->
    <div class="flex items-center gap-2 pl-2 border-l border-border">
      <span class="text-xs text-muted max-w-32 truncate">{$authEmail}</span>
      <button
        class="text-xs text-muted hover:text-white px-2 py-1 rounded hover:bg-border transition-colors"
        on:click={authLogout}
        title="登出"
      >
        登出
      </button>
    </div>
  </div>
</header>
