'use client'

import { use, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { HTTPError } from 'ky'
import {
  ArrowLeft,
  Loader2,
  XCircle,
  RotateCcw,
  Edit,
  CheckCircle2,
  Clock,
  Truck,
  Package,
  Ban,
  CreditCard,
  RefreshCw,
  Undo2,
  FileDown,
} from 'lucide-react'

import { ordersApi } from '@/lib/api/orders'
import { paymentsApi } from '@/lib/api/payments'
import { returnsApi } from '@/lib/api/returns'
import { formatPrice } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import { useEventStream } from '@/hooks/useEventStream'
import type { OrderStatus } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// Map status → allowed transitions (mirrors backend state machine).
const allowedTransitions = (s: OrderStatus): OrderStatus[] => {
  switch (s) {
    case 'pending': return ['accepted', 'rejected', 'cancelled']
    case 'accepted': return ['processing', 'cancelled']
    case 'processing': return ['ready', 'cancelled']
    case 'ready': return ['shipped', 'cancelled']
    case 'shipped': return ['delivered']
    case 'delivered': return ['completed']
    case 'confirmed': return ['accepted', 'cancelled']
    default: return []
  }
}

const cancellable = (s: OrderStatus) =>
  ['pending', 'confirmed', 'accepted', 'processing', 'ready'].includes(s)

// Single visual lookup for the timeline icons by audit action string.
function iconFor(action: string) {
  if (action.includes('created')) return <Package className="h-4 w-4" />
  if (action.includes('cancelled')) return <Ban className="h-4 w-4" />
  if (action.includes('status_updated')) return <RefreshCw className="h-4 w-4" />
  if (action.includes('payment') || action.includes('paid')) return <CreditCard className="h-4 w-4" />
  if (action.includes('shipped')) return <Truck className="h-4 w-4" />
  if (action.includes('delivered') || action.includes('completed')) return <CheckCircle2 className="h-4 w-4" />
  return <Clock className="h-4 w-4" />
}

// prettyAction renders audit-log actions for the timeline. Backend writes
// strings like "order.status_updated:shipped" or "payment.captured" — strip
// the prefix and replace separators for human display.
function prettyAction(a: string): string {
  let s = a.replace(/^order\./, '')
  if (s.startsWith('status_updated:')) {
    return 'Status → ' + s.slice('status_updated:'.length)
  }
  s = s.replace(/^payment\./, 'payment ')
  return s.replace(/_/g, ' ')
}

function statusBadge(s: OrderStatus) {
  const map: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    accepted: 'bg-blue-100 text-blue-800 border-blue-200',
    processing: 'bg-indigo-100 text-indigo-800 border-indigo-200',
    ready: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    shipped: 'bg-purple-100 text-purple-800 border-purple-200',
    delivered: 'bg-green-100 text-green-800 border-green-200',
    completed: 'bg-green-600 text-white border-green-600',
    confirmed: 'bg-emerald-100 text-emerald-800 border-emerald-200',
    cancelled: 'bg-red-100 text-red-800 border-red-200',
    rejected: 'bg-red-100 text-red-800 border-red-200',
    refunded: 'bg-gray-100 text-gray-800 border-gray-200',
  }
  return <Badge variant="outline" className={map[s] || ''}>{s}</Badge>
}

function paymentBadge(p: string) {
  const map: Record<string, string> = {
    paid: 'bg-emerald-600 text-white border-emerald-600',
    unpaid: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    partial: 'bg-orange-100 text-orange-800 border-orange-200',
    refunded: 'bg-zinc-100 text-zinc-700 border-zinc-200',
  }
  return <Badge variant="outline" className={map[p] || ''}>{p || '—'}</Badge>
}

export default function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const qc = useQueryClient()
  const { can } = usePermission()

  const [cancelOpen, setCancelOpen] = useState(false)
  const [refundOpen, setRefundOpen] = useState(false)
  const [statusOpen, setStatusOpen] = useState(false)
  const [returnOpen, setReturnOpen] = useState(false)
  const [cancelReason, setCancelReason] = useState('')
  const [refundReason, setRefundReason] = useState('')
  const [returnReason, setReturnReason] = useState('')
  const [returnQtys, setReturnQtys] = useState<Record<string, number>>({})
  const [newStatus, setNewStatus] = useState<OrderStatus | ''>('')

  // Refresh on relevant SSE events for THIS order. Backend embeds the
  // order id under `data.id` (order events) or `data.order_id` (payment
  // events). Fall back to refreshing on any payment event since they're
  // less frequent.
  useEventStream(['order', 'payment'], (evt) => {
    const data = evt.data as { id?: string; order_id?: string } | undefined
    const targetID = data?.id ?? data?.order_id
    if (targetID === id || evt.type.startsWith('payment.')) {
      qc.invalidateQueries({ queryKey: ['order', id] })
      qc.invalidateQueries({ queryKey: ['order', id, 'timeline'] })
      qc.invalidateQueries({ queryKey: ['order', id, 'items'] })
      qc.invalidateQueries({ queryKey: ['payments'] })
    }
  })

  const orderQ = useQuery({ queryKey: ['order', id], queryFn: () => ordersApi.getById(id) })
  const itemsQ = useQuery({ queryKey: ['order', id, 'items'], queryFn: () => ordersApi.getItems(id) })
  const timelineQ = useQuery({
    queryKey: ['order', id, 'timeline'],
    queryFn: () => ordersApi.getTimeline(id),
  })
  const paymentsQ = useQuery({ queryKey: ['payments'], queryFn: paymentsApi.list })
  const returnsQ = useQuery({
    queryKey: ['order', id, 'returns'],
    queryFn: () => returnsApi.listForOrder(id),
  })
  // Cancel-preview only fetched when dialog opens — avoids extra requests
  // on every page load.
  const cancelPreviewQ = useQuery({
    queryKey: ['order', id, 'cancel-preview'],
    queryFn: () => ordersApi.getCancelPreview(id),
    enabled: cancelOpen,
  })

  const order = orderQ.data
  const items = itemsQ.data ?? []
  const timeline = timelineQ.data ?? []
  const payment = paymentsQ.data?.find(p => p.order_id === id)
  const returns = returnsQ.data ?? []

  const cancelMut = useMutation({
    mutationFn: () => ordersApi.cancel(id, cancelReason),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['order', id] })
      qc.invalidateQueries({ queryKey: ['order', id, 'timeline'] })
      qc.invalidateQueries({ queryKey: ['orders'] })
      setCancelOpen(false)
      setCancelReason('')
      toast.success('Order cancelled')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Cancel failed')
      } else toast.error(err.message)
    },
  })

  const refundMut = useMutation({
    mutationFn: () => {
      if (!payment) throw new Error('no captured payment for this order')
      return paymentsApi.refund(payment.id, 0, refundReason)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['payments'] })
      qc.invalidateQueries({ queryKey: ['order', id] })
      setRefundOpen(false)
      setRefundReason('')
      toast.success('Refund submitted')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Refund failed')
      } else toast.error(err.message)
    },
  })

  const statusMut = useMutation({
    mutationFn: (s: OrderStatus) => ordersApi.updateStatus(id, { status: s }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['order', id] })
      qc.invalidateQueries({ queryKey: ['order', id, 'timeline'] })
      qc.invalidateQueries({ queryKey: ['orders'] })
      setStatusOpen(false)
      setNewStatus('')
      toast.success('Status updated')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Status update failed')
      } else toast.error(err.message)
    },
  })

  const returnMut = useMutation({
    mutationFn: () => {
      const picked = Object.entries(returnQtys)
        .filter(([, q]) => q > 0)
        .map(([order_item_id, quantity]) => ({ order_item_id, quantity }))
      if (picked.length === 0) throw new Error('select at least one item to return')
      return returnsApi.create(id, { reason: returnReason, items: picked })
    },
    onSuccess: (rr) => {
      qc.invalidateQueries({ queryKey: ['order', id, 'returns'] })
      qc.invalidateQueries({ queryKey: ['returns'] })
      setReturnOpen(false)
      setReturnReason('')
      setReturnQtys({})
      toast.success('Return request submitted')
      router.push(`/returns/${rr.id}`)
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Return request failed')
      } else toast.error(err.message)
    },
  })

  if (orderQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (orderQ.isError || !order) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => router.push('/orders')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> Back
        </Button>
        <p className="text-muted-foreground">Order not found.</p>
      </div>
    )
  }

  const transitions = allowedTransitions(order.status)
  const subtotal = items.reduce((acc, it) => acc + it.quantity * it.unit_price, 0)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push('/orders')}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h2 className="text-2xl font-bold tracking-tight">
              Order <span className="font-mono text-base text-muted-foreground">#{order.id.split('-')[0]}</span>
            </h2>
            <p className="text-sm text-muted-foreground">
              Placed {new Date(order.created_at).toLocaleString()}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={async () => {
              try {
                const blob = await ordersApi.invoicePdf(id)
                const url = URL.createObjectURL(blob)
                const a = document.createElement('a')
                a.href = url
                a.download = `invoice-${id.split('-')[0]}.pdf`
                a.click()
                URL.revokeObjectURL(url)
              } catch (e) {
                toast.error(e instanceof Error ? e.message : 'Invoice download failed')
              }
            }}
          >
            <FileDown className="mr-2 h-4 w-4" /> Invoice PDF
          </Button>
          {can(PERMISSIONS.ORDERS_MANAGE) && transitions.length > 0 && (
            <Button variant="outline" onClick={() => { setNewStatus(''); setStatusOpen(true) }}>
              <Edit className="mr-2 h-4 w-4" /> Change status
            </Button>
          )}
          {can(PERMISSIONS.ORDERS_MANAGE) && cancellable(order.status) && (
            <Button variant="outline" className="text-red-600" onClick={() => setCancelOpen(true)}>
              <XCircle className="mr-2 h-4 w-4" /> Cancel
            </Button>
          )}
          {can(PERMISSIONS.ORDERS_MANAGE) && order.payment_status === 'paid' && payment?.status === 'captured' && (
            <Button variant="outline" className="text-amber-600" onClick={() => setRefundOpen(true)}>
              <RotateCcw className="mr-2 h-4 w-4" /> Refund
            </Button>
          )}
          {(order.status === 'delivered' || order.status === 'completed') && (
            <Button
              variant="outline"
              onClick={() => {
                // pre-fill all items at full ordered quantity
                const seed: Record<string, number> = {}
                for (const it of items) seed[it.id] = it.quantity
                setReturnQtys(seed)
                setReturnReason('')
                setReturnOpen(true)
              }}
            >
              <Undo2 className="mr-2 h-4 w-4" /> Request return
            </Button>
          )}
        </div>
      </div>

      {/* Existing returns for this order — quick links */}
      {returns.length > 0 && (
        <div className="rounded-md border bg-amber-50 dark:bg-amber-950/20 border-amber-200 px-4 py-3 text-sm">
          <div className="font-medium mb-1">Return requests on this order</div>
          <div className="flex flex-wrap gap-2">
            {returns.map((r) => (
              <button
                key={r.id}
                onClick={() => router.push(`/returns/${r.id}`)}
                className="underline font-mono text-xs"
              >
                #{r.id.split('-')[0]} ({r.status})
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Left col — order info + items */}
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Summary</CardTitle>
              <div className="flex gap-2">
                {statusBadge(order.status)}
                {paymentBadge(order.payment_status)}
              </div>
            </CardHeader>
            <CardContent className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <div className="text-muted-foreground">Order type</div>
                <div className="font-medium">{order.order_type || '—'}</div>
              </div>
              <div>
                <div className="text-muted-foreground">User</div>
                <div className="font-mono text-xs">{order.user_id.split('-')[0]}</div>
              </div>
              {order.buyer_org_id && (
                <div>
                  <div className="text-muted-foreground">Buyer org</div>
                  <div className="font-mono text-xs">{order.buyer_org_id.split('-')[0]}</div>
                </div>
              )}
              {order.supplier_org_id && (
                <div>
                  <div className="text-muted-foreground">Supplier org</div>
                  <div className="font-mono text-xs">{order.supplier_org_id.split('-')[0]}</div>
                </div>
              )}
              <div>
                <div className="text-muted-foreground">Last updated</div>
                <div>{new Date(order.updated_at).toLocaleString()}</div>
              </div>
              {order.cancelled_at && (
                <div>
                  <div className="text-muted-foreground">Cancelled</div>
                  <div>{new Date(order.cancelled_at).toLocaleString()}</div>
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Items</CardTitle>
            </CardHeader>
            <CardContent>
              {itemsQ.isLoading ? (
                <div className="py-6 flex justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>
              ) : items.length === 0 ? (
                <p className="text-sm text-muted-foreground py-4">No items.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Product</TableHead>
                      <TableHead className="text-right">Qty</TableHead>
                      <TableHead className="text-right">Unit price</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map(it => (
                      <TableRow key={it.id}>
                        <TableCell>{it.product_name || it.product_id.split('-')[0]}</TableCell>
                        <TableCell className="text-right">{it.quantity}</TableCell>
                        <TableCell className="text-right">{formatPrice(it.unit_price)}</TableCell>
                        <TableCell className="text-right font-medium">{formatPrice(it.quantity * it.unit_price)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}

              <Separator className="my-4" />
              <div className="space-y-1 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Subtotal</span>
                  <span>{formatPrice(order.subtotal || subtotal)}</span>
                </div>
                {(order.delivery_fee ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Delivery</span>
                    <span>{formatPrice(order.delivery_fee || 0)}</span>
                  </div>
                )}
                {(order.discount ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Discount</span>
                    <span>−{formatPrice(order.discount || 0)}</span>
                  </div>
                )}
                <div className="flex justify-between pt-2 text-base font-semibold border-t mt-2">
                  <span>Total</span>
                  <span>{formatPrice(order.total_amount)}</span>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Right col — timeline + payment */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Payment</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              {!payment ? (
                <p className="text-muted-foreground">No payment recorded.</p>
              ) : (
                <>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Status</span>
                    <Badge variant="outline">{payment.status}</Badge>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Amount</span>
                    <span>{formatPrice(payment.amount)}</span>
                  </div>
                  {payment.method && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Method</span>
                      <span className="capitalize">{payment.method}</span>
                    </div>
                  )}
                  {payment.razorpay_payment_id && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">RazorPay ID</span>
                      <span className="font-mono text-xs">{payment.razorpay_payment_id}</span>
                    </div>
                  )}
                  {payment.is_mock && (
                    <Badge variant="secondary" className="mt-2">Mock payment</Badge>
                  )}
                </>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Timeline</CardTitle>
            </CardHeader>
            <CardContent>
              {timelineQ.isLoading ? (
                <div className="py-6 flex justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>
              ) : timeline.length === 0 ? (
                <p className="text-sm text-muted-foreground">No events.</p>
              ) : (
                <ol className="relative border-l border-border ml-3 space-y-4">
                  {timeline.map((evt) => (
                    <li key={evt.id} className="ml-4">
                      <span className="absolute -left-3 flex h-6 w-6 items-center justify-center rounded-full bg-background border text-muted-foreground">
                        {iconFor(evt.action)}
                      </span>
                      <div className="text-sm font-medium">{prettyAction(evt.action)}</div>
                      <div className="text-xs text-muted-foreground">
                        {new Date(evt.created_at).toLocaleString()}
                      </div>
                    </li>
                  ))}
                </ol>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Cancel dialog */}
      <Dialog open={cancelOpen} onOpenChange={setCancelOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Cancel order?</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              Reserved stock will be returned. This cannot be undone.
              {cancelPreviewQ.isLoading ? (
                <span className="block mt-2 text-xs text-muted-foreground">
                  <Loader2 className="inline h-3 w-3 mr-1 animate-spin" /> Computing refund…
                </span>
              ) : cancelPreviewQ.data && cancelPreviewQ.data.payment_paid ? (
                <span className="block mt-2 font-medium text-amber-700 dark:text-amber-400">
                  {cancelPreviewQ.data.refund_amount > 0
                    ? `Refund: ${formatPrice(cancelPreviewQ.data.refund_amount)} (${cancelPreviewQ.data.refund_percent}%). ${cancelPreviewQ.data.reason}`
                    : cancelPreviewQ.data.reason}
                </span>
              ) : null}
            </p>
            <div>
              <Label htmlFor="cancel-reason" className="text-xs">Reason (optional)</Label>
              <Input
                id="cancel-reason"
                value={cancelReason}
                onChange={(e) => setCancelReason(e.target.value)}
                placeholder="e.g. customer request"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCancelOpen(false)}>Keep order</Button>
            <Button
              variant="destructive"
              disabled={cancelMut.isPending}
              onClick={() => cancelMut.mutate()}
            >
              {cancelMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Cancel order
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Refund dialog */}
      <Dialog open={refundOpen} onOpenChange={setRefundOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Refund payment?</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              Full refund of {payment ? formatPrice(payment.amount) : '—'} will be issued via RazorPay.
              Order status remains; payment status will flip to refunded once RazorPay confirms.
            </p>
            <div>
              <Label htmlFor="refund-reason" className="text-xs">Reason (optional)</Label>
              <Input
                id="refund-reason"
                value={refundReason}
                onChange={(e) => setRefundReason(e.target.value)}
                placeholder="e.g. duplicate order"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRefundOpen(false)}>Cancel</Button>
            <Button
              disabled={refundMut.isPending}
              onClick={() => refundMut.mutate()}
            >
              {refundMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Issue refund
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Status dialog */}
      <Dialog open={statusOpen} onOpenChange={setStatusOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Change order status</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <Label className="text-xs">New status</Label>
            <Select value={newStatus} onValueChange={(v) => setNewStatus(v as OrderStatus)}>
              <SelectTrigger>
                <SelectValue placeholder={`Current: ${order.status}`} />
              </SelectTrigger>
              <SelectContent>
                {transitions.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStatusOpen(false)}>Cancel</Button>
            <Button
              disabled={!newStatus || statusMut.isPending}
              onClick={() => newStatus && statusMut.mutate(newStatus)}
            >
              {statusMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Return-request dialog */}
      <Dialog open={returnOpen} onOpenChange={setReturnOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Request return</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <Label htmlFor="return-reason" className="text-xs">Reason (required)</Label>
              <Input
                id="return-reason"
                value={returnReason}
                onChange={(e) => setReturnReason(e.target.value)}
                placeholder="e.g. wrong item / defective / damaged"
              />
            </div>
            <div>
              <div className="text-xs font-medium text-muted-foreground mb-1">Items</div>
              <div className="rounded-md border divide-y">
                {items.map((it) => {
                  const qty = returnQtys[it.id] ?? 0
                  return (
                    <div key={it.id} className="flex items-center justify-between px-3 py-2 text-sm">
                      <div>
                        <div className="font-mono text-xs">{it.product_id.split('-')[0]}</div>
                        <div className="text-xs text-muted-foreground">
                          ordered {it.quantity} × {formatPrice(it.unit_price)}
                        </div>
                      </div>
                      <Input
                        type="number"
                        min={0}
                        max={it.quantity}
                        value={qty}
                        onChange={(e) => {
                          const n = Math.min(it.quantity, Math.max(0, Number(e.target.value) || 0))
                          setReturnQtys((s) => ({ ...s, [it.id]: n }))
                        }}
                        className="w-20 text-right"
                      />
                    </div>
                  )
                })}
              </div>
              <div className="flex justify-between text-sm pt-2">
                <span className="text-muted-foreground">Estimated refund</span>
                <span className="font-semibold">
                  {formatPrice(
                    items.reduce((acc, it) => acc + (returnQtys[it.id] ?? 0) * it.unit_price, 0)
                  )}
                </span>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setReturnOpen(false)}>Cancel</Button>
            <Button
              disabled={!returnReason || returnMut.isPending}
              onClick={() => returnMut.mutate()}
            >
              {returnMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Submit return
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
