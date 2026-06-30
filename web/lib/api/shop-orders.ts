import { api } from './client'
import type { AdminShopOrder, UUID } from '@/types/api'

export const shopOrdersApi = {
  list: (status?: string) =>
    api
      .get('admin/shop/orders', { searchParams: status ? { status, limit: 100 } : { limit: 100 } })
      .json<{ items: AdminShopOrder[] }>()
      .then((r) => r.items),

  updateStatus: (id: UUID, status: string) =>
    api.put(`admin/shop/orders/${id}/status`, { json: { status } }).json<{ status: string }>(),
}
