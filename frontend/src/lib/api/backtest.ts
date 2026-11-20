import { apiFetch } from './client'

export interface BacktestJob {
  id: number
  job_id: string
  type: string
  strategy: string
  symbols: string // JSON array string，例如 '["2330","2454"]'
  timeframe: string
  start_date: string
  end_date: string
  status: 'pending' | 'running' | 'done' | 'failed'
  trigger: string
  error?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface BacktestResult {
  id: number
  job_id: string
  strategy: string
  total_return: number
  annual_return: number
  win_rate: number
  max_drawdown: number
  sharpe_ratio: number
  total_trades: number
  win_trades: number
  loss_trades: number
  avg_pnl: number
  // 每一檔實際取到的資料範圍（I-137）；I-137 之前的結果是 null
  data_coverage?: DataCoverage[] | null
  created_at: string
}

// 覆蓋範圍的一檔（格式見 docs/database-schema.md 的 backtest_results）。
// first_bar 晚於開始日可能只是遇到週末或休市，⛔ 不判定成缺漏；範圍內的缺漏也⛔ 不偵測。
export interface DataCoverage {
  symbol: string
  first_bar: string | null
  last_bar: string | null
  bars: number
  warmup_bars: number
  warmup_required: number
  skipped: boolean
}

export interface BacktestTrade {
  id: number
  job_id: string
  symbol: string
  direction: 'BUY' | 'SELL'
  entry_time?: string
  exit_time?: string
  entry_price: number
  exit_price: number
  size: number
  pnl: number
  pnl_pct: number
  commission: number
  created_at: string
}

export interface SubmitBacktestOptions {
  strategy: string
  symbols: string[]
  timeframe?: string
  start_date: string
  end_date: string
}

// 內建的 strategy 選項：既有 backtrader 版 + 新的模組化版（見
// python/backtest/modular/strategy.py 的 STRATEGY_PRESETS）
export const BACKTEST_STRATEGIES = [
  { value: 'breakout_v1', label: 'breakout_v1（backtrader 版）' },
  { value: 'breakout_swing_atr_v1', label: 'breakout_swing_atr_v1（Swing S/R + ATR停損）' },
  { value: 'breakout_volprofile_composite_v1', label: 'breakout_volprofile_composite_v1（量價分布 + 複合停損）' },
  { value: 'pullback_atrchannel_structural_v1', label: 'pullback_atrchannel_structural_v1（ATR通道 + 結構停損）' },
  { value: 'pullback_swing_composite_v1', label: 'pullback_swing_composite_v1（Swing S/R + 複合停損）' },
] as const

// backtrader 引擎的策略（python 的 STRATEGY_MAP）：一次只能回測一檔（I-137；真正的多檔見 docs/issue.md I-138）。
// ⛔ 改這份名單要同步 Go 的 BacktraderStrategies 與 python 的 STRATEGY_MAP（scripts/check-backtest-contract.py 比對）。
export const BACKTRADER_STRATEGIES = ['breakout_v1'] as const

export function isBacktraderStrategy(strategy: string): boolean {
  return (BACKTRADER_STRATEGIES as readonly string[]).includes(strategy)
}

export async function submitBacktest(opts: SubmitBacktestOptions): Promise<BacktestJob> {
  const res = await apiFetch<{ job: BacktestJob }>('/backtest', {
    method: 'POST',
    body: JSON.stringify({ timeframe: '1d', ...opts }),
  })
  return res.job
}

export async function listBacktestJobs(limit = 20): Promise<BacktestJob[]> {
  const res = await apiFetch<{ jobs: BacktestJob[]; total: number }>(`/backtest?limit=${limit}`)
  return res.jobs ?? []
}

export async function getBacktestJob(jobId: string): Promise<{ job: BacktestJob; result: BacktestResult | null }> {
  return apiFetch(`/backtest/${jobId}`)
}

export async function getBacktestTrades(jobId: string): Promise<BacktestTrade[]> {
  const res = await apiFetch<{ trades: BacktestTrade[]; total: number }>(`/backtest/${jobId}/trades`)
  return res.trades ?? []
}

export async function cancelBacktestJob(jobId: string): Promise<void> {
  await apiFetch(`/backtest/${jobId}`, { method: 'DELETE' })
}
