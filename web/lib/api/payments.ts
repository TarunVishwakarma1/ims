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

  // Mock-only endpoints — dev/test
  mockCapture: (razorpayOrderID: string) =>
    api
      .post('payments/mock/capture', { json: { razorpay_order_id: razorpayOrderID } })
      .json<{ status: string }>(),

  mockFail: (razorpayOrderID: string, reason?: string) =>
    api
      .post('payments/mock/fail', { json: { razorpay_order_id: razorpayOrderID, reason } })
      .json<{ status: string }>(),
}
