import { apiFetch } from './client'

export type PortfolioOwnerType = 'TENANT' | 'USER' | 'GROUP'

export interface Portfolio {
  id: number
  tenant_id: number
  name: string
  owner_type: PortfolioOwnerType | string
  owner_id: number | null
  created_by_user_id: number | null
  is_default: boolean
  can_write: boolean
  created_at: string
  updated_at: string
}

export async function listPortfolios(): Promise<Portfolio[]> {
  const response = await apiFetch<{ portfolios: Portfolio[] }>('/portfolios')
  return response.portfolios ?? []
}

export async function createPortfolio(input: { name: string; group_id?: number }): Promise<Portfolio> {
  const response = await apiFetch<{ portfolio: Portfolio }>('/portfolios', {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return response.portfolio
}
