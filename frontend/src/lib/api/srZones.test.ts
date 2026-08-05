import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from './client'
import {
  getSREvaluationJob,
  getSRZoneAnalysis,
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

  // 留白的 builder 參數必須整個鍵不送：Go 的 omitempty 只擋 0 值，送 null 會讓 Python
  // 把它當成明確設定而覆寫預設值。
  it('omits zone builder params entirely when not provided', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ job_id: 'sr_eval_job_003', status: 'pending', message: 'started', symbols: 1 })

    await runSREvaluation({ symbols: ['2330'] })

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    const body = JSON.parse(String(init?.body))
    expect(Object.keys(body)).not.toContain('atr_width_multiplier')
    expect(Object.keys(body)).not.toContain('max_merge_width_multiple')
    expect(Object.keys(body)).not.toContain('atr_lookback')
    expect(Object.keys(body)).not.toContain('atr_period')
  })

  it('sends zone builder params as numbers when provided', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ job_id: 'sr_eval_job_004', status: 'pending', message: 'started', symbols: 1 })

    await runSREvaluation({
      symbols: ['2330'],
      atrWidthMultiplier: 1.2,
      maxMergeWidthMultiple: 2.5,
      atrLookback: 120,
      atrPeriod: 14,
    })

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    expect(JSON.parse(String(init?.body))).toMatchObject({
      atr_width_multiplier: 1.2,
      max_merge_width_multiple: 2.5,
      atr_lookback: 120,
      atr_period: 14,
    })
  })

  // 0 / 負數 / NaN 都不送：Go 的 omitempty 讓 0 傳不到 Python，照送只會變成
  // 「參數收下卻毫無效果」的靜默失效。NaN 則是 <input type="number"> 輸入非數字的產物。
  it('drops non-positive and NaN zone builder params', async () => {
    vi.mocked(apiFetch).mockResolvedValue({ job_id: 'sr_eval_job_005', status: 'pending', message: 'started', symbols: 1 })

    await runSREvaluation({
      symbols: ['2330'],
      atrWidthMultiplier: 0,
      maxMergeWidthMultiple: -1,
      atrPeriod: Number.NaN,
      atrLookback: 120,
    })

    const [, init] = vi.mocked(apiFetch).mock.calls[0]
    const body = JSON.parse(String(init?.body))
    expect(Object.keys(body)).not.toContain('atr_width_multiplier')
    expect(Object.keys(body)).not.toContain('max_merge_width_multiple')
    expect(Object.keys(body)).not.toContain('atr_period')
    // 正數仍要照送，護欄不能把有效值一起吃掉。
    expect(body.atr_lookback).toBe(120)
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

// zone_builder_runtime_config 走 analysis 區塊回來，之前在 Go 端就被丟掉。
// 這組測試鎖的是「API 層有把它normalize 進 analysis 物件」，不是 UI 呈現。
describe('srZones analysis 的 zone builder runtime config', () => {
  function pipelineResponse(analysisExtra: Record<string, unknown>) {
    return {
      pipeline_version: 'v2',
      analysis: {
        id: 1,
        symbol: '2330',
        timeframe: '1d',
        analyzed_at: '2026-08-05T00:00:00Z',
        current_price: 600,
        model_version: 'v4',
        model_config_hash: 'cfg',
        period_summaries: [],
        analysis_tips: [],
        chip_summary: null,
        created_at: '2026-08-05T00:00:00Z',
        ...analysisExtra,
      },
      features: { global_trend: 0.03, global_volatility: 0.02 },
      score: { global_expected_value: 0.01, global_confidence: 0.7, global_risk_reward_ratio: 1.5 },
      evidence: null,
      decision: null,
      explanation: null,
      scenario: null,
      probability_context: null,
      zones: [],
    }
  }

  it('normalizes zone_builder_runtime_config into the analysis object', async () => {
    vi.mocked(apiFetch).mockResolvedValue(
      pipelineResponse({
        zone_builder_runtime_config: {
          enabled: true,
          bucket: 'HIGH_VOLATILITY',
          reason_code: 'VOLATILITY_BUCKET_CONFIG',
        },
      })
    )

    const { analysis } = await getSRZoneAnalysis(1)

    expect(analysis.zone_builder_runtime_config).toEqual({
      enabled: true,
      bucket: 'HIGH_VOLATILITY',
      reason_code: 'VOLATILITY_BUCKET_CONFIG',
    })
  })

  // 057 migration 之前的舊分析回 null＝沒有這項紀錄。不可被補成 {enabled:false}，
  // 那會把「沒紀錄」變成「adaptive 明確關閉」，是兩件不同的事。
  it('keeps null for analyses recorded before the column existed', async () => {
    vi.mocked(apiFetch).mockResolvedValue(pipelineResponse({ zone_builder_runtime_config: null }))

    const { analysis } = await getSRZoneAnalysis(1)

    expect(analysis.zone_builder_runtime_config).toBeNull()
  })
})
