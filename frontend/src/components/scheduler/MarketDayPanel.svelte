<script lang="ts">
  import { onMount } from 'svelte'
  import {
    clearMarketDayOverride,
    fetchMarketDayOverrides,
    setMarketDayOverride,
    type MarketDayOverridesResponse,
  } from '../../lib/api/marketDays'
  import { ApiError } from '../../lib/api/client'
  import type { MarketDayOverride } from '../../lib/api/intraday'

  // 臨時休市（颱風假）與推翻日曆的手動設定（I-139）。三種狀態：依日曆、手動休市、手動照常開市。
  // 儲存與取消都要填原因；下一輪排程與畫面的下一次輪詢就生效。

  let data: MarketDayOverridesResponse | null = null
  let loadError = ''
  let date = ''
  let action: 'closed' | 'open' = 'closed'
  let reason = ''
  // 全日休市的確認**綁定選定的日期**（v1.2 實作 review）：勾選時記下是哪一天，換日期就清掉，⛔ 不能沿用到另一個日期。
  let confirmed = false
  let confirmedFor = ''
  let saving = false
  let message = ''
  let error = ''

  const errorLabels: Record<string, string> = {
    past_date: '只能設定今天或之後的日期',
    weekend: '週末一律休市，不能設定',
    invalid_date: '日期格式不正確',
    invalid_action: '設定的種類不正確',
    reason_required: '請填寫原因',
    reason_too_long: '原因最多 200 字',
    no_active_override: '這一天沒有生效中的手動設定',
  }

  const actionLabels: Record<MarketDayOverride['action'], string> = {
    closed: '手動休市',
    open: '手動照常開市',
    cleared: '取消手動設定',
  }

  $: if (confirmed && confirmedFor !== date) {
    confirmed = false
    confirmedFor = ''
  }

  function onConfirmChange(e: Event) {
    confirmed = (e.currentTarget as HTMLInputElement).checked
    confirmedFor = confirmed ? date : ''
  }

  onMount(load)

  async function load() {
    loadError = ''
    try {
      data = await fetchMarketDayOverrides()
      if (!date) date = data.today.date
    } catch {
      loadError = '載入休市設定失敗'
    }
  }

  // 今天的判斷與依據。
  function todayText(d: MarketDayOverridesResponse): string {
    const t = d.today
    switch (t.source) {
      case 'weekend':
        return '週末，休市'
      case 'manual':
        return t.is_trading ? '手動照常開市' : '手動休市'
      case 'fallback':
        return '日曆取不到，照平日執行'
      default:
        return t.is_trading ? '依日曆：開市' : '依日曆：休市'
    }
  }

  function apiErrorText(e: unknown): string {
    if (e instanceof ApiError) return errorLabels[e.message] ?? '操作失敗，請稍後再試'
    return '操作失敗，請稍後再試'
  }

  async function save() {
    error = ''
    message = ''
    if (!reason.trim()) {
      error = errorLabels.reason_required
      return
    }
    if (action === 'closed' && !(confirmed && confirmedFor === date)) {
      error = '請先確認交易所公告的是全日休市'
      return
    }
    saving = true
    try {
      const res = await setMarketDayOverride(date, action, reason)
      message = `已設定 ${res.override.trade_date} ${actionLabels[res.override.action]}，下一輪排程生效`
      reason = ''
      confirmed = false
      confirmedFor = ''
      await load()
    } catch (e) {
      error = apiErrorText(e)
    } finally {
      saving = false
    }
  }

  async function clear() {
    error = ''
    message = ''
    if (!reason.trim()) {
      error = errorLabels.reason_required
      return
    }
    saving = true
    try {
      const res = await clearMarketDayOverride(date, reason)
      message = `已取消 ${res.override.trade_date} 的手動設定，回到依日曆判斷`
      reason = ''
      await load()
    } catch (e) {
      error = apiErrorText(e)
    } finally {
      saving = false
    }
  }

  function formatTime(ts: string): string {
    return new Date(ts).toLocaleString('zh-TW', { hour12: false, timeZone: 'Asia/Taipei' })
  }
</script>

<section class="bg-panel border border-border rounded-xl px-5 py-4 mb-4" data-testid="market-day-panel">
  <div class="flex items-center justify-between mb-2">
    <h2 class="text-white font-medium text-sm">交易日與臨時休市</h2>
    {#if data}
      <span class="text-xs text-gray-300" data-testid="market-day-today">今天（{data.today.date}）：{todayText(data)}</span>
    {/if}
  </div>
  {#if loadError}
    <p class="text-rise text-xs">{loadError}</p>
  {/if}

  <p class="text-[11px] text-amber-400 mb-3" data-testid="market-day-warning">
    依交易所公告設定。停班範圍（全日、上午、下午）對交易的處理不同，以公告為準；本設定代表<strong>全日休市</strong>，只有部分時段停班而照常交易時不要設定。
  </p>

  <div class="flex flex-wrap items-end gap-3 text-xs">
    <label class="flex flex-col gap-1 text-muted">
      日期
      <input type="date" class="bg-surface border border-border rounded px-2 py-1 text-white" bind:value={date} data-testid="market-day-date" />
    </label>
    <label class="flex items-center gap-1 text-gray-300">
      <input type="radio" bind:group={action} value="closed" data-testid="market-day-action-closed" /> 手動休市
    </label>
    <label class="flex items-center gap-1 text-gray-300">
      <input type="radio" bind:group={action} value="open" data-testid="market-day-action-open" /> 手動照常開市
    </label>
    <label class="flex flex-col gap-1 text-muted grow min-w-[12rem]">
      原因（必填，例如「證交所公告 颱風全日休市」）
      <input type="text" maxlength="200" class="bg-surface border border-border rounded px-2 py-1 text-white" bind:value={reason} data-testid="market-day-reason" />
    </label>
  </div>
  {#if action === 'closed'}
    <label class="flex items-center gap-1 text-xs text-gray-300 mt-2">
      <input type="checkbox" checked={confirmed} on:change={onConfirmChange} data-testid="market-day-confirm" /> 我已確認交易所公告 {date} 全日休市
    </label>
  {/if}
  <div class="flex gap-2 mt-3">
    <button
      class="text-xs px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg disabled:opacity-50"
      disabled={saving}
      on:click={save}
      data-testid="market-day-save"
    >儲存</button>
    <button
      class="text-xs px-3 py-1.5 border border-border text-muted hover:text-white rounded-lg disabled:opacity-50"
      disabled={saving}
      on:click={clear}
      data-testid="market-day-clear"
    >取消這一天的手動設定</button>
  </div>
  {#if message}
    <p class="text-green-400 text-xs mt-2" data-testid="market-day-message">{message}</p>
  {/if}
  {#if error}
    <p class="text-rise text-xs mt-2" data-testid="market-day-error">{error}</p>
  {/if}

  {#if data}
    {#if data.current.length > 0}
      <h3 class="text-muted text-xs mt-4 mb-1">生效中的手動設定</h3>
      <ul class="text-xs text-gray-300 space-y-0.5" data-testid="market-day-current">
        {#each data.current as o (o.id)}
          <li>{o.trade_date}：{actionLabels[o.action]}（{o.reason}）</li>
        {/each}
      </ul>
    {/if}
    {#if data.history.length > 0}
      <h3 class="text-muted text-xs mt-4 mb-1">操作紀錄</h3>
      <ul class="text-[11px] text-muted space-y-0.5" data-testid="market-day-history">
        {#each data.history as o (o.id)}
          <li>{o.trade_date}｜{actionLabels[o.action]}｜{o.reason}｜使用者 #{o.user_id}｜{formatTime(o.created_at)}</li>
        {/each}
      </ul>
    {/if}
  {/if}
</section>
