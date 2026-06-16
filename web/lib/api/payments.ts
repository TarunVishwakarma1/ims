import { api } from './client'
import type { Payment, CreatePaymentOrderResponse, UUID } from '@/types/api'

export interface PaymentConfig {
  mock: boolean
  live: boolean
  key_id: string
}

export const paymentsApi = {
  getConfig: () => api.get('payments/config').json<PaymentConfig>(),

  createOrder: (data: { order_id: UUID; amount: number }) =>
    api.post('payments/orders', { json: data }).json<CreatePaymentOrderResponse>(),

  list: () => api.get('payments').json<Payment[]>(),

  getById: (id: string) => api.get(`payments/${id}`).json<Payment>(),

  refund: (id: string, amount?: number, reason?: string) =>
    api
      .post(`payments/${id}/refund`, { json: { amount: amount ?? 0, reason: reason || '' } })
      .json<{ message: string }>(),

  // Mock-only endpoints — dev/test
  mockCapture: (razorpayOrderID: string) =>
    api
      .post('payments/mock/capture', { json: { razorpay_order_id: razorpayOrderID } })
      .json<{ status: string }>(),

  mockFail: (razorpayOrderID: string, reason?: string) =>
    api
      .post('payments/mock/fail', { json: { razorpay_order_id: razorpayOrderID, reason } })
      .json<{ status: string }>(),

  // DLQ admin
  listDLQ: () => api.get('payments/webhooks/dlq').json<DLQEvent[]>(),
  replayDLQ: (id: string) =>
    api.post(`payments/webhooks/dlq/${id}/replay`).json<{ message: string }>(),
}

export interface DLQEvent {
  id: string
  provider: string
  event_id: string
  event_type: string
  status: string
  error?: string | null
  created_at: string
  processed_at?: string | null
}
