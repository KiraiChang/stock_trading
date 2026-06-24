import { apiFetch } from './client'

export async function login(
  email: string,
  password: string,
): Promise<{ token: string; expires_in: number }> {
  return apiFetch('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export async function register(
  email: string,
  password: string,
): Promise<{ user_id: number; email: string }> {
  return apiFetch('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}
