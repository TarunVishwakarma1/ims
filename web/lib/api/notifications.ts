import { api } from './client'

export type NotificationStatus = 'pending' | 'sent' | 'failed' | 'dlq'

export interface Notification {
  id: string
  channel: string
  recipient: string
  subject: string
  body_text: string
  body_html?: string | null
  status: NotificationStatus
  attempts: number
  last_error?: string | null
  next_attempt: string
  created_at: string
  sent_at?: string | null
}

export const notificationsApi = {
  list: (status: NotificationStatus = 'dlq') =>
    api.get('notifications', { searchParams: { status } }).json<Notification[]>(),
  getById: (id: string) => api.get(`notifications/${id}`).json<Notification>(),
  resend: (id: string) =>
    api.post(`notifications/${id}/resend`).json<{ message: string }>(),
}
