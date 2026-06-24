<script lang="ts">
  import { login, register } from '../lib/api/auth'
  import { authLogin } from '../lib/stores/auth'

  let mode: 'login' | 'register' = 'login'
  let email = ''
  let password = ''
  let error = ''
  let loading = false

  async function handleSubmit() {
    error = ''
    loading = true
    try {
      if (mode === 'register') {
        await register(email, password)
      }
      const res = await login(email, password)
      authLogin(res.token, email)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (mode === 'login') {
        if (msg.includes('403') || msg.includes('Forbidden')) {
          error = '帳號尚未啟用，請聯絡管理員開通'
        } else {
          error = '帳號或密碼錯誤'
        }
      } else if (msg.includes('409') || msg.includes('Conflict')) {
        error = '此 Email 已被註冊，請直接登入'
      } else {
        error = '註冊失敗，請確認 Email 格式與密碼長度（至少 6 碼）'
      }
    } finally {
      loading = false
    }
  }
</script>

<div class="min-h-screen bg-surface flex items-center justify-center p-4">
  <div class="w-full max-w-sm">
    <!-- Logo / Title -->
    <div class="text-center mb-8">
      <h1 class="text-2xl font-bold text-white">台股技術分析</h1>
      <p class="text-muted text-sm mt-1">Trading Assistant</p>
    </div>

    <!-- Card -->
    <div class="bg-panel border border-border rounded-xl p-6">
      <!-- Tab switch -->
      <div class="flex mb-6 bg-surface rounded-lg p-1">
        <button
          class="flex-1 py-1.5 text-sm rounded-md transition-colors
                 {mode === 'login' ? 'bg-indigo-600 text-white font-medium' : 'text-muted hover:text-white'}"
          on:click={() => { mode = 'login'; error = '' }}
        >
          登入
        </button>
        <button
          class="flex-1 py-1.5 text-sm rounded-md transition-colors
                 {mode === 'register' ? 'bg-indigo-600 text-white font-medium' : 'text-muted hover:text-white'}"
          on:click={() => { mode = 'register'; error = '' }}
        >
          註冊
        </button>
      </div>

      <form on:submit|preventDefault={handleSubmit} class="flex flex-col gap-4">
        <div>
          <label class="block text-xs text-muted mb-1" for="email">Email</label>
          <input
            id="email"
            type="email"
            bind:value={email}
            placeholder="admin@trading.com"
            required
            class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        <div>
          <label class="block text-xs text-muted mb-1" for="password">密碼</label>
          <input
            id="password"
            type="password"
            bind:value={password}
            placeholder={mode === 'register' ? '至少 6 個字元' : ''}
            required
            class="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        {#if error}
          <p class="text-rise text-xs">{error}</p>
        {/if}

        <button
          type="submit"
          disabled={loading}
          class="w-full bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed
                 text-white font-medium text-sm py-2 rounded-lg transition-colors mt-1"
        >
          {loading ? '處理中...' : mode === 'login' ? '登入' : '註冊並登入'}
        </button>
      </form>
    </div>
  </div>
</div>
