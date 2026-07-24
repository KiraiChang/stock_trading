import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { apiFetch, ApiError } from './client'
import { authLogout } from '../stores/auth'

// stores/auth 會在 module load 讀 localStorage 並回傳 token；這裡 mock 掉，
// 專注測 apiFetch 對 HTTP 狀態碼與錯誤 body 的處理。
vi.mock('../stores/auth', () => ({
  getToken: vi.fn(() => 'tkn'),
  authLogout: vi.fn(),
}))

function mockFetchOnce(res: Partial<Response> & { json?: () => Promise<unknown> }) {
  const fetchMock = vi.fn().mockResolvedValue(res)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('apiFetch', () => {
  it('成功時回傳 json，並帶上 Bearer token 與 /api/v1 前綴', async () => {
    const fetchMock = mockFetchOnce({ ok: true, status: 200, json: async () => ({ a: 1 }) })

    const data = await apiFetch<{ a: number }>('/x')

    expect(data).toEqual({ a: 1 })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/x')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tkn')
  })

  it('非 2xx 時丟出 ApiError，並採用後端 error 訊息', async () => {
    mockFetchOnce({ ok: false, status: 404, json: async () => ({ error: '資料不足' }) })

    await expect(apiFetch('/x')).rejects.toMatchObject({
      name: 'ApiError',
      status: 404,
      message: '資料不足',
    })
  })

  it('401 時登出並丟 ApiError', async () => {
    mockFetchOnce({ ok: false, status: 401, json: async () => ({}) })

    await expect(apiFetch('/x')).rejects.toBeInstanceOf(ApiError)
    expect(vi.mocked(authLogout)).toHaveBeenCalledOnce()
  })
})
