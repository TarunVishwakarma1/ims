import { api } from './client'
import type { Order, OrderItem, CreateOrderRequest, UpdateOrderStatusRequest } from '@/types/api'

export interface CancelPreview {
  eligible: boolean
  status: string
  payment_paid: boolean
  refund_percent: number
  refund_amount: number
  reason: string
  blocked?: string
}

export interface OrderTimelineEntry {
  id: string
  org_id: string
  user_id?: string | null
  action: string
  entity: string
  entity_id: string
  ip_address: string
  created_at: string
}

export interface OrderListResult {
  items: Order[]
  total: number
  page: number
  per_page: number
}

export interface OrderListFilters {
  status?: string
  payment_status?: string
  order_type?: string
  search?: string
  from?: string // RFC3339
  to?: string   // RFC3339
  page?: number
  per_page?: number
}

function filtersToSearchParams(f: OrderListFilters): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === '' || v === null) continue
    out[k] = String(v)
  }
  return out
}

export const ordersApi = {
  list: (filters: OrderListFilters = {}) =>
    api.get('orders', { searchParams: filtersToSearchParams(filters) }).json<OrderListResult>(),
  getById: (id: string) => api.get(`orders/${id}`).json<Order>(),
  getItems: (id: string) => api.get(`orders/${id}/items`).json<OrderItem[]>(),
  getTimeline: (id: string) => api.get(`orders/${id}/timeline`).json<OrderTimelineEntry[]>(),
  getCancelPreview: (id: string) =>
    api.get(`orders/${id}/cancel-preview`).json<CancelPreview>(),
  create: (data: CreateOrderRequest) => api.post('orders', { json: data }).json<Order>(),
  updateStatus: (id: string, data: UpdateOrderStatusRequest) =>
    api.put(`orders/${id}/status`, { json: data }).json<{ message: string }>(),
  bulkStatus: (ids: string[], status: string) =>
    api.post('orders/bulk-status', { json: { ids, status } }).json<{ applied: number; skipped: number }>(),
  cancel: (id: string, reason?: string) =>
    api.post(`orders/${id}/cancel`, { json: { reason: reason || '' } }).json<{ message: string }>(),
  delete: (id: string) => api.delete(`orders/${id}`),
  exportCsv: (filters: OrderListFilters = {}) =>
    api.get('orders/export', { searchParams: filtersToSearchParams(filters) }).blob(),
  invoicePdf: (id: string) => api.get(`orders/${id}/invoice.pdf`).blob(),
}
