'use client'

import { useCallback, useEffect, useMemo, useState, Suspense } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, Eye, Plus, XCircle, RotateCcw, Download, ChevronLeft, ChevronRight, Search } from 'lucide-react'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { toast } from 'sonner'
import { useRouter, useSearchParams } from 'next/navigation'
import { HTTPError } from 'ky'

import { ordersApi } from '@/lib/api/orders'
import { paymentsApi } from '@/lib/api/payments'
import { formatPrice } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { useAuthStore } from '@/lib/stores/auth-store'
import { PERMISSIONS } from '@/lib/rbac'
import { useEventStream } from '@/hooks/useEventStream'
import type { Order, OrderStatus } from '@/types/api'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const orderStatusSchema = z.object({
  status: z.enum(['pending', 'accepted', 'rejected', 'processing', 'ready', 'shipped', 'delivered', 'completed', 'refunded', 'confirmed', 'cancelled']),
})
type OrderStatusFormValues = z.infer<typeof orderStatusSchema>

const getAllowedStatuses = (current: OrderStatus): OrderStatus[] => {
  switch (current) {
    case 'pending': return ['accepted', 'rejected', 'cancelled']
    case 'accepted': return ['processing', 'cancelled']
    case 'processing': return ['ready', 'cancelled']
    case 'ready': return ['shipped', 'cancelled']
    case 'shipped': return ['delivered']
    case 'delivered': return ['completed']
    case 'confirmed': return ['accepted', 'cancelled']
    case 'completed':
    case 'rejected':
    case 'refunded':
    case 'cancelled':
      return []
    default: return []
  }
}

function OrdersContent() {
  const queryClient = useQueryClient()
  const router = useRouter()
  const { can } = usePermission()
  const { user } = useAuthStore()

  // Real-time: refetch on order events AND payment events (capture, refund).
  useEventStream(['order', 'payment'], (evt) => {
    queryClient.invalidateQueries({ queryKey: ['orders'] })
    if (evt.type === 'order.created') {
      toast.success('New order received')
    } else if (evt.type === 'order.status_changed') {
      toast.info('Order status updated')
    } else if (evt.type === 'payment.captured') {
      toast.success('Payment received')
    } else if (evt.type === 'payment.failed') {
      toast.error('Payment failed')
    } else if (evt.type === 'payment.refunded') {
      toast.info('Payment refunded')
    }
  })

  const [isStatusDialogOpen, setIsStatusDialogOpen] = useState(false)
  const [isCancelDialogOpen, setIsCancelDialogOpen] = useState(false)
  const [isRefundDialogOpen, setIsRefundDialogOpen] = useState(false)
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)
  const [cancelReason, setCancelReason] = useState('')
  const [refundReason, setRefundReason] = useState('')

  // Filter / pagination state — synced with URL query params so a refresh
  // doesn't nuke them and shared links land on the same view.
  const searchParams = useSearchParams()
  const statusFilter = searchParams.get('status') ?? ''
  const paymentFilter = searchParams.get('payment_status') ?? ''
  const search = searchParams.get('search') ?? ''
  const fromDate = searchParams.get('from') ?? ''
  const toDate = searchParams.get('to') ?? ''
  const page = Number(searchParams.get('page') ?? '1')
  const perPage = Number(searchParams.get('per_page') ?? '25')

  // Mutator: builds a new URL with overridden params and pushes via router.
  // Clearing a param (set to '') drops it from the URL entirely.
  const setParams = useCallback((patch: Record<string, string | number | undefined>) => {
    const next = new URLSearchParams(searchParams.toString())
    for (const [k, v] of Object.entries(patch)) {
      if (v === undefined || v === '' || v === null) next.delete(k)
      else next.set(k, String(v))
    }
    router.replace(`/orders${next.toString() ? `?${next.toString()}` : ''}`, { scroll: false })
  }, [searchParams, router])

  // Search input keeps a local mirror so each keystroke debounces nicely.
  const [searchInput, setSearchInput] = useState(search)
  useEffect(() => { setSearchInput(search) }, [search])
  useEffect(() => {
    if (searchInput === search) return
    const t = setTimeout(() => setParams({ search: searchInput || undefined, page: 1 }), 250)
    return () => clearTimeout(t)
  }, [searchInput, search, setParams])

  const [isExporting, setIsExporting] = useState(false)
  // Bulk selection — set of selected order ids on the current page.
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())
  const [bulkStatus, setBulkStatus] = useState('')

  // Convert YYYY-MM-DD to RFC3339 covering the full local day. Backend
  // compares against created_at as TIMESTAMPTZ, so the bounds need to be
  // an explicit instant — empty input → undefined → no constraint.
  const filters = useMemo(() => {
    const fromRFC = fromDate ? new Date(fromDate + 'T00:00:00').toISOString() : undefined
    const toRFC = toDate ? new Date(toDate + 'T23:59:59.999').toISOString() : undefined
    return {
      status: statusFilter || undefined,
      payment_status: paymentFilter || undefined,
      search: search || undefined,
      from: fromRFC,
      to: toRFC,
      page,
      per_page: perPage,
    }
  }, [statusFilter, paymentFilter, search, fromDate, toDate, page, perPage])

  const { data: ordersResult, isLoading } = useQuery({
    queryKey: ['orders', filters],
    queryFn: () => ordersApi.list(filters),
  })

  // Fetch cancel-preview only while the dialog is open for the selected order.
  const cancelPreviewQ = useQuery({
    queryKey: ['order', selectedOrder?.id, 'cancel-preview'],
    queryFn: () => ordersApi.getCancelPreview(selectedOrder!.id),
    enabled: isCancelDialogOpen && !!selectedOrder,
  })

  // Payments index so we can map order → payment id for refunds.
  const { data: rawPayments } = useQuery({
    queryKey: ['payments'],
    queryFn: paymentsApi.list,
  })

  const orders = ordersResult?.items ?? []
  const totalOrders = ordersResult?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(totalOrders / perPage))
  const payments = rawPayments ?? []
  const paymentByOrderID = new Map<string, string>()
  for (const p of payments) {
    if (p.order_id && p.status === 'captured') {
      paymentByOrderID.set(p.order_id, p.id)
    }
  }

  const cancelMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => ordersApi.cancel(id, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      queryClient.invalidateQueries({ queryKey: ['inventory'] })
      setIsCancelDialogOpen(false)
      setCancelReason('')
      toast.success('Order cancelled')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Cancel failed')
      } else {
        toast.error(err.message)
      }
    },
  })

  const refundMutation = useMutation({
    mutationFn: ({ paymentID, reason }: { paymentID: string; reason: string }) =>
      paymentsApi.refund(paymentID, 0, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['payments'] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      setIsRefundDialogOpen(false)
      setRefundReason('')
      toast.success('Refund submitted')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Refund failed')
      } else {
        toast.error(err.message)
      }
    },
  })

  const canCancel = (s: OrderStatus) =>
    s === 'pending' || s === 'confirmed' || s === 'accepted' || s === 'processing' || s === 'ready'

  const bulkStatusMutation = useMutation({
    mutationFn: ({ ids, status }: { ids: string[]; status: string }) => ordersApi.bulkStatus(ids, status),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      setSelectedIDs(new Set())
      setBulkStatus('')
      if (res.skipped > 0) {
        toast.info(`${res.applied} updated, ${res.skipped} skipped (not eligible)`)
      } else {
        toast.success(`${res.applied} orders updated`)
      }
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Bulk update failed')
      } else toast.error(err.message)
    },
  })

  const updateStatusMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: { status: OrderStatus } }) => ordersApi.updateStatus(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      setIsStatusDialogOpen(false)
      toast.success('Order status updated')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const errorData = await err.response.json().catch(() => ({}))
        toast.error(errorData.error || errorData.message || 'Failed to update status')
      } else {
        toast.error(err.message || 'Failed to update status')
      }
    }
  })

  const {
    control,
    handleSubmit,
    reset,
    watch,
    formState: { isSubmitting }
  } = useForm<OrderStatusFormValues>({
    resolver: zodResolver(orderStatusSchema),
    defaultValues: {
      status: 'pending',
    }
  })

  // eslint-disable-next-line react-hooks/incompatible-library
  const currentStatus = watch('status')

  const onSubmitStatus = (data: OrderStatusFormValues) => {
    if (selectedOrder) {
      updateStatusMutation.mutate({ id: selectedOrder.id, data })
    }
  }

  const getStatusBadge = (status: OrderStatus) => {
    switch(status) {
      case 'pending': return <Badge variant="outline" className="bg-yellow-100 text-yellow-800 border-yellow-200">Pending</Badge>
      case 'accepted': return <Badge variant="outline" className="bg-blue-100 text-blue-800 border-blue-200">Accepted</Badge>
      case 'processing': return <Badge variant="outline" className="bg-indigo-100 text-indigo-800 border-indigo-200">Processing</Badge>
      case 'ready': return <Badge variant="outline" className="bg-cyan-100 text-cyan-800 border-cyan-200">Ready</Badge>
      case 'shipped': return <Badge variant="outline" className="bg-purple-100 text-purple-800 border-purple-200">Shipped</Badge>
      case 'delivered': return <Badge variant="outline" className="bg-green-100 text-green-800 border-green-200">Delivered</Badge>
      case 'completed': return <Badge variant="default" className="bg-green-600 text-white">Completed</Badge>
      case 'confirmed': return <Badge variant="outline" className="bg-emerald-100 text-emerald-800 border-emerald-200">Confirmed</Badge>
      case 'cancelled': return <Badge variant="destructive">Cancelled</Badge>
      case 'rejected': return <Badge variant="destructive">Rejected</Badge>
      case 'refunded': return <Badge variant="outline" className="bg-gray-100 text-gray-800 border-gray-200">Refunded</Badge>
      default: return <Badge variant="outline">{status}</Badge>
    }
  }

  const getTypeBadge = (type: string) => {
    switch(type) {
      case 'internal': return <Badge variant="secondary">Internal</Badge>
      case 'b2b': return <Badge variant="outline" className="border-blue-200 text-blue-700 bg-blue-50">B2B</Badge>
      case 'b2c': return <Badge variant="outline" className="border-green-200 text-green-700 bg-green-50">B2C</Badge>
      default: return <Badge variant="secondary">{type || 'Unknown'}</Badge>
    }
  }

  const getPaymentBadge = (status: string) => {
    switch(status) {
      case 'paid':     return <Badge className="bg-emerald-600 hover:bg-emerald-600 text-white">Paid</Badge>
      case 'unpaid':   return <Badge variant="outline" className="bg-yellow-100 text-yellow-800 border-yellow-200">Unpaid</Badge>
      case 'partial':  return <Badge variant="outline" className="bg-orange-100 text-orange-800 border-orange-200">Partial</Badge>
      case 'refunded': return <Badge variant="outline" className="bg-zinc-100 text-zinc-700 border-zinc-200">Refunded</Badge>
      default:         return <Badge variant="outline">{status || '—'}</Badge>
    }
  }

  const allowedTransitions = selectedOrder ? getAllowedStatuses(selectedOrder.status) : []

  // Trigger a browser download of the filtered orders as CSV. We reuse the
  // current filters (minus pagination) so what you see is what you export.
  const handleExport = async () => {
    try {
      setIsExporting(true)
      const { status, payment_status, search: searchQ, from, to } = filters
      const blob = await ordersApi.exportCsv({ status, payment_status, search: searchQ, from, to })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `orders-${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(url)
      toast.success('Export downloaded')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Export failed')
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Orders</h2>
          <p className="text-muted-foreground">Manage and process customer orders.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" disabled={isExporting} onClick={handleExport}>
            {isExporting
              ? <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              : <Download className="mr-2 h-4 w-4" />
            }
            Export CSV
          </Button>
          {can(PERMISSIONS.ORDERS_CREATE) && (
            <Button onClick={() => router.push('/marketplace')}>
              <Plus className="mr-2 h-4 w-4" />
              Create Order
            </Button>
          )}
        </div>
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative">
          <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search order / user id"
            className="pl-8 w-64"
          />
        </div>
        <Select
          value={statusFilter || 'all'}
          onValueChange={(v) => setParams({ status: v === 'all' ? undefined : v, page: 1 })}
        >
          <SelectTrigger className="w-44"><SelectValue placeholder="Status" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            {['pending','accepted','processing','ready','shipped','delivered','completed','cancelled','rejected','refunded','confirmed'].map(s => (
              <SelectItem key={s} value={s}>{s}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={paymentFilter || 'all'}
          onValueChange={(v) => setParams({ payment_status: v === 'all' ? undefined : v, page: 1 })}
        >
          <SelectTrigger className="w-44"><SelectValue placeholder="Payment" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All payment</SelectItem>
            {['unpaid','paid','partial','refunded'].map(s => (
              <SelectItem key={s} value={s}>{s}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="flex items-center gap-1">
          <Input
            type="date"
            value={fromDate}
            onChange={(e) => setParams({ from: e.target.value || undefined, page: 1 })}
            className="w-40"
            aria-label="From date"
          />
          <span className="text-muted-foreground text-xs">→</span>
          <Input
            type="date"
            value={toDate}
            onChange={(e) => setParams({ to: e.target.value || undefined, page: 1 })}
            className="w-40"
            aria-label="To date"
          />
        </div>
        {(statusFilter || paymentFilter || search || fromDate || toDate) && (
          <Button
            variant="ghost"
            onClick={() => {
              setSearchInput('')
              setParams({ status: undefined, payment_status: undefined, search: undefined, from: undefined, to: undefined, page: 1 })
            }}
          >
            Reset
          </Button>
        )}
        <span className="ml-auto text-sm text-muted-foreground">
          {totalOrders} {totalOrders === 1 ? 'order' : 'orders'}
        </span>
      </div>

      {/* Bulk action toolbar — visible only when one or more rows are selected */}
      {selectedIDs.size > 0 && can(PERMISSIONS.ORDERS_MANAGE) && (
        <div className="flex items-center gap-3 rounded-md border bg-amber-50 dark:bg-amber-950/20 border-amber-200 px-4 py-2 text-sm">
          <span className="font-medium">{selectedIDs.size} selected</span>
          <span className="text-muted-foreground">Mark as:</span>
          <Select value={bulkStatus} onValueChange={setBulkStatus}>
            <SelectTrigger className="w-44 h-8"><SelectValue placeholder="Choose status" /></SelectTrigger>
            <SelectContent>
              {['accepted','processing','ready','shipped','delivered','completed','cancelled'].map(s => (
                <SelectItem key={s} value={s}>{s}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            size="sm"
            disabled={!bulkStatus || bulkStatusMutation.isPending}
            onClick={() => bulkStatusMutation.mutate({ ids: Array.from(selectedIDs), status: bulkStatus })}
          >
            {bulkStatusMutation.isPending && <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />}
            Apply
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setSelectedIDs(new Set())}>
            Clear selection
          </Button>
        </div>
      )}

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              {can(PERMISSIONS.ORDERS_MANAGE) && (
                <TableHead className="w-8">
                  <Checkbox
                    checked={orders.length > 0 && orders.every(o => selectedIDs.has(o.id))}
                    onCheckedChange={(v) => {
                      const next = new Set(selectedIDs)
                      if (v) orders.forEach(o => next.add(o.id))
                      else orders.forEach(o => next.delete(o.id))
                      setSelectedIDs(next)
                    }}
                    aria-label="Select all on page"
                  />
                </TableHead>
              )}
              <TableHead>Order ID</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>User</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Payment</TableHead>
              <TableHead className="text-right">Total</TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeleton columns={can(PERMISSIONS.ORDERS_MANAGE) ? 9 : 8} rows={5} />
            ) : orders.length > 0 ? (
              orders.map((order) => (
                <TableRow
                  key={order.id}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => router.push(`/orders/${order.id}`)}
                >
                  {can(PERMISSIONS.ORDERS_MANAGE) && (
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        checked={selectedIDs.has(order.id)}
                        onCheckedChange={(v) => {
                          const next = new Set(selectedIDs)
                          if (v) next.add(order.id)
                          else next.delete(order.id)
                          setSelectedIDs(next)
                        }}
                        aria-label={`Select order ${order.id}`}
                      />
                    </TableCell>
                  )}
                  <TableCell className="font-mono text-xs">{order.id.split('-')[0]}</TableCell>
                  <TableCell>{getTypeBadge(order.order_type)}</TableCell>
                  <TableCell className="font-medium">
                    {order.user_id === user?.id ? (
                      <Badge variant="outline" className="bg-slate-100">You</Badge>
                    ) : (
                      <span className="font-mono text-xs text-muted-foreground">{order.user_id.split('-')[0]}</span>
                    )}
                  </TableCell>
                  <TableCell>{getStatusBadge(order.status)}</TableCell>
                  <TableCell>{getPaymentBadge(order.payment_status)}</TableCell>
                  <TableCell className="text-right">{formatPrice(order.total_amount)}</TableCell>
                  <TableCell>{new Date(order.created_at).toLocaleDateString()}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-2">
                      {can(PERMISSIONS.ORDERS_VIEW) && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => router.push(`/orders/${order.id}`)}
                          title="View details"
                        >
                          <Eye className="w-4 h-4 text-zinc-600" />
                        </Button>
                      )}
                      {can(PERMISSIONS.ORDERS_MANAGE) && (
                        <Button
                          variant="ghost"
                          size="icon"
                          disabled={getAllowedStatuses(order.status).length === 0}
                          onClick={() => {
                            setSelectedOrder(order)
                            reset({ status: order.status })
                            setIsStatusDialogOpen(true)
                          }}
                          title="Change status"
                        >
                          <Edit className="w-4 h-4 text-blue-600" />
                        </Button>
                      )}
                      {can(PERMISSIONS.ORDERS_MANAGE) && canCancel(order.status) && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setSelectedOrder(order)
                            setCancelReason('')
                            setIsCancelDialogOpen(true)
                          }}
                          title="Cancel order"
                        >
                          <XCircle className="w-4 h-4 text-red-600" />
                        </Button>
                      )}
                      {can(PERMISSIONS.ORDERS_MANAGE) && order.payment_status === 'paid' && paymentByOrderID.has(order.id) && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setSelectedOrder(order)
                            setRefundReason('')
                            setIsRefundDialogOpen(true)
                          }}
                          title="Refund payment"
                        >
                          <RotateCcw className="w-4 h-4 text-amber-600" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={can(PERMISSIONS.ORDERS_MANAGE) ? 9 : 8} className="h-24 text-center text-muted-foreground">
                  No orders found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Rows per page</span>
          <Select value={String(perPage)} onValueChange={(v) => setParams({ per_page: Number(v), page: 1 })}>
            <SelectTrigger className="w-20"><SelectValue /></SelectTrigger>
            <SelectContent>
              {[10, 25, 50, 100].map(n => <SelectItem key={n} value={String(n)}>{n}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="icon"
            disabled={page <= 1}
            onClick={() => setParams({ page: Math.max(1, page - 1) })}
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            disabled={page >= totalPages}
            onClick={() => setParams({ page: Math.min(totalPages, page + 1) })}
          >
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Update Status Dialog */}
      <Dialog open={isStatusDialogOpen} onOpenChange={setIsStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Order Status</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmitStatus)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="status">Status</Label>
              <Controller
                name="status"
                control={control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select status" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={selectedOrder?.status || 'pending'} disabled>
                        Current: {selectedOrder?.status}
                      </SelectItem>
                      {allowedTransitions.map((status) => (
                        <SelectItem key={status} value={status}>
                          {status.charAt(0).toUpperCase() + status.slice(1)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsStatusDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || updateStatusMutation.isPending || !allowedTransitions.includes(currentStatus)}>
                {(isSubmitting || updateStatusMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Status
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Cancel confirmation */}
      <Dialog open={isCancelDialogOpen} onOpenChange={setIsCancelDialogOpen}>
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
            <Button variant="outline" onClick={() => setIsCancelDialogOpen(false)}>
              Keep order
            </Button>
            <Button
              variant="destructive"
              disabled={cancelMutation.isPending}
              onClick={() =>
                selectedOrder && cancelMutation.mutate({ id: selectedOrder.id, reason: cancelReason })
              }
            >
              {cancelMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Cancel order
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Refund confirmation */}
      <Dialog open={isRefundDialogOpen} onOpenChange={setIsRefundDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Refund payment?</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              Full amount {selectedOrder && (<span className="font-semibold">{formatPrice(selectedOrder.total_amount)}</span>)} will be refunded to the customer&apos;s original payment method via RazorPay. Funds typically arrive in 5-7 business days.
            </p>
            <div>
              <Label htmlFor="refund-reason" className="text-xs">Reason (optional)</Label>
              <Input
                id="refund-reason"
                value={refundReason}
                onChange={(e) => setRefundReason(e.target.value)}
                placeholder="e.g. damaged item"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsRefundDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              className="bg-amber-600 hover:bg-amber-700 text-white"
              disabled={refundMutation.isPending}
              onClick={() => {
                if (!selectedOrder) return
                const paymentID = paymentByOrderID.get(selectedOrder.id)
                if (!paymentID) {
                  toast.error('No captured payment found for this order')
                  return
                }
                refundMutation.mutate({ paymentID, reason: refundReason })
              }}
            >
              {refundMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Refund
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function OrdersPage() {
  return (
    <Suspense fallback={
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold tracking-tight">Orders</h2>
            <p className="text-muted-foreground">Manage your incoming and outgoing orders.</p>
          </div>
        </div>
        <TableSkeleton columns={8} />
      </div>
    }>
      <OrdersContent />
    </Suspense>
  )
}
