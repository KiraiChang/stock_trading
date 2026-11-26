import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'

vi.mock('../../lib/api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/api/client')>()),
  apiFetch: vi.fn().mockResolvedValue({}),
}))

import WatchlistTable from './WatchlistTable.svelte'
import { selectedSymbol, watchlist } from '../../lib/stores/market'
import { currentRoute } from '../../lib/stores/router'

beforeEach(() => {
  selectedSymbol.set('')
  currentRoute.set('dashboard')
  watchlist.set([
    { id: 1, symbol: '2330', name: '台積電', sector: '半導體', watched: false },
    { id: 2, symbol: '2454', name: '聯發科', sector: '半導體', watched: false },
  ])
})

// T-089 D4：觀察清單接交易計畫
describe('WatchlistTable 的「計畫」按鈕', () => {
  it('按「計畫」設定 selectedSymbol 並切到交易分析頁', async () => {
    render(WatchlistTable)
    await fireEvent.click(screen.getByTestId('open-plan-2454'))
    expect(get(selectedSymbol)).toBe('2454')
    expect(get(currentRoute)).toBe('positions')
  })

  it('點列本身只切換 K 線圖、⛔ 不切換 route', async () => {
    render(WatchlistTable)
    await fireEvent.click(screen.getByText('台積電'))
    expect(get(selectedSymbol)).toBe('2330')
    expect(get(currentRoute)).toBe('dashboard')
  })
})
