'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { PackageCheck, Loader2, Truck, CheckCircle2 } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { shopOrdersApi } from '@/lib/api/shop-orders'
import { formatPrice, cn } from '@/lib/utils'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { AdminShopOrder, ShopOrderStatus } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'

const STATUS_FILTERS = ['all', 'pending', 'confirmed', 'shipped', 'delivered', 'cancelled'] as const

function statusBadge(status: ShopOrderStatus): string {
  switch (status) {
    case 'delivered': return 'bg-emerald-100 text-emerald-800 border-emerald-200'
    case 'shipped': return 'bg-sky-100 text-sky-800 border-sky-200'
    case 'confirmed': return 'bg-indigo-100 text-indigo-800 border-indigo-200'
    case 'cancelled':
    case 'cancelling': return 'bg-rose-100 text-rose-800 border-rose-200'
    default: return 'bg-amber-100 text-amber-800 border-amber-200'
  }
}

// Next admin action per status — mirrors the backend state machine.
function nextAction(status: ShopOrderStatus): { to: ShopOrderStatus; label: string; Icon: typeof Truck } | null {
  if (status === 'confirmed') return { to: 'shipped', label: 'Mark shipped', Icon: Truck }
  if (status === 'shipped') return { to: 'delivered', label: 'Mark delivered', Icon: CheckCircle2 }
  return null
}

export default function ShopOrdersPage() {
  const qc = useQueryClient()
  const { user } = useAuthStore()
  const canManage = hasPermission(user?.role, PERMISSIONS.ORDERS_MANAGE)
  const [filter, setFilter] = useState<string>('all')

  const { data: orders = [], isLoading } = useQuery({
    queryKey: ['shop-orders', filter],
    queryFn: () => shopOrdersApi.list(filter === 'all' ? undefined : filter),
  })

  const advanceMut = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      shopOrdersApi.updateStatus(id, status),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ['shop-orders'] })
      toast.success(v.status === 'shipped' ? 'Marked shipped — customer notified' : 'Marked delivered — customer notified')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error === 'invalid_transition' ? 'That status change isn’t allowed' : data.error || 'Update failed')
      } else toast.error(err.message)
    },
  })

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <PackageCheck className="h-6 w-6" /> Shop orders
          </h2>
          <p className="text-muted-foreground">
            Customer (B2C) orders. Advance status to ship and deliver — the customer is
            emailed at each step.
          </p>
        </div>
        <Select value={filter} onValueChange={setFilter}>
          <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
          <SelectContent>
            {STATUS_FILTERS.map((s) => (
              <SelectItem key={s} value={s} className="capitalize">{s}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Orders</CardTitle>
          <CardDescription>{orders.length} orders</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground text-sm">Loading…</div>
          ) : orders.length === 0 ? (
            <div className="py-12 text-center text-muted-foreground text-sm">No orders.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Invoice</TableHead>
                  <TableHead>Customer</TableHead>
                  <TableHead>Items</TableHead>
                  <TableHead>Total</TableHead>
                  <TableHead>Payment</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Placed</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((o: AdminShopOrder) => {
                  const action = nextAction(o.status)
                  return (
                    <TableRow key={o.id}>
                      <TableCell className="font-mono text-xs">{o.invoice_number || o.id.slice(0, 8)}</TableCell>
                      <TableCell>
                        <div className="text-sm">{o.customer_name || '—'}</div>
                        <div className="text-xs text-muted-foreground">{o.customer_phone}</div>
                      </TableCell>
                      <TableCell>{o.item_count}</TableCell>
                      <TableCell>{formatPrice(o.total_paise)}</TableCell>
                      <TableCell className="text-xs">
                        <span className="capitalize">{o.payment_method || '—'}</span>
                        <span className="text-muted-foreground"> · {o.payment_status}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className={cn('font-normal capitalize', statusBadge(o.status))}>
                          {o.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs">{new Date(o.created_at).toLocaleDateString()}</TableCell>
                      <TableCell className="text-right">
                        {canManage && action ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={advanceMut.isPending}
                            onClick={() => advanceMut.mutate({ id: o.id, status: action.to })}
                          >
                            {advanceMut.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <action.Icon className="mr-2 h-4 w-4" />}
                            {action.label}
                          </Button>
                        ) : (
                          <span className="text-xs text-muted-foreground">
                            {o.status === 'pending' ? 'Awaiting payment' : '—'}
                          </span>
                        )}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
