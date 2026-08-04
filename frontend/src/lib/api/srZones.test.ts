import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import {
  getSREvaluationJob,
  listSREvaluationJobs,
  listSRRegressionResults,
  runSREvaluation,
} from './srZones'

vi.mock('./client', () => ({
  apiFetch: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(apiFetch).mockReset()
})

describe('srZones evaluation api', () => {
  it('starts evaluation with explicit replay options', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      job_id: 'sr_eval_job_001',
      status: 'pending',
      message: 'started',
      symbols: 2,
    })

    const result = await runSREvaluation({
      symbols: ['2330', '0050'],
      timeframe: '1d',
      limit: 900,
      writeDb: true,
      decisionReplay: true,
      replayMaxRows: 50,
    })

    expect(result.job_id).toBe('sr_eval_job_001')
    expect(apiFetch).toHaveBeenCalledWith('/sr-zones/evaluate', {
      method: 'POST',
      body: JSON.stringify({
        symbols: ['2330', '0050'],
        timeframe: '1d',
        limit: 900,
        write_db: true,
        decision_replay: true,
        replay_max_rows: 50,
      }),
    })
  })

  it('uses conservative evaluation defaults', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ job_id: 'sr_eval_job_002', status: 'pending', message: 'started', symbols: 1 })

    await runSREvaluation({ symbols: ['2330'] })

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    expect(JSON.parse(String(init?.body))).toEqual({
      symbols: ['2330'],
      timeframe: '1d',
      limit: 1500,
      write_db: false,
      decision_replay: false,
      replay_max_rows: 200,
    })
  })

  it('reads evaluation jobs and regression results from the expected endpoints', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce({ job: { job_id: 'sr_eval_job_001' } })
      .mockResolvedValueOnce({ jobs: [{ job_id: 'sr_eval_job_002' }], total: 1 })
      .mockResolvedValueOnce({ results: [{ run_id: 'sr_replay_001' }], total: 1 })

    await expect(getSREvaluationJob('sr_eval_job_001')).resolves.toMatchObject({ job_id: 'sr_eval_job_001' })
    await expect(listSREvaluationJobs(7)).resolves.toEqual([{ job_id: 'sr_eval_job_002' }])
    await expect(listSRRegressionResults('sr_zone_decision_replay_p0', 11)).resolves.toEqual([{ run_id: 'sr_replay_001' }])

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/sr-zones/evaluation-jobs/sr_eval_job_001')
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/sr-zones/evaluation-jobs?limit=7')
    expect(apiFetch).toHaveBeenNthCalledWith(
      3,
      '/sr-zones/regression-results?limit=11&schema_version=sr_zone_decision_replay_p0'
    )
  })
})
