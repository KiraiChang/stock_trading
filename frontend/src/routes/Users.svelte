<script lang="ts">
  import { onMount } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchUsers, updateUserStatus, type UserItem } from '../lib/api/users'

  let users: UserItem[] = []
  let loading = true
  let error = ''
  let updating: number | null = null

  onMount(async () => {
    await load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      users = await fetchUsers()
    } catch {
      error = '載入使用者清單失敗'
    } finally {
      loading = false
    }
  }

  async function toggleStatus(user: UserItem) {
    const next = user.status === 'active' ? 'inactive' : 'active'
    updating = user.id
    try {
      await updateUserStatus(user.id, next)
      users = users.map((u) => u.id === user.id ? { ...u, status: next } : u)
    } catch {
      error = `更新使用者 ${user.email} 狀態失敗`
    } finally {
      updating = null
    }
  }
</script>

<Layout>
  <div class="max-w-3xl mx-auto">
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-white font-semibold">使用者管理</h1>
      <button
        class="text-xs text-muted hover:text-white px-3 py-1.5 border border-border rounded-lg transition-colors"
        on:click={load}
      >
        重新整理
      </button>
    </div>

    {#if error}
      <p class="text-rise text-sm mb-4">{error}</p>
    {/if}

    <div class="bg-panel border border-border rounded-xl overflow-hidden">
      <table class="w-full text-sm">
        <thead>
          <tr class="text-muted text-xs border-b border-border">
            <th class="text-left px-5 py-3">ID</th>
            <th class="text-left px-4 py-3">Email</th>
            <th class="text-left px-4 py-3">註冊時間</th>
            <th class="text-center px-4 py-3">狀態</th>
            <th class="text-center px-4 py-3">操作</th>
          </tr>
        </thead>
        <tbody>
          {#if loading}
            <tr>
              <td colspan="5" class="px-5 py-8 text-center text-muted">載入中...</td>
            </tr>
          {:else if users.length === 0}
            <tr>
              <td colspan="5" class="px-5 py-8 text-center text-muted">尚無使用者</td>
            </tr>
          {:else}
            {#each users as user (user.id)}
              <tr class="border-b border-border/50 hover:bg-border/20 transition-colors">
                <td class="px-5 py-3 text-muted font-mono text-xs">{user.id}</td>
                <td class="px-4 py-3 text-white">{user.email}</td>
                <td class="px-4 py-3 text-muted text-xs font-mono">{user.created_at}</td>
                <td class="px-4 py-3 text-center">
                  <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium
                    {user.status === 'active'
                      ? 'bg-green-900/40 text-green-400'
                      : 'bg-gray-700/60 text-gray-400'}">
                    <span class="w-1.5 h-1.5 rounded-full
                      {user.status === 'active' ? 'bg-green-400' : 'bg-gray-500'}">
                    </span>
                    {user.status === 'active' ? '啟用' : '停用'}
                  </span>
                </td>
                <td class="px-4 py-3 text-center">
                  <button
                    class="text-xs px-3 py-1 rounded-lg border transition-colors disabled:opacity-40
                      {user.status === 'active'
                        ? 'border-fall/40 text-fall hover:bg-fall/10'
                        : 'border-green-600/40 text-green-400 hover:bg-green-900/20'}"
                    disabled={updating === user.id}
                    on:click={() => toggleStatus(user)}
                  >
                    {updating === user.id ? '...' : user.status === 'active' ? '停用' : '啟用'}
                  </button>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <p class="text-muted text-xs mt-3">
      新註冊的帳號預設為「停用」，需在此頁面手動啟用後才能登入。
    </p>
  </div>
</Layout>
