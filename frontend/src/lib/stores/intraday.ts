import { writable } from 'svelte/store'
import type { IntradayStatusResponse } from '../api/intraday'

// 盤中逐檔狀態（T-088）。Dashboard 盤中每 60 秒、非盤中每 5 分鐘重抓一次（lib/utils/intradayPolling.ts）。
export const intradayStatus = writable<IntradayStatusResponse | null>(null)
