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
      writeDb: true,
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
