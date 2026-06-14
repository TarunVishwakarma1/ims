import { api } from './client'
import type { Order, OrderItem, CreateOrderRequest, UpdateOrderStatusRequest } from '@/types/api'

export const ordersApi = {
  list: () => api.get('orders').json<Order[]>(),
  getById: (id: string) => api.get(`orders/${id}`).json<Order>(),
  getItems: (id: string) => api.get(`orders/${id}/items`).json<OrderItem[]>(),
  create: (data: CreateOrderRequest) => api.post('orders', { json: data }).json<Order>(),
  updateStatus: (id: string, data: UpdateOrderStatusRequest) => api.put(`orders/${id}/status`, { json: data }).json<{ message: string }>(),
  delete: (id: string) => api.delete(`orders/${id}`),
}
