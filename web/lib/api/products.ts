import { api } from './client'
import type { Product, CreateProductRequest, UpdateProductRequest } from '@/types/api'

export const productsApi = {
  list: () => api.get('products').json<Product[]>(),
  getById: (id: string) => api.get(`products/${id}`).json<Product>(),
  create: (data: CreateProductRequest) => api.post('products', { json: data }).json<Product>(),
  update: (id: string, data: UpdateProductRequest) => api.put(`products/${id}`, { json: data }).json<Product>(),
  delete: (id: string) => api.delete(`products/${id}`),
}
