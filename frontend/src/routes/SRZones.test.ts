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
        by_role: { SUPPORT: { rows: 130, support_hold_rate: 0.62, average_forward_return: 0.015 } },
      },
    })

    expect(screen.getByText(/模型層指標/)).toBeInTheDocument()
    expect(screen.getByText(/hold AUC 0\.812/)).toBeInTheDocument()
    expect(screen.getByText(/Zone 層指標/)).toBeInTheDocument()
    expect(screen.getByText(/支撐守住 62\.0%/)).toBeInTheDocument()
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
})

// 四個 ATR 參數是 T-003 的調參入口：Go 與 Python 早就支援，先前只是前端沒開欄位。
// 留白必須送 undefined（API 層再轉成「整個鍵不送」），不可送 0——0 是合法設定值。
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
