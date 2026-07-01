import { api } from './client'
import type { SalesSummary } from '@/types/api'

export const analyticsApi = {
  // Org-scoped sales summary over the last `days` (backend clamps 1–365).
  sales: (days = 30) =>
    api.get('admin/shop/analytics', { searchParams: { days } }).json<SalesSummary>(),
}
