import { api } from './client'
import type { LoginRequest, LoginResponse, User } from '@/types/api'

export const authApi = {
  login: (data: LoginRequest) => api.post('auth/login', { json: data }).json<LoginResponse>(),
  refresh: (token: string) => api.post('auth/refresh', { json: { refresh_token: token } }).json<LoginResponse>(),
  me: () => api.get('auth/me').json<User>(),
}
