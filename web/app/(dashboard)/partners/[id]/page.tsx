'use client'

import { use } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Loader2, ArrowDownLeft, ArrowUpRight } from 'lucide-react'

import { partnersApi, type Partner } from '@/lib/api/partners'
import { formatPrice } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function statusBadge(s: string) {
  const map: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    accepted: 'bg-blue-100 text-blue-800 border-blue-200',
    processing: 'bg-indigo-100 text-indigo-800 border-indigo-200',
    ready: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    shipped: 'bg-purple-100 text-purple-800 border-purple-200',
    delivered: 'bg-green-100 text-green-800 border-green-200',
    completed: 'bg-green-600 text-white border-green-600',
    cancelled: 'bg-red-100 text-red-800 border-red-200',
    refunded: 'bg-gray-100 text-gray-800 border-gray-200',
  }
  return <Badge variant="outline" className={map[s] || ''}>{s}</Badge>
}

function RelationshipCard({ p, icon, label }: { p: Partner; icon: React.ReactNode; label: string }) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <CardTitle className="flex items-center gap-2 text-base">
          {icon} {label}
        </CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-3 gap-4 text-sm">
        <div>
          <div className="text-muted-foreground text-xs">Orders</div>
          <div className="text-lg font-semibold">{p.order_count}</div>
        </div>
        <div>
          <div className="text-muted-foreground text-xs">Total value</div>
          <div className="text-lg font-semibold">{formatPrice(p.total_value)}</div>
        </div>
        <div>
          <div className="text-muted-foreground text-xs">Active</div>
          <div className="text-lg font-semibold">{p.pending_count}</div>
        </div>
      </CardContent>
    </Card>
  )
}

export default function PartnerDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['partner', id],
    queryFn: () => partnersApi.getById(id),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => router.push('/partners')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> Back
        </Button>
        <p className="text-muted-foreground">Partner not found.</p>
      </div>
    )
  }

  const { organization: org, as_supplier, as_buyer, recent_orders } = data

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => router.push('/partners')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{org.name}</h2>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span>{org.slug}</span>
            {!org.is_active && <Badge variant="destructive">Inactive</Badge>}
            <Badge variant="outline">{org.plan_type}</Badge>
          </div>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        {as_supplier && (
          <RelationshipCard
            p={as_supplier}
            icon={<ArrowDownLeft className="h-4 w-4 text-blue-600" />}
            label="As supplier (sold to us)"
          />
        )}
        {as_buyer && (
          <RelationshipCard
            p={as_buyer}
            icon={<ArrowUpRight className="h-4 w-4 text-emerald-600" />}
            label="As buyer (bought from us)"
          />
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Recent orders</CardTitle>
        </CardHeader>
        <CardContent>
          {recent_orders.length === 0 ? (
            <p className="text-sm text-muted-foreground">No orders yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Order</TableHead>
                  <TableHead>Counterparty</TableHead>
                  <TableHead>Direction</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Payment</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead>Date</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recent_orders.map((o) => {
                  // Direction relative to viewer: if THEY are supplier and
                  // viewer org is buyer on this row, viewer is the buyer.
                  const direction = o.supplier_org_id === id ? 'inbound' : 'outbound'
                  return (
                    <TableRow
                      key={o.id}
                      className="cursor-pointer hover:bg-muted/50"
                      onClick={() => router.push(`/orders/${o.id}`)}
                    >
                      <TableCell className="font-mono text-xs">{o.id.split('-')[0]}</TableCell>
                      <TableCell className="text-sm">
                        {direction === 'inbound'
                          ? (o.buyer_org_name || o.buyer_org_id?.split('-')[0] || '—')
                          : (o.supplier_org_name || o.supplier_org_id?.split('-')[0] || '—')}
                      </TableCell>
                      <TableCell>
                        {direction === 'inbound'
                          ? <Badge variant="outline" className="bg-blue-50 border-blue-200 text-blue-700">In</Badge>
                          : <Badge variant="outline" className="bg-emerald-50 border-emerald-200 text-emerald-700">Out</Badge>
                        }
                      </TableCell>
                      <TableCell>{statusBadge(o.status)}</TableCell>
                      <TableCell>{o.payment_status || '—'}</TableCell>
                      <TableCell className="text-right">{formatPrice(o.total_amount)}</TableCell>
                      <TableCell>{new Date(o.created_at).toLocaleDateString()}</TableCell>
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
