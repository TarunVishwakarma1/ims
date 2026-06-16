import { api } from './client'
import type { Order, Organization } from '@/types/api'

export type PartnerRole = 'supplier' | 'buyer'

export interface Partner {
  org_id: string
  name: string
  slug: string
  role: PartnerRole
  order_count: number
  total_value: number
  pending_count: number
  last_order_at: string | null
}

export interface PartnerDetail {
  organization: Organization
  as_supplier?: Partner
  as_buyer?: Partner
  recent_orders: Order[]
}

export const partnersApi = {
  list: (role: PartnerRole) =>
    api.get('partners', { searchParams: { role } }).json<Partner[]>(),
  getById: (id: string) => api.get(`partners/${id}`).json<PartnerDetail>(),
}
