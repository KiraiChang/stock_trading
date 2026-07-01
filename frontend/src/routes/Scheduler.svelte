<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import { fetchSchedulerStatus, type SchedulerJob } from '../lib/api/scheduler'

  const REFRESH_MS = 15000

  const jobLabel: Record<string, string> = {
    pre_market: '盤前初始化',
    intraday: '盤中分K',
    daily_close: '收盤後結算',
  }

  const statusLabel: Record<string, string> = {
    running: '執行中',
    success: '成功',
    partial: '部分失敗',
    failed: '失敗',
    never_run: '尚未執行',
  }

  const statusClass: Record<string, string> = {
    running: 'bg-blue-900/40 text-blue-400',
    success: 'bg-green-900/40 text-green-400',
    partial: 'bg-yellow-900/40 text-yellow-400',
    failed: 'bg-red-900/40 text-red-400',
    never_run: 'bg-gray-700/60 text-gray-400',
  }

  const dotClass: Record<string, string> = {
    running: 'bg-blue-400',
    success: 'bg-green-400',
    partial: 'bg-yellow-400',
    failed: 'bg-red-400',
    never_run: 'bg-gray-500',
  }

  let jobs: SchedulerJob[] = []
  let loading = true
  let error = ''
  let timer: ReturnType<typeof setInterval>

  onMount(async () => {
    await load()
    timer = setInterval(load, REFRESH_MS)
  })

  onDestroy(() => clearInterval(timer))

  async function load() {
    if (jobs.length === 0) loading = true
    error = ''
    try {
      jobs = await fetchSchedulerStatus()
    } catch {
      error = '載入排程狀態失敗'
    } finally {
      loading = false
    }
  }

  function formatTime(ts?: string): string {
    if (!ts) return '—'
    return new Date(ts).toLocaleString('zh-TW', { hour12: false })
  }
</script>

<Layout>
  <div class="max-w-4xl mx-auto">
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-white font-semibold">排程監控</h1>
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

    {#if loading}
      <div class="bg-panel border border-border rounded-xl px-5 py-8 text-center text-muted text-sm">
        載入中...
      </div>
    {:else}
      <div class="grid gap-4">
        {#each jobs as job (job.job_name)}
          <div class="bg-panel border border-border rounded-xl px-5 py-4">
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-2">
                <h2 class="text-white font-medium text-sm">{jobLabel[job.job_name] ?? job.job_name}</h2>
                <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium
                  {statusClass[job.status] ?? 'bg-gray-700/60 text-gray-400'}">
                  <span class="w-1.5 h-1.5 rounded-full {dotClass[job.status] ?? 'bg-gray-500'}"></span>
                  {statusLabel[job.status] ?? job.status}
                </span>
              </div>
              {#if job.stale}
                <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-900/40 text-red-400">
                  ⚠ 已延遲未執行
                </span>
              {/if}
            </div>

            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
              <div>
                <p class="text-muted mb-1">開始時間</p>
                <p class="text-white font-mono">{formatTime(job.started_at)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">結束時間</p>
                <p class="text-white font-mono">{formatTime(job.finished_at)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">股票數</p>
                <p class="text-white">{job.symbols_total}</p>
              </div>
              <div>
                <p class="text-muted mb-1">失敗數</p>
                <p class={job.symbols_failed > 0 ? 'text-fall' : 'text-white'}>{job.symbols_failed}</p>
              </div>
            </div>

            {#if job.error}
              <p class="text-fall text-xs mt-3 font-mono break-all">{job.error}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <p class="text-muted text-xs mt-3">
      每 {REFRESH_MS / 1000} 秒自動刷新一次；「已延遲未執行」代表該排程超過預期間隔沒有新的執行紀錄。
    </p>
  </div>
</Layout>
