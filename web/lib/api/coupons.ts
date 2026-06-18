import { api } from './client'
import type { Coupon, CouponValidateResponse, UUID } from '@/types/api'

export const couponsApi = {
  list: () => api.get('coupons').json<Coupon[]>(),
  getById: (id: string) => api.get(`coupons/${id}`).json<Coupon>(),
  create: (data: Partial<Coupon>) => api.post('coupons', { json: data }).json<Coupon>(),
  update: (id: string, data: Partial<Coupon>) => api.put(`coupons/${id}`, { json: data }).json<Coupon>(),
  delete: (id: string) => api.delete(`coupons/${id}`).text(),

  validate: (data: { supplier_org_id: UUID; code: string; subtotal: number }) =>
    api.post('coupons/validate', { json: data }).json<CouponValidateResponse>(),
}
