import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Analysis from './Analysis.svelte'
import { listAnalyses, type StockAnalysis } from '../lib/api/analysis'

vi.mock('../lib/api/analysis', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api/analysis')>()
  return {
    ...actual,
    createAnalysis: vi.fn(),
    listAnalyses: vi.fn(),
    getAnalysis: vi.fn(),
    verifyAnalysis: vi.fn(),
    deleteAnalysis: vi.fn(),
  }
})

function analysisRow(): StockAnalysis {
  return {
    id: 1,
    symbol: '2330',
    timeframe: '1d',
    analyzed_at: '2026-08-01T00:00:00Z',
    current_price: 1000,
    trend: 'BULLISH',
    entry_status: 'WATCHING',
    entry_direction: 'LONG',
    entry_price: 990,
    created_at: '2026-08-01T00:00:00Z',
  }
}

beforeEach(() => {
  vi.mocked(listAnalyses).mockReset()
})

// 破壞性動作按鈕先前用 text-fall（依色票是綠色），把「刪除」標成安全色。
describe('Analysis 頁面的刪除按鈕顏色', () => {
  it('刪除分析用紅色，不能用行情色 text-fall', async () => {
    vi.mocked(listAnalyses).mockResolvedValue([analysisRow()])
    render(Analysis)

    const remove = await screen.findByRole('button', { name: '刪除' })

    expect(remove).toHaveClass('text-red-400')
    expect(remove).not.toHaveClass('text-fall')
  })
})
