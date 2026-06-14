import { api } from './client'
import type { Category, CreateCategoryRequest, UpdateCategoryRequest } from '@/types/api'

export const categoriesApi = {
  list: () => api.get('categories').json<Category[]>(),
  getById: (id: string) => api.get(`categories/${id}`).json<Category>(),
  create: (data: CreateCategoryRequest) => api.post('categories', { json: data }).json<Category>(),
  update: (id: string, data: UpdateCategoryRequest) => api.put(`categories/${id}`, { json: data }).json<Category>(),
  delete: (id: string) => api.delete(`categories/${id}`),
}
