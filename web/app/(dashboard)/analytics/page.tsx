'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { BarChart3, Loader2 } from 'lucide-react'

import { analyticsApi } from '@/lib/api/analytics'
import { formatPrice } from '@/lib/utils'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

const RANGES = [7, 30, 90] as const

export default function AnalyticsPage() {
  const { user } = useAuthStore()
  const canView = hasPermission(user?.role, PERMISSIONS.ORDERS_VIEW)
  const [days, setDays] = useState<number>(30)

  const { data, isLoading } = useQuery({
    queryKey: ['shop-analytics', days],
    queryFn: () => analyticsApi.sales(days),
    enabled: canView,
  })

  if (!canView) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <p className="text-sm text-muted-foreground">
          You don&apos;t have permission to view analytics.
        </p>
      </div>
    )
  }

  const maxRevenue = Math.max(1, ...(data?.by_day ?? []).map((d) => d.revenue_paise))

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <BarChart3 className="h-5 w-5 text-primary" />
          <h1 className="text-2xl font-bold tracking-tight">Sales</h1>
        </div>
        <div className="flex gap-1">
          {RANGES.map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setDays(r)}
              className={`h-8 rounded-md border px-3 text-sm ${
                days === r ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
              }`}
            >
              {r}d
            </button>
          ))}
        </div>
      </div>

      {isLoading || !data ? (
        <div className="p-8"><Loader2 className="animate-spin" /></div>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm font-medium">Orders</CardTitle></CardHeader>
              <CardContent><div className="text-2xl font-bold">{data.orders}</div></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm font-medium">Revenue</CardTitle></CardHeader>
              <CardContent><div className="text-2xl font-bold">{formatPrice(data.revenue_paise)}</div></CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2"><CardTitle className="text-sm font-medium">Avg order</CardTitle></CardHeader>
              <CardContent><div className="text-2xl font-bold">{formatPrice(data.avg_order_paise)}</div></CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>By day</CardTitle>
              <CardDescription>Non-cancelled orders over the last {data.days} days.</CardDescription>
            </CardHeader>
            <CardContent>
              {data.by_day.length === 0 ? (
                <p className="text-sm text-muted-foreground">No orders in this period.</p>
              ) : (
                <ul className="space-y-1.5">
                  {data.by_day.map((d) => (
                    <li key={d.date} className="flex items-center gap-3 text-sm">
                      <span className="w-24 shrink-0 text-muted-foreground">{d.date}</span>
                      <span className="relative h-5 flex-1 overflow-hidden rounded bg-muted">
                        <span
                          className="absolute inset-y-0 left-0 rounded bg-primary/70"
                          style={{ width: `${(d.revenue_paise / maxRevenue) * 100}%` }}
                        />
                      </span>
                      <span className="w-28 shrink-0 text-right tabular-nums">{formatPrice(d.revenue_paise)}</span>
                      <span className="w-14 shrink-0 text-right text-muted-foreground tabular-nums">{d.orders} ord</span>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
