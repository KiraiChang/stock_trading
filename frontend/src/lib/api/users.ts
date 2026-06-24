import { apiFetch } from './client'

export interface UserItem {
  id: number
  email: string
  status: 'active' | 'inactive'
  created_at: string
}

export async function fetchUsers(): Promise<UserItem[]> {
  const res = await apiFetch<{ users: UserItem[] }>('/users')
  return res.users ?? []
}

export async function updateUserStatus(id: number, status: 'active' | 'inactive'): Promise<void> {
  await apiFetch(`/users/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}
