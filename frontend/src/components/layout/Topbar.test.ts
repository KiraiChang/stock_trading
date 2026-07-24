import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/svelte'
import Topbar from './Topbar.svelte'

// 元件種子測試：驗證 svelte 編譯 + jsdom 渲染 + jest-dom matcher 整條 harness 可用。
describe('Topbar', () => {
  it('渲染標題與登出按鈕', () => {
    const { getByText, getByRole } = render(Topbar)

    expect(getByText('台股技術分析')).toBeInTheDocument()
    expect(getByRole('button', { name: '登出' })).toBeInTheDocument()
  })
})
