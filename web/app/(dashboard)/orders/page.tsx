'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, Eye, Plus } from 'lucide-react'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { toast } from 'sonner'
import { useRouter } from 'next/navigation'
import { HTTPError } from 'ky'

import { ordersApi } from '@/lib/api/orders'
import { formatPrice } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { useAuthStore } from '@/lib/stores/auth-store'
import { PERMISSIONS } from '@/lib/rbac'
import { useEventStream } from '@/hooks/useEventStream'
import type { Order, OrderStatus, OrderItem } from '@/types/api'
import { Button } from '@/components/ui/button'
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
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

function OrderItemsList({ orderId }: { orderId: string }) {
  const { data: items, isLoading } = useQuery({
    queryKey: ['orders', orderId, 'items'],
    queryFn: () => ordersApi.getItems(orderId),
  })

  if (isLoading) {
    return <div className="py-6 flex justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
  }

  if (!items || items.length === 0) {
    return <div className="py-6 text-center text-muted-foreground">No items found for this order.</div>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Product</TableHead>
          <TableHead className="text-right">Qty</TableHead>
          <TableHead className="text-right">Unit Price</TableHead>
          <TableHead className="text-right">Total</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map(item => (
          <TableRow key={item.id}>
            <TableCell>{item.product_name || item.product_id.split('-')[0]}</TableCell>
            <TableCell className="text-right">{item.quantity}</TableCell>
            <TableCell className="text-right">{formatPrice(item.unit_price)}</TableCell>
            <TableCell className="text-right font-medium">{formatPrice(item.quantity * item.unit_price)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export default function OrdersPage() {
  const queryClient = useQueryClient()
  const router = useRouter()
  const { can } = usePermission()
  const { user } = useAuthStore()

  // Real-time: refetch when any order event fires, toast new orders.
  useEventStream(['order'], (evt) => {
    queryClient.invalidateQueries({ queryKey: ['orders'] })
    if (evt.type === 'order.created') {
      toast.success('New order received')
    } else if (evt.type === 'order.status_changed') {
      toast.info('Order status updated')
    }
  })

  const [isStatusDialogOpen, setIsStatusDialogOpen] = useState(false)
  const [isItemsDialogOpen, setIsItemsDialogOpen] = useState(false)
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)

  const { data: rawOrders, isLoading } = useQuery({
    queryKey: ['orders'],
    queryFn: ordersApi.list,
  })

  const orders = rawOrders ?? []

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

  const allowedTransitions = selectedOrder ? getAllowedStatuses(selectedOrder.status) : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Orders</h2>
          <p className="text-muted-foreground">Manage and process customer orders.</p>
        </div>
        {can(PERMISSIONS.ORDERS_CREATE) && (
          <Button onClick={() => router.push('/marketplace')}>
            <Plus className="mr-2 h-4 w-4" />
            Create Order
          </Button>
        )}
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Order ID</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>User</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Total</TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeleton columns={7} rows={5} />
            ) : orders.length > 0 ? (
              orders.map((order) => (
                <TableRow key={order.id}>
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
                  <TableCell className="text-right">{formatPrice(order.total_amount)}</TableCell>
                  <TableCell>{new Date(order.created_at).toLocaleDateString()}</TableCell>
                  <TableCell>
                    <div className="flex items-center justify-end gap-2">
                      {can(PERMISSIONS.ORDERS_VIEW) && (
                        <Button variant="ghost" size="icon" onClick={() => {
                          setSelectedOrder(order)
                          setIsItemsDialogOpen(true)
                        }}>
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
                        >
                          <Edit className="w-4 h-4 text-blue-600" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={7} className="h-24 text-center text-muted-foreground">
                  No orders found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
              <Button type="submit" disabled={isSubmitting || updateStatusMutation.isPending || !allowedTransitions.includes(watch('status'))}>
                {(isSubmitting || updateStatusMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Status
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* View Items Dialog */}
      <Dialog open={isItemsDialogOpen} onOpenChange={setIsItemsDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Order Items</DialogTitle>
          </DialogHeader>
          
          {selectedOrder && (
            <OrderItemsList orderId={selectedOrder.id} />
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setIsItemsDialogOpen(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
