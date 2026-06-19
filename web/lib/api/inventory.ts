import { api } from './client'
import type { Inventory, UpdateInventoryRequest } from '@/types/api'

export const inventoryApi = {
  list: () => api.get('inventory').json<Inventory[]>(),
  listLowStock: () => api.get('inventory/low-stock').json<Inventory[]>(),
  getById: (id: string) => api.get(`inventory/${id}`).json<Inventory>(),
  getTimeline: (id: string) => api.get(`inventory/${id}/timeline`).json<Array<{ id: string; action: string; created_at: string; user_id?: string | null }>>(),
  create: (data: { product_id: string; quantity: number; low_stock_threshold: number }) => api.post('inventory', { json: data }).json<Inventory>(),
  getByProductId: (productId: string) => api.get(`inventory/${productId}`).json<Inventory>(),
  update: (productId: string, data: UpdateInventoryRequest) => api.put(`inventory/${productId}`, { json: data }).json<Inventory>(),
}
