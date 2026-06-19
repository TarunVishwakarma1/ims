import { api } from './client'

export interface AuditEntry {
  id: string
  org_id: string
  user_id?: string | null
  action: string
  entity: string
  entity_id: string
  ip_address: string
  created_at: string
}

export interface AuditFilters {
  entity?: string
  action_prefix?: string
  user_id?: string
  from?: string
  to?: string
  page?: number
  per_page?: number
}

export interface AuditListResult {
  items: AuditEntry[]
  total: number
  page: number
  per_page: number
}

function toQuery(f: AuditFilters): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === '' || v === null) continue
    out[k] = String(v)
  }
  return out
}

export const auditApi = {
  list: (f: AuditFilters = {}) =>
    api.get('audit', { searchParams: toQuery(f) }).json<AuditListResult>(),
}
