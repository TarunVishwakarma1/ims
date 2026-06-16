import { api } from './client'

export type ReturnStatus =
  | 'requested'
  | 'approved'
  | 'rejected'
  | 'in_transit'
  | 'received'
  | 'refunded'

export interface ReturnItem {
  id: string
  return_id: string
  order_item_id: string
  product_id: string
  quantity: number
  unit_price: number
}

export type ReturnDirection = 'incoming' | 'outgoing' | ''

export interface ReturnRequest {
  id: string
  org_id: string
  order_id: string
  requested_by: string
  status: ReturnStatus
  reason: string
  refund_amount: number
  rejection_note?: string | null
  created_at: string
  updated_at: string
  approved_at?: string | null
  received_at?: string | null
  refunded_at?: string | null
  direction?: ReturnDirection
  items?: ReturnItem[]
}

export interface CreateReturnPayload {
  reason: string
  items: { order_item_id: string; quantity: number }[]
}

export const returnsApi = {
  listAll: () => api.get('returns').json<ReturnRequest[]>(),
  listForOrder: (orderID: string) =>
    api.get(`orders/${orderID}/returns`).json<ReturnRequest[]>(),
  getById: (id: string) => api.get(`returns/${id}`).json<ReturnRequest>(),
  create: (orderID: string, body: CreateReturnPayload) =>
    api.post(`orders/${orderID}/returns`, { json: body }).json<ReturnRequest>(),
  approve: (id: string) =>
    api.post(`returns/${id}/approve`).json<{ message: string }>(),
  reject: (id: string, note: string) =>
    api.post(`returns/${id}/reject`, { json: { note } }).json<{ message: string }>(),
  markReceived: (id: string) =>
    api.post(`returns/${id}/received`).json<{ message: string }>(),
}
