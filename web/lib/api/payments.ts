import { api } from './client'
import type { Payment, CreatePaymentOrderResponse, Refund, UUID } from '@/types/api'

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

  getReceipt: (id: string) => api.get(`payments/${id}/receipt`).json<Receipt>(),

  listRefunds: (paymentId: string) =>
    api.get(`payments/${paymentId}/refunds`).json<Refund[]>(),

  // Triggers a CSV download via ky's blob() so the browser save dialog fires.
  // ky carries auth headers; cookies-only setups would also work via fetch
  // with credentials, but we standardize on ky for header parity.
  exportCsv: async (params?: { from?: string; to?: string }) => {
    const blob = await api
      .get('payments/export.csv', { searchParams: params || {} })
      .blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `payments_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },

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
  listEvents: (status?: string) =>
    api.get('payments/webhooks/events', { searchParams: status ? { status } : {} }).json<DLQEvent[]>(),
  webhookSecretStatus: () =>
    api.get('payments/webhooks/secrets').json<{
      primary_set: boolean
      prev_set: boolean
      live_mode: boolean
      mock_mode: boolean
      key_id: string
    }>(),
  getDLQEvent: (id: string) => api.get(`payments/webhooks/dlq/${id}`).json<DLQEventFull>(),
  replayDLQ: (id: string) =>
    api.post(`payments/webhooks/dlq/${id}/replay`).json<{ message: string }>(),
}

export interface DLQEventFull extends DLQEvent {
  payload?: unknown
  signature?: string | null
}

export interface ReceiptLine {
  Name: string
  Qty: number
  Total: string
}

export interface Receipt {
  payment_id: string
  order_id: string
  razorpay_payment_id: string
  method: string
  amount: number
  currency: string
  captured_at: string
  buyer_name: string
  buyer_email: string
  org_name: string
  items: ReceiptLine[]
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
