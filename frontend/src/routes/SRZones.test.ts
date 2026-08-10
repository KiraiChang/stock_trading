import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/svelte'
import { tick } from 'svelte'
import SRZones from './SRZones.svelte'
import {
  getModelStatus,
  getSREvaluationJob,
  listSREvaluationJobs,
  listSRRegressionResults,
  listSRZoneAnalyses,
  listTrainJobs,
  runSREvaluation,
  type SREvaluationJob,
} from '../lib/api/srZones'

// 元件層測試：srZones.ts 的 API 契約已由 srZones.test.ts 保護，這裡驗的是
// SRZones 頁面 evaluation 區塊的行為——送出前的前置驗證、payload 推導，以及
// job 輪詢的終止條件。只 mock API function，`derivedReasonLabel` 等純函式沿用真實實作。
vi.mock('../lib/api/srZones', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api/srZones')>()
  return {
    ...actual,
    createSRZoneAnalysis: vi.fn(),
    listSRZoneAnalyses: vi.fn(),
    getSRZoneAnalysis: vi.fn(),
    deleteSRZoneAnalysis: vi.fn(),
    verifySRZoneAnalysis: vi.fn(),
    triggerSRScoringTrain: vi.fn(),
    getTrainJob: vi.fn(),
    listTrainJobs: vi.fn(),
    pruneTrainJobs: vi.fn(),
    getModelStatus: vi.fn(),
    runSREvaluation: vi.fn(),
    getSREvaluationJob: vi.fn(),
    listSREvaluationJobs: vi.fn(),
    listSRRegressionResults: vi.fn(),
  }
})

const SYMBOLS_PLACEHOLDER = '股票代號，逗號分隔（留空 = 上方股票代號）'
const LIMIT_TITLE = 'evaluation 用的歷史K棒根數（每檔股票）'
const REPLAY_ROWS_TITLE = 'Decision Replay 輸出 rows 上限'
const POLL_INTERVAL_MS = 3000

function doneJob(overrides: Partial<SREvaluationJob> = {}): SREvaluationJob {
  return {
    id: 1,
    job_id: 'sr_eval_job_001',
    status: 'done',
    symbols: '["2330"]',
    timeframe: '1d',
    fetch_limit: 1500,
    mode: 'decision_replay',
    write_db: true,
    replay_max_rows: 100,
    run_id: 'sr_replay_001',
    schema_version: 'sr_zone_decision_replay_p0',
    pipeline_version: 'sr_zone_decision_replay_p0',
    rows: 12,
    sources: 3,
    report: { run_id: 'sr_replay_001', rows: 12 },
    error: null,
    started_at: null,
    finished_at: null,
    created_at: '2026-08-04T02:00:00Z',
    ...overrides,
  }
}

beforeEach(() => {
  vi.mocked(listSRZoneAnalyses).mockResolvedValue([])
  vi.mocked(listTrainJobs).mockResolvedValue([])
  vi.mocked(listSREvaluationJobs).mockResolvedValue([])
  vi.mocked(listSRRegressionResults).mockResolvedValue([])
  // Python service 未啟動的既有分支：元件 catch 後把 modelStatus 設成 null。
  vi.mocked(getModelStatus).mockRejectedValue(new Error('python service down'))
  vi.mocked(runSREvaluation).mockReset()
  vi.mocked(getSREvaluationJob).mockReset()
})

afterEach(() => {
  vi.useRealTimers()
})

// onMount 的 loader 都是 mock，microtask 內就結算；evaluation 區塊本身不等 loading。
async function renderSRZones() {
  render(SRZones)
  await tick()
  await tick()
  return screen.getByRole('button', { name: '開始驗證' })
}

describe('SRZones 頁面 evaluation 送出前驗證', () => {
  it('沒有股票代號時擋下並顯示錯誤，不呼叫 API', async () => {
    const button = await renderSRZones()

    await fireEvent.click(button)

    expect(await screen.findByText('請輸入 evaluation 股票代號')).toBeInTheDocument()
    expect(runSREvaluation).not.toHaveBeenCalled()
  })

  it('抓取根數低於 80 時擋下並顯示錯誤', async () => {
    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.input(screen.getByTitle(LIMIT_TITLE), { target: { value: '50' } })

    await fireEvent.click(button)

    expect(await screen.findByText('evaluation 抓取根數至少要 80 根')).toBeInTheDocument()
    expect(runSREvaluation).not.toHaveBeenCalled()
  })

  it('decision replay 模式下 replay rows 非正數時擋下並顯示錯誤', async () => {
    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.input(screen.getByTitle(REPLAY_ROWS_TITLE), { target: { value: '0' } })

    await fireEvent.click(button)

    expect(await screen.findByText('replay rows 必須大於 0')).toBeInTheDocument()
    expect(runSREvaluation).not.toHaveBeenCalled()
  })
})

describe('SRZones 頁面 evaluation 送出與輪詢', () => {
  it('送出時帶入由模式推導的 payload 並顯示後端訊息', async () => {
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_001',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 2,
    })
    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), {
      target: { value: '2330, 2454' },
    })

    await fireEvent.click(button)

    expect(runSREvaluation).toHaveBeenCalledWith({
      symbols: ['2330', '2454'],
      timeframe: '1d',
      limit: 1500,
      decisionReplay: true,
      replayMaxRows: 100,
      // 預設不寫入：寫 DB 會啟動該模型的 production entry gate，不該是點兩下的副作用。
      writeDb: false,
    })
    expect(await screen.findByText('已在背景開始 evaluation')).toBeInTheDocument()
  })

  it('輪詢到 done 後停止輪詢、顯示 run_id 並重新載入 regression 結果', async () => {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_001',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(doneJob())

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)

    const regressionCallsBefore = vi.mocked(listSRRegressionResults).mock.calls.length

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()

    expect(getSREvaluationJob).toHaveBeenCalledTimes(1)
    expect(screen.getByText('已完成 sr_replay_001')).toBeInTheDocument()
    // write_db=true 的 job 完成後要重抓 regression 結果，UI 才會反映新寫入的那筆。
    expect(vi.mocked(listSRRegressionResults).mock.calls.length).toBe(regressionCallsBefore + 1)

    // 已進入終態就不該再輪詢。
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 2)
    expect(getSREvaluationJob).toHaveBeenCalledTimes(1)
  })

  it('輪詢到 failed 時顯示 job 的錯誤並停止輪詢', async () => {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_002',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(
      doneJob({ status: 'failed', error: 'python upstream unavailable', run_id: null, report: null })
    )

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()

    // 錯誤訊息列與 active job 面板都會帶上 job.error，所以是多個節點。
    expect(screen.getAllByText('python upstream unavailable').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: '開始驗證' })).toBeEnabled()

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 2)
    expect(getSREvaluationJob).toHaveBeenCalledTimes(1)
  })
})

describe('SRZones 頁面 evaluation 的寫入開關與治理判定', () => {
  it('勾選寫入結果後才送 writeDb=true，並顯示會啟動 gate 的警語', async () => {
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_003',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    const button = await renderSRZones()
    expect(screen.queryByText(/會寫入 stock_sr_regression_results/)).not.toBeInTheDocument()

    await fireEvent.click(screen.getByLabelText('寫入結果'))

    expect(await screen.findByText(/會寫入 stock_sr_regression_results/)).toBeInTheDocument()

    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)

    expect(runSREvaluation).toHaveBeenCalledWith(expect.objectContaining({ writeDb: true }))
  })

  it('job 完成後顯示治理判定與覆蓋率，不必寫 DB 就看得到', async () => {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_004',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 2,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(
      doneJob({
        write_db: false,
        report: {
          run_id: 'sr_replay_001',
          governance_evaluation: {
            health_state: 'DEGRADED',
            blocking_flags: [],
            warning_flags: ['REPLAY_SYMBOL_COVERAGE_PARTIAL'],
            confidence_gate: { allow_entry: true, max_entry_state: 'SMALL_ENTRY' },
          },
          replay_coverage: {
            symbols_requested: 4,
            symbols_covered: 2,
            symbols_skipped: ['00947', '00981A'],
            coverage_ratio: 0.5,
          },
        },
      })
    )

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()

    expect(screen.getByText('模型治理')).toBeInTheDocument()
    expect(screen.getByText('DEGRADED')).toBeInTheDocument()
    expect(screen.getByText('allow_entry=true')).toBeInTheDocument()
    expect(screen.getByText('max_entry_state=SMALL_ENTRY')).toBeInTheDocument()
    expect(screen.getByText('REPLAY_SYMBOL_COVERAGE_PARTIAL')).toBeInTheDocument()
    expect(screen.getByText(/覆蓋 2\/4/)).toBeInTheDocument()
    expect(screen.getByText(/略過 00947, 00981A/)).toBeInTheDocument()
  })

  it('Zone Evaluation 模式（report 沒有治理判定）不顯示治理區塊', async () => {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_005',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(
      doneJob({ report: { run_id: 'sr_eval_001', rows: 12 } })
    )

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()

    expect(screen.queryByText('模型治理')).not.toBeInTheDocument()
  })
})

describe('SRZones 治理判定的顏色語意', () => {
  it('allow_entry=false 要用紅色（text-rise）標示被擋單', async () => {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_006',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(
      doneJob({
        report: {
          run_id: 'sr_replay_002',
          governance_evaluation: {
            health_state: 'UNRELIABLE',
            blocking_flags: ['REPLAY_SAMPLE_TOO_SMALL'],
            warning_flags: [],
            confidence_gate: { allow_entry: false, max_entry_state: 'WAIT_CONFIRMATION' },
          },
        },
      })
    )

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()

    // tailwind.config.js：rise=#e74c3c(紅)、fall=#2ecc71(綠)。被擋單是壞消息，必須是紅色。
    const blocked = screen.getByText('allow_entry=false')
    expect(blocked).toHaveClass('text-rise')
    expect(screen.getByText('max_entry_state=WAIT_CONFIRMATION')).toBeInTheDocument()
  })
})

// 兩種 report schema 的欄位幾乎互斥（evaluation 有 model_metrics / zone_outcomes，
// decision replay 有 outcome_summary / governance_evaluation），面板一律 by-presence 渲染。
// 這組測試鎖的就是「哪個 report 出現哪些區塊」。
describe('SRZones evaluation report 的核心指標區塊', () => {
  async function runWithReport(report: SREvaluationJob['report'], jobId = 'sr_eval_job_010') {
    vi.useFakeTimers()
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: jobId,
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    vi.mocked(getSREvaluationJob).mockResolvedValue(doneJob({ report }))

    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.click(button)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await tick()
  }

  it('Zone Evaluation report 顯示模型層與 Zone 層，不顯示治理與 Decision 層', async () => {
    await runWithReport({
      run_id: 'sr_eval_010',
      rows: 240,
      model_metrics: {
        model_available: true,
        hold: {
          rows: 240,
          positive_rows: 150,
          auc: 0.812,
          brier_score: 0.14,
          log_loss: 0.44,
          calibration: {
            rows: 240,
            binned_rows: 240,
            expected_calibration_error: 0.031,
            max_calibration_error: 0.09,
            insufficient_sample: false,
            bins: [
              { lower: 0.0, upper: 0.1, rows: 20, mean_predicted: 0.05, observed_rate: 0.1, gap: 0.05 },
              { lower: 0.9, upper: 1.0, rows: 0, mean_predicted: null, observed_rate: null, gap: null },
            ],
          },
        },
        break: { rows: 240, positive_rows: 60, auc: 0.735, brier_score: 0.18, log_loss: 0.52, calibration: null },
      },
      zone_outcomes: {
        rows: 240,
        support_hold_rate: 0.62,
        resistance_rejection_rate: 0.55,
        break_positive_rate: 0.21,
        average_forward_return: 0.012,
        // 分層 fixture 的形狀要跟 Python `_zone_outcome_group` 實際輸出一致：六個 key、
        // by_role 只有一種角色所以另一個比率是 null。這裡曾經憑印象手寫、用了
        // Python 從不產生的 key，於是前後端測試各自對著虛構的形狀互相印證。
        by_role: {
          SUPPORT: {
            rows: 130,
            hold_rate: 0.62,
            support_hold_rate: 0.62,
            resistance_rejection_rate: null,
            break_positive_rate: 0.19,
            average_forward_return: 0.015,
          },
          RESISTANCE: {
            rows: 110,
            hold_rate: 0.55,
            support_hold_rate: null,
            resistance_rejection_rate: 0.55,
            break_positive_rate: 0.23,
            average_forward_return: -0.004,
          },
        },
      },
    })

    expect(screen.getByText(/模型層指標/)).toBeInTheDocument()
    expect(screen.getByText(/hold AUC 0\.812/)).toBeInTheDocument()
    expect(screen.getByText(/Zone 層指標/)).toBeInTheDocument()
    expect(screen.getByText(/支撐守住 62\.0%/)).toBeInTheDocument()

    // 分層表要真的印出數字。只斷言「Zone 層指標」這個標題存在，正是那個 bug 能活這麼久的原因：
    // 三個比率欄位全是 `—` 也照樣通過。
    const supportRow = screen.getByText('SUPPORT').closest('tr')
    const supportCells = Array.from(supportRow?.querySelectorAll('td') ?? []).map((td) =>
      td.textContent?.trim()
    )
    expect(supportCells).toContain('130')
    expect(supportCells).toContain('62.0%') // support_hold_rate
    expect(supportCells).toContain('19.0%') // break_positive_rate
    // 只有一種角色的那一欄是 null → 破折號，不能印成 0.0%。
    expect(supportCells).toContain('—')
    expect(supportCells).not.toContain('0.0%')
    // Zone Evaluation 的 report 沒有這兩塊——先前面板只渲染治理區塊，才會整頁空白。
    expect(screen.queryByText('模型治理')).not.toBeInTheDocument()
    expect(screen.queryByText(/Decision 層指標/)).not.toBeInTheDocument()
  })

  it('Decision Replay report 顯示 Decision 層與確認成效，不顯示模型層', async () => {
    await runWithReport(
      {
        run_id: 'sr_replay_010',
        rows: 40,
        outcome_summary: {
          at_zone_rate: 0.12,
          rr_summary: {
            rows_with_entry_rr: 18,
            average_entry_rr: 1.85,
            median_entry_rr: 1.7,
            rows_with_position_rr: 12,
            average_position_rr: 2.1,
          },
          by_final_entry_state: {
            ENTRY_ALLOWED: {
              rows: 8,
              rows_with_forward_return: 8,
              average_forward_return: 0.023,
              positive_forward_return_rate: 0.75,
              negative_forward_return_rate: 0.25,
            },
          },
          daily_confirmation_summary: {
            rows: 40,
            support_next_hold_rate: 0.68,
            resistance_next_rejection_rate: 0.45,
            support_two_bar_confirm_rate: 0.52,
            average_next_close_return: 0.004,
            failure_distribution: { SUPPORT_CONFIRMATION_FAILED: 6 },
          },
        },
      },
      'sr_eval_job_011'
    )

    expect(screen.getByText(/Decision 層指標/)).toBeInTheDocument()
    expect(screen.getByText(/AT_ZONE 12\.0%/)).toBeInTheDocument()
    expect(screen.getByText(/平均 entry RR 1\.85R/)).toBeInTheDocument()
    expect(screen.getByText(/隔日／兩日確認成效/)).toBeInTheDocument()
    expect(screen.getByText(/支撐隔日守住 68\.0%/)).toBeInTheDocument()
    expect(screen.queryByText(/模型層指標/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Zone 層指標/)).not.toBeInTheDocument()
  })

  it('RR 分布把 p10/中位數/p90 攤開，沒有樣本的 RR 不列出來', async () => {
    // fixture 取自 2026-08-07 的真實 report 形狀：entry RR 平均遠高於中位數（右偏），
    // position RR 則是 count=0（真實資料上 position_rr 全數 UNAVAILABLE）。
    await runWithReport(
      {
        run_id: 'sr_replay_012',
        rows: 40,
        outcome_summary: {
          at_zone_rate: 0.12,
          rr_summary: {
            rows_with_entry_rr: 1889,
            average_entry_rr: 6.45,
            median_entry_rr: 2.34,
            rows_with_execution_rr: 931,
            median_execution_rr: 1.05,
            rows_with_position_rr: 0,
            entry_rr_distribution: {
              count: 1889, average: 6.45, stddev: 30.1, min: 0,
              p10: 0, p25: 0.98, median: 2.34, p75: 4.78, p90: 11.2, max: 1032,
            },
            execution_rr_distribution: {
              count: 931, average: 3.2, stddev: 8.4, min: 0,
              p10: 0, p25: 0.13, median: 1.05, p75: 2.47, p90: 5.08, max: 95.76,
            },
            position_rr_distribution: {
              count: 0, average: null, stddev: null, min: null,
              p10: null, p25: null, median: null, p75: null, p90: null, max: null,
            },
          },
        },
      },
      'sr_eval_job_013'
    )

    expect(screen.getByText(/RR 分布/)).toBeInTheDocument()

    // 有樣本的兩列要印出具體分位數，不能只驗標題存在
    const entryRow = screen.getByText('entry RR').closest('tr')
    const entryCells = Array.from(entryRow?.querySelectorAll('td') ?? []).map((td) => td.textContent?.trim())
    expect(entryCells).toContain('1889')
    expect(entryCells).toContain('2.34R') // 中位數
    expect(entryCells).toContain('11.20R') // p90
    expect(entryCells).toContain('1032.00R') // 極端值要看得到，這正是平均被拉高的原因

    expect(screen.getByText('execution RR')).toBeInTheDocument()
    // count=0 的 position RR 整列不出現——畫一排破折號只是噪音
    expect(screen.queryByText('position RR')).not.toBeInTheDocument()
  })

  it('模型不可用時 hold/break 是 null，仍要渲染且數值顯示破折號', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_011',
        model_metrics: { model_available: false, hold: null, break: null },
      },
      'sr_eval_job_012'
    )

    expect(screen.getByText(/模型不可用，本次無機率指標/)).toBeInTheDocument()
    // null 不能被印成 0——那會讓「沒資料」看起來像「完美校準」。
    expect(screen.getAllByText(/AUC=—/).length).toBe(2)
    expect(screen.getAllByText('無校準資料').length).toBe(2)
  })

  it('calibration 樣本不足時要標示不可用於調參', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_012',
        model_metrics: {
          model_available: true,
          hold: {
            rows: 20,
            auc: 0.6,
            calibration: {
              rows: 20,
              binned_rows: 20,
              expected_calibration_error: 0.21,
              insufficient_sample: true,
              bins: [],
            },
          },
          break: null,
        },
      },
      'sr_eval_job_013'
    )

    const notice = screen.getByText('樣本不足，ECE 抖動大，不可用於調參')
    // 這是警示，必須是紅色；tailwind 的 fall 是綠色，不能拿來標壞消息。
    expect(notice).toHaveClass('text-rise')
  })

  it('report 的 warnings 要顯示且用紅色', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_013',
        warnings: ['model unavailable: no such file'],
      },
      'sr_eval_job_014'
    )

    const warning = screen.getByText('model unavailable: no such file')
    expect(warning).toHaveClass('text-rise')
  })

  it('report 有 volatility_profiles 時顯示波動側寫，缺鍵時整區不出現', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_014',
        volatility_profiles: {
          '2330': {
            symbol: '2330',
            timeframe: '1d',
            bucket: 'HIGH_VOLATILITY',
            atr_pct: 0.042,
            average_range_pct: 0.038,
            touch_count: 26,
            candle_count: 1200,
            touch_density_per_100_bars: 2.17,
            lookback_bars: 240,
            thresholds: { low_volatility_max: 0.015, high_volatility_min: 0.035 },
          },
        },
      },
      'sr_eval_job_015'
    )

    expect(screen.getByText(/波動側寫/)).toBeInTheDocument()
    expect(screen.getByText('HIGH_VOLATILITY')).toBeInTheDocument()
    expect(screen.getByText('4.2%')).toBeInTheDocument()
    // 門檻要一起顯示，否則使用者無從判斷 bucket 是怎麼分出來的。
    expect(screen.getByText(/低波動 ≤ 1\.5%/)).toBeInTheDocument()
  })

  it('report 沒有 volatility_profiles 時不顯示波動側寫', async () => {
    await runWithReport({ run_id: 'sr_eval_015', rows: 10 }, 'sr_eval_job_016')

    expect(screen.queryByText(/波動側寫/)).not.toBeInTheDocument()
  })

  // daily confirmation 的分層才是 T-028 的價值所在：「量能不足時的隔日守住表現」
  // 這類問題總表的五個 rate 答不了。以下鎖住分層有被渲染、且分群是依語意切的。
  it('daily confirmation 的十五個分層依語意分三群顯示', async () => {
    const group = (rows: number) => ({
      rows,
      next_zone_result_counts: { SUPPORT_HELD: rows },
      two_bar_result_counts: { SUPPORT_CONFIRMED: rows },
      average_next_close_return: 0.004,
      average_two_bar_close_return: 0.006,
      positive_two_bar_return_rate: 0.6,
      negative_two_bar_return_rate: 0.4,
      failure_distribution: { SUPPORT_CONFIRMATION_OK: rows },
    })

    await runWithReport(
      {
        run_id: 'sr_eval_016',
        outcome_summary: {
          daily_confirmation_summary: {
            rows: 120,
            support_next_hold_rate: 0.68,
            by_state: { CONFIRMED: group(40) },
            by_primary_role: { SUPPORT: group(40) },
            by_volume_context: { VOLUME_CONFIRMED: group(40) },
            by_event_sequence: { TOUCH_THEN_HOLD: group(40) },
            by_market_event_types: { BREAKOUT: group(40) },
            by_event_market_state: { TRENDING: group(40) },
            by_rr_gate: { RR_OK: group(40) },
            by_rr_gate_reason_code: { RR_ABOVE_MIN: group(40) },
            by_rr_bucket: { 'RR_1.5_2.0': group(40) },
            // 2026-08-07 新增的六個細分層。欄位名與 Python
            // `_daily_confirmation_summary()` 的輸出一致，桶名取自真實 report。
            // 故意用「強桶在前、弱桶在後」的插入順序，驗證渲染會依序數重排
            by_volume_strength: { VOL_GTE_2_5: group(40), VOL_LT_0_8: group(30) },
            by_primary_market_event: { EXTREME_VOLUME: group(40) },
            by_market_event_count: { EVENTS_2: group(40) },
            by_stop_distance_bucket: { SD_1_TO_3PCT: group(40) },
            by_entry_executability: { EXECUTABLE_NOW: group(40) },
            by_rr_formula_state: { REWARD_MISSING: group(40) },
          },
        },
      },
      'sr_eval_job_017'
    )

    expect(screen.getByText(/結果面分層/)).toBeInTheDocument()
    expect(screen.getByText(/條件面分層/)).toBeInTheDocument()
    expect(screen.getByText(/RR 面分層/)).toBeInTheDocument()
    expect(screen.getByText('VOLUME_CONFIRMED')).toBeInTheDocument()
    expect(screen.getByText('RR_ABOVE_MIN')).toBeInTheDocument()

    // 逐個分層標題斷言，**不用「總共幾個 chip」這種總數**：每加一個分層就要改數字，
    // 而改錯數字比漏測更難察覺（見 docs/development-workflow.md §3）。這裡漏接任何一個分層都會失敗。
    for (const title of [
      '依確認狀態', '依 zone 角色',
      '依量能條件', '依量能強弱', '依事件序列', '依主要事件', '依事件數',
      '依市場事件類型', '依事件市場狀態',
      '依 RR gate', '依 RR gate 原因碼', '依 RR bucket',
      '依停損距離', '依進場可執行性', '依 RR 公式齊備性',
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument()
    }
    // 新分層的桶名要真的出現在畫面上，不是只有標題
    for (const bucket of [
      'VOL_GTE_2_5', 'EXTREME_VOLUME', 'EVENTS_2',
      'SD_1_TO_3PCT', 'EXECUTABLE_NOW', 'REWARD_MISSING',
    ]) {
      expect(screen.getByText(bucket)).toBeInTheDocument()
    }

    // 序數桶要依強弱排序，**不是字典序**：localeCompare 會把 VOL_LT_0_8 排到
    // VOL_GTE_2_5 後面，讀者由上往下掃會把最弱的看成最強之後而誤判單調趨勢。
    // 標題的下一個兄弟節點才是對應的表；用 parentElement.querySelector('table')
    // 會抓到容器裡的第一張表（也就是上一個分層的），測到錯的東西。
    const volumeTable = screen.getByText('依量能強弱').nextElementSibling as HTMLElement
    expect(volumeTable?.tagName).toBe('TABLE')
    const volumeRows = Array.from(volumeTable.querySelectorAll('tbody tr'))
      .map((tr) => tr.querySelector('td')?.textContent?.trim())
      // chip 列（隔日/兩日/失敗）與桶列同在 tbody，只取桶名那幾列
      .filter((text): text is string => !!text && text.startsWith('VOL_'))
    expect(volumeRows).toEqual(['VOL_LT_0_8', 'VOL_GTE_2_5'])
    // chip 前綴仍要標明來源，否則分不出是隔日還是兩日的結果
    expect(screen.getAllByText('隔日/SUPPORT_HELD').length).toBeGreaterThan(0)
  })

  it('只有部分分層有資料時，其餘的群整塊不出現', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_017',
        outcome_summary: {
          daily_confirmation_summary: {
            rows: 30,
            by_volume_context: { VOLUME_CONFIRMED: { rows: 30, average_two_bar_close_return: 0.01 } },
          },
        },
      },
      'sr_eval_job_018'
    )

    expect(screen.getByText(/條件面分層/)).toBeInTheDocument()
    expect(screen.queryByText(/結果面分層/)).not.toBeInTheDocument()
    expect(screen.queryByText(/RR 面分層/)).not.toBeInTheDocument()
  })

  it('分層樣本數低於門檻才標示樣本不足，足夠的不標', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_018',
        outcome_summary: {
          daily_confirmation_summary: {
            rows: 45,
            by_rr_gate: {
              RR_BLOCKED: { rows: 3, positive_two_bar_return_rate: 1 },
              RR_OK: { rows: 42, positive_two_bar_return_rate: 0.55 },
            },
          },
        },
      },
      'sr_eval_job_019'
    )

    // 兩個方向一起斷言：只標樣本少的那組。只測單邊的話，「永遠顯示」也會綠。
    const notices = screen.getAllByText('樣本不足')
    expect(notices.length).toBe(1)
    // 警示一律紅色；tailwind 的 fall 是綠色，不能拿來標壞消息。
    expect(notices[0]).toHaveClass('text-rise')
  })

  it('分層報酬為 null 時顯示破折號而非 0', async () => {
    await runWithReport(
      {
        run_id: 'sr_eval_019',
        outcome_summary: {
          daily_confirmation_summary: {
            rows: 25,
            positive_two_bar_return_rate: 0.5,
            negative_two_bar_return_rate: 0.5,
            by_state: {
              UNRESOLVED: {
                rows: 25,
                average_next_close_return: null,
                average_two_bar_close_return: null,
                positive_two_bar_return_rate: null,
                negative_two_bar_return_rate: null,
              },
            },
          },
        },
      },
      'sr_eval_job_020'
    )

    // 摘要行新增的兩個 rate 也要露出來。
    expect(screen.getByText(/兩日正報酬率=50\.0%/)).toBeInTheDocument()

    // 只斷言全頁的破折號數量會過鬆（`—` 到處都是），要鎖到這一列上。
    const row = screen.getByText('UNRESOLVED').closest('tr')
    const cells = Array.from(row?.querySelectorAll('td') ?? []).map((td) => td.textContent?.trim())
    // 名稱、rows(25)，其餘四欄全 null → 四個破折號。
    expect(cells.filter((text) => text === '—').length).toBe(4)
    // null 印成 0% 會讓「沒資料」看起來像「一半機率」，這是這條測試真正要擋的事。
    expect(cells).not.toContain('0.0%')
  })
})

// 四個 ATR 參數是 T-003 的調參入口：Go 與 Python 早就支援，先前只是前端沒開欄位。
// 留白必須送 undefined，API 層再轉成「整個鍵不送」。0 與負數同樣不送——Go 的
// `omitempty` 會在轉發給 Python 前把 0 丟掉，前端硬送只會得到「參數收下卻無效果」的
// 靜默失效（見 srZones.ts 的 optionalBuilderParams）。
describe('SRZones evaluation 的 zone builder 參數輸入', () => {
  const ATR_WIDTH_TITLE = 'atr_width_multiplier：zone 寬度 = ATR × 此倍數'
  const ATR_PERIOD_TITLE = 'atr_period：ATR 本身的計算期數'

  it('四個參數留白時送出 undefined，不干擾後端預設', async () => {
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_020',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })

    await fireEvent.click(button)

    expect(runSREvaluation).toHaveBeenCalledWith(
      expect.objectContaining({
        atrWidthMultiplier: undefined,
        maxMergeWidthMultiple: undefined,
        atrLookback: undefined,
        atrPeriod: undefined,
      })
    )
  })

  it('填入的參數以數字送出', async () => {
    vi.mocked(runSREvaluation).mockResolvedValue({
      job_id: 'sr_eval_job_021',
      status: 'pending',
      message: '已在背景開始 evaluation',
      symbols: 1,
    })
    const button = await renderSRZones()
    await fireEvent.input(screen.getByPlaceholderText(SYMBOLS_PLACEHOLDER), { target: { value: '2330' } })
    await fireEvent.input(screen.getByTitle(ATR_WIDTH_TITLE), { target: { value: '1.2' } })
    await fireEvent.input(screen.getByTitle(ATR_PERIOD_TITLE), { target: { value: '14' } })

    await fireEvent.click(button)

    expect(runSREvaluation).toHaveBeenCalledWith(
      expect.objectContaining({ atrWidthMultiplier: 1.2, atrPeriod: 14 })
    )
  })
})
