import { api } from './client'
import type { User, CreateUserRequest, UpdateUserRequest } from '@/types/api'

export const usersApi = {
  list: () => api.get('users').json<User[]>(),
  getById: (id: string) => api.get(`users/${id}`).json<User>(),
  create: (data: CreateUserRequest) => api.post('users', { json: data }).json<User>(),
  update: (id: string, data: UpdateUserRequest) => api.put(`users/${id}`, { json: data }).json<User>(),
  delete: (id: string) => api.delete(`users/${id}`),
}
