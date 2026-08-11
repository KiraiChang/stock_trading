import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import Scheduler from './Scheduler.svelte'
import {
  fetchSchedulerStatus,
  triggerSREvaluationRun,
  triggerCorporateActionSyncRun,
  type SchedulerJob,
} from '../lib/api/scheduler'

// 元件層測試：API 契約已由 scheduler.test.ts 保護，這裡只驗「手動觸發 SR 驗證」
// 的元件狀態機（是否呼叫 API、進行中禁用、成功／失敗訊息）。
vi.mock('../lib/api/scheduler', () => ({
  fetchSchedulerStatus: vi.fn(),
  triggerDailyCloseRun: vi.fn(),
  triggerStockSymbolSyncRun: vi.fn(),
  triggerSREvaluationRun: vi.fn(),
  triggerCorporateActionSyncRun: vi.fn(),
}))

const srEvaluationJob: SchedulerJob = {
  job_name: 'sr_evaluation',
  status: 'success',
  symbols_total: 2,
  symbols_failed: 0,
  stale: false,
  started_at: '2026-08-04T02:00:00Z',
  finished_at: '2026-08-04T02:03:00Z',
}

beforeEach(() => {
  vi.mocked(fetchSchedulerStatus).mockReset()
  vi.mocked(triggerSREvaluationRun).mockReset()
  vi.mocked(fetchSchedulerStatus).mockResolvedValue([srEvaluationJob])
})

// onMount 的第一次載入完成前畫面停在「載入中...」，用 findBy 等到列表渲染出來。
async function renderSchedulerPage() {
  render(Scheduler)
  return screen.findByRole('button', { name: '手動執行 SR 驗證' })
}

describe('Scheduler 頁面的 sr_evaluation 區塊', () => {
  it('渲染 SR Zone 驗證排程列與手動執行按鈕', async () => {
    await renderSchedulerPage()

    expect(screen.getByText('SR Zone 驗證')).toBeInTheDocument()
    expect(screen.getByText('成功')).toBeInTheDocument()
  })

  it('點擊手動執行會呼叫 triggerSREvaluationRun 並顯示後端訊息', async () => {
    vi.mocked(triggerSREvaluationRun).mockResolvedValue({ message: '已在背景觸發 SR evaluation' })
    const button = await renderSchedulerPage()

    await fireEvent.click(button)

    expect(triggerSREvaluationRun).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('已在背景觸發 SR evaluation')).toBeInTheDocument()
  })

  it('觸發進行中時按鈕禁用並顯示觸發中文案', async () => {
    let resolveTrigger: (value: { message: string }) => void = () => {}
    vi.mocked(triggerSREvaluationRun).mockReturnValue(
      new Promise((resolve) => {
        resolveTrigger = resolve
      })
    )
    const button = await renderSchedulerPage()

    await fireEvent.click(button)

    const pending = await screen.findByRole('button', { name: '觸發中...' })
    expect(pending).toBeDisabled()

    resolveTrigger({ message: 'ok' })
    await waitFor(() => expect(screen.getByRole('button', { name: '手動執行 SR 驗證' })).toBeEnabled())
  })

  it('觸發失敗時顯示錯誤訊息且不留在觸發中狀態', async () => {
    vi.mocked(triggerSREvaluationRun).mockRejectedValue(new Error('boom'))
    const button = await renderSchedulerPage()

    await fireEvent.click(button)

    expect(await screen.findByText('觸發失敗，請稍後再試')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '手動執行 SR 驗證' })).toBeEnabled()
  })
})

describe('Scheduler 頁面的錯誤色語意', () => {
  // tailwind.config.js：rise=#e74c3c(紅)、fall=#2ecc71(綠)，是台股「漲紅跌綠」的行情色。
  // 作業層的錯誤與失敗數是警示，必須紅色；先前誤用 text-fall 會把失敗顯示成綠色。
  it('job 錯誤與失敗數用紅色（text-rise）', async () => {
    vi.mocked(fetchSchedulerStatus).mockResolvedValue([
      {
        ...srEvaluationJob,
        status: 'failed',
        symbols_failed: 3,
        error: 'python upstream unavailable',
      },
    ])
    await renderSchedulerPage()

    expect(screen.getByText('python upstream unavailable')).toHaveClass('text-rise')
    expect(screen.getByText('3')).toHaveClass('text-rise')
  })

  it('手動觸發失敗訊息用紅色（text-rise）', async () => {
    vi.mocked(triggerSREvaluationRun).mockRejectedValue(new Error('boom'))
    const button = await renderSchedulerPage()

    await fireEvent.click(button)

    expect(await screen.findByText('觸發失敗，請稍後再試')).toHaveClass('text-rise')
  })
})

describe('Scheduler 頁面的 corporate_action_sync 區塊', () => {
  const job: SchedulerJob = {
    job_name: 'corporate_action_sync',
    status: 'never_run',
    symbols_total: 0,
    symbols_failed: 0,
    stale: true,
  }

  beforeEach(() => {
    vi.mocked(triggerCorporateActionSyncRun).mockReset()
    vi.mocked(fetchSchedulerStatus).mockResolvedValue([job])
  })

  // 這個入口存在的理由：排程是平日 06:30，部署若晚於那個時間，
  // 沒有手動觸發就得等到隔天才驗得了還原是否正確。
  it('渲染排程列與手動執行按鈕', async () => {
    render(Scheduler)
    expect(await screen.findByRole('button', { name: '手動執行還原同步' })).toBeTruthy()
    expect(screen.getByText('公司行動與股價還原')).toBeTruthy()
  })

  it('點擊手動執行會呼叫 API 並顯示後端訊息', async () => {
    vi.mocked(triggerCorporateActionSyncRun).mockResolvedValue({ message: '已在背景重新觸發' })
    render(Scheduler)
    const btn = await screen.findByRole('button', { name: '手動執行還原同步' })

    await fireEvent.click(btn)

    await waitFor(() => expect(triggerCorporateActionSyncRun).toHaveBeenCalledTimes(1))
    expect(await screen.findByText('已在背景重新觸發')).toBeTruthy()
  })

  it('觸發失敗時顯示錯誤訊息且不留在觸發中狀態', async () => {
    vi.mocked(triggerCorporateActionSyncRun).mockRejectedValue(new Error('boom'))
    render(Scheduler)
    const btn = await screen.findByRole('button', { name: '手動執行還原同步' })

    await fireEvent.click(btn)

    expect(await screen.findByText('觸發失敗，請稍後再試')).toBeTruthy()
    expect(screen.getByRole('button', { name: '手動執行還原同步' })).toBeTruthy()
  })
})
