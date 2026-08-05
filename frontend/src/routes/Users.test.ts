import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Users from './Users.svelte'
import { fetchUsers, type UserItem } from '../lib/api/users'

vi.mock('../lib/api/users', () => ({
  fetchUsers: vi.fn(),
  updateUserStatus: vi.fn(),
}))

function user(overrides: Partial<UserItem> = {}): UserItem {
  return {
    id: 1,
    email: 'a@example.com',
    status: 'active',
    created_at: '2026-08-01T00:00:00Z',
    ...overrides,
  }
}

beforeEach(() => {
  vi.mocked(fetchUsers).mockReset()
})

// tailwind.config.js：rise=#e74c3c(紅)、fall=#2ecc71(綠) 是台股行情色。
// 破壞性動作按鈕先前誤用 text-fall，導致「停用」與「啟用」兩個狀態都是綠色，
// 使用者完全分不出哪個是危險操作。
describe('Users 頁面的狀態切換按鈕顏色', () => {
  it('停用（破壞性）用紅色、啟用用綠色，兩者必須可區分', async () => {
    vi.mocked(fetchUsers).mockResolvedValue([
      user({ id: 1, email: 'active@example.com', status: 'active' }),
      user({ id: 2, email: 'inactive@example.com', status: 'inactive' }),
    ])
    render(Users)

    const deactivate = await screen.findByRole('button', { name: '停用' })
    const activate = screen.getByRole('button', { name: '啟用' })

    expect(deactivate).toHaveClass('text-red-400')
    expect(deactivate).not.toHaveClass('text-fall')
    expect(activate).toHaveClass('text-green-400')
  })
})
