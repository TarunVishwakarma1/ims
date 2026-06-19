import { api } from './client'
import type { LoginRequest, LoginResponse, User, SignupRequest } from '@/types/api'

export const authApi = {
  login: (data: LoginRequest) =>
    api.post('auth/login', { json: data }).json<LoginResponse>(),

  refresh: (token: string) =>
    api.post('auth/refresh', { json: { refresh_token: token } }).json<LoginResponse>(),

  me: () => api.get('auth/me').json<User>(),

  register: (data: SignupRequest) =>
    api.post('auth/register', { json: data }).json<LoginResponse>(),

  logout: (refreshToken: string) =>
    api.post('auth/logout', { json: { refresh_token: refreshToken } }).json<{ message: string }>(),

  verifyEmail: (otp: string) =>
    api.post('auth/verify-email', { json: { otp } }).json<{ message: string }>(),

  resendVerification: () =>
    api.post('auth/resend-verification').json<{ message: string }>(),

  requestPasswordReset: (email: string) =>
    api.post('auth/password-reset/request', { json: { email } }).json<{ message: string }>(),

  confirmPasswordReset: (token: string, password: string) =>
    api.post('auth/password-reset/confirm', { json: { token, password } }).json<{ message: string }>(),
}
