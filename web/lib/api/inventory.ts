import { api } from './client'
import type { Inventory, UpdateInventoryRequest } from '@/types/api'

export const inventoryApi = {
  list: () => api.get('inventory').json<Inventory[]>(),
  getByProductId: (productId: string) => api.get(`inventory/${productId}`).json<Inventory>(),
  update: (productId: string, data: UpdateInventoryRequest) => api.put(`inventory/${productId}`, { json: data }).json<Inventory>(),
}
