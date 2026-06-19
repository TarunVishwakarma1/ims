'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import {
  CreditCard,
  Search,
  CircleCheck,
  CircleX,
  Clock,
  RotateCcw,
  Filter,
  Download,
} from 'lucide-react'
import { toast } from 'sonner'

import { paymentsApi } from '@/lib/api/payments'
import { formatPrice, cn } from '@/lib/utils'
import type { Payment, PaymentStatus } from '@/types/api'

import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'

const STATUS_META: Record<PaymentStatus, { label: string; cls: string; icon: typeof CircleCheck }> = {
  created:            { label: 'Pending',             cls: 'bg-amber-100 text-amber-800 border-amber-200',     icon: Clock },
  authorized:         { label: 'Authorized',          cls: 'bg-sky-100 text-sky-800 border-sky-200',           icon: Clock },
  captured:           { label: 'Captured',            cls: 'bg-emerald-100 text-emerald-800 border-emerald-200', icon: CircleCheck },
  failed:             { label: 'Failed',              cls: 'bg-rose-100 text-rose-800 border-rose-200',         icon: CircleX },
  partially_refunded: { label: 'Partial refund',      cls: 'bg-amber-100 text-amber-800 border-amber-200',     icon: RotateCcw },
  refunded:           { label: 'Refunded',            cls: 'bg-violet-100 text-violet-800 border-violet-200',   icon: RotateCcw },
}

export default function PaymentsPage() {
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<PaymentStatus | 'all'>('all')

  const { data: payments = [], isLoading } = useQuery({
    queryKey: ['payments'],
    queryFn: paymentsApi.list,
  })

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return payments.filter((p) => {
      if (statusFilter !== 'all' && p.status !== statusFilter) return false
      if (!q) return true
      return (
        p.id.toLowerCase().includes(q) ||
        p.razorpay_order_id?.toLowerCase().includes(q) ||
        p.razorpay_payment_id?.toLowerCase().includes(q) ||
        p.order_id?.toLowerCase().includes(q)
      )
    })
  }, [payments, search, statusFilter])

  const counts = useMemo(() => {
    const c: Record<string, number> = { all: payments.length, created: 0, captured: 0, failed: 0, refunded: 0 }
    payments.forEach((p) => { c[p.status] = (c[p.status] ?? 0) + 1 })
    return c
  }, [payments])

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <CreditCard className="h-6 w-6" /> Payments
          </h2>
          <p className="text-muted-foreground">
            Every payment your organization has initiated. Filter by status or search by ID.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={async () => {
            try {
              await paymentsApi.exportCsv()
              toast.success('CSV downloaded')
            } catch {
              toast.error('Export failed')
            }
          }}
        >
          <Download className="mr-2 h-4 w-4" /> Export CSV
        </Button>
      </div>

      <div className="grid gap-3 grid-cols-2 sm:grid-cols-5">
        {(['all', 'created', 'captured', 'failed', 'refunded'] as const).map((s) => (
          <button
            key={s}
            onClick={() => setStatusFilter(s as never)}
            className={cn(
              'rounded-xl border bg-white/70 dark:bg-zinc-900/40 backdrop-blur-sm p-3 text-left transition',
              'hover:border-zinc-300 hover:shadow-sm',
              statusFilter === s && 'border-indigo-400 ring-1 ring-indigo-400/40',
            )}
          >
            <div className="text-xs uppercase tracking-wide text-muted-foreground">{s}</div>
            <div className="text-2xl font-semibold mt-1">{counts[s] ?? 0}</div>
          </button>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col sm:flex-row gap-3 sm:items-center sm:justify-between">
            <div>
              <CardTitle className="text-base">Transactions</CardTitle>
              <CardDescription className="text-xs">
                {filtered.length} of {payments.length} payments
              </CardDescription>
            </div>
            <div className="flex gap-2">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
                <Input
                  className="pl-8 w-64"
                  placeholder="Search ID, order, rzp_…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as never)}>
                <SelectTrigger className="w-36">
                  <Filter className="h-3.5 w-3.5 mr-1" /> <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All statuses</SelectItem>
                  <SelectItem value="created">Pending</SelectItem>
                  <SelectItem value="captured">Captured</SelectItem>
                  <SelectItem value="failed">Failed</SelectItem>
                  <SelectItem value="refunded">Refunded</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground text-sm">Loading payments…</div>
          ) : filtered.length === 0 ? (
            <div className="py-12 text-center text-muted-foreground text-sm">No payments match these filters.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Order</TableHead>
                  <TableHead>Amount</TableHead>
                  <TableHead>Method</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((p) => <Row key={p.id} payment={p} />)}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function Row({ payment }: { payment: Payment }) {
  const meta = STATUS_META[payment.status] ?? STATUS_META.created
  const Icon = meta.icon
  return (
    <TableRow>
      <TableCell className="font-mono text-xs">
        <Link href={`/payments/${payment.id}`} className="hover:underline text-indigo-600 dark:text-indigo-400">
          {payment.id.slice(0, 8)}…
        </Link>
        {payment.is_mock && (
          <Badge variant="outline" className="ml-2 text-[10px] py-0">MOCK</Badge>
        )}
      </TableCell>
      <TableCell>
        {payment.order_id ? (
          <Link href={`/orders/${payment.order_id}`} className="text-xs hover:underline">
            {payment.order_id.slice(0, 8)}…
          </Link>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        )}
      </TableCell>
      <TableCell className="font-medium">{formatPrice(payment.amount)}</TableCell>
      <TableCell className="text-sm capitalize">{payment.method ?? '—'}</TableCell>
      <TableCell>
        <Badge variant="outline" className={cn('gap-1 font-normal', meta.cls)}>
          <Icon className="h-3 w-3" />
          {meta.label}
        </Badge>
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">
        {new Date(payment.created_at).toLocaleString()}
      </TableCell>
    </TableRow>
  )
}
