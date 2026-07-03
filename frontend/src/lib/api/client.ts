import { authLogout, getToken } from '../stores/auth'

const BASE_URL = (import.meta.env.VITE_API_URL as string) ?? ''

// ApiError 保留後端回應的 HTTP 狀態碼與 error 訊息，讓呼叫端可以依狀態碼
// 顯示不同提示（例如 503=模型未訓練、404=資料不足、502=上游服務沒開），
// 不是每種錯誤都顯示同一句話。後端已經把「安全、不外洩內部細節」的訊息
// 組好放在 error 欄位，這裡只是原樣往上傳遞，不做二次加工。
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function apiFetch<T>(path: string, opts?: RequestInit): Promise<T> {
  const token = getToken()
  const res = await fetch(`${BASE_URL}/api/v1${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...opts,
  })

  if (res.status === 401) {
    authLogout()
    throw new ApiError(401, 'Unauthorized')
  }

  if (!res.ok) {
    let message = `API ${path} failed: ${res.status}`
    try {
      const body = await res.json()
      if (body && typeof body.error === 'string' && body.error) message = body.error
    } catch {
      // 回應不是 JSON 或沒有 error 欄位，用預設訊息
    }
    throw new ApiError(res.status, message)
  }
  return res.json() as Promise<T>
}
