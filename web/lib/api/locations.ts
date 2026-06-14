import { api } from './client'
import type { OrgLocation, CreateLocationRequest } from '@/types/api'

export const locationsApi = {
  list: () => api.get('locations').json<OrgLocation[]>(),
  create: (data: CreateLocationRequest) => api.post('locations', { json: data }).json<OrgLocation>(),
  update: (id: string, data: Partial<CreateLocationRequest>) => api.put(`locations/${id}`, { json: data }).json<OrgLocation>(),
  delete: (id: string) => api.delete(`locations/${id}`),
}
