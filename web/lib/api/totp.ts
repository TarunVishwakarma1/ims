import { api } from './client'

export const totpApi = {
  enroll: () =>
    api.post('auth/2fa/enroll').json<{ uri: string; secret: string }>(),
  confirm: (code: string) =>
    api.post('auth/2fa/confirm', { json: { code } }).json<{ backup_codes: string[] }>(),
  disable: () =>
    api.post('auth/2fa/disable').json<{ message: string }>(),
  verifyLogin: (pendingToken: string, code: string) =>
    api.post('auth/login/verify-2fa', {
      json: { pending_token: pendingToken, code },
    }).json<unknown>(),
}
