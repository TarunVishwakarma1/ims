import { api } from './client'
import type { ProductStorefront, UUID } from '@/types/api'

export type ProductStorefrontInput = Omit<ProductStorefront, 'product_id'>

export const productStorefrontApi = {
  get: (id: UUID) => api.get(`products/${id}/storefront`).json<ProductStorefront>(),
  set: (id: UUID, data: ProductStorefrontInput) =>
    api.put(`products/${id}/storefront`, { json: data }).json<ProductStorefront>(),
}
