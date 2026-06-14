'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, Eye } from 'lucide-react'

import { ordersApi } from '@/lib/api/orders'
import { formatPrice } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import type { Order, OrderStatus } from '@/types/api'

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
  status: z.enum(['pending', 'confirmed', 'cancelled']),
})
type OrderStatusFormValues = z.infer<typeof orderStatusSchema>

export default function OrdersPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
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
    },
  })

  const {
    control,
    handleSubmit,
    reset,
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
      case 'pending':
        return <Badge variant="outline" className="bg-yellow-100 text-yellow-800 border-yellow-200">Pending</Badge>
      case 'confirmed':
        return <Badge variant="outline" className="bg-green-100 text-green-800 border-green-200">Confirmed</Badge>
      case 'cancelled':
        return <Badge variant="outline" className="bg-red-100 text-red-800 border-red-200">Cancelled</Badge>
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Orders</h2>
          <p className="text-muted-foreground">Manage and process customer orders.</p>
        </div>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Order ID</TableHead>
              <TableHead>User ID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Total Amount</TableHead>
              <TableHead>Date</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center">
                  <Loader2 className="w-6 h-6 animate-spin mx-auto text-muted-foreground" />
                </TableCell>
              </TableRow>
            ) : orders.length > 0 ? (
              orders.map((order) => (
                <TableRow key={order.id}>
                  <TableCell className="font-mono text-xs">{order.id.split('-')[0]}</TableCell>
                  <TableCell className="font-mono text-xs">{order.user_id.split('-')[0]}</TableCell>
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
                        <Button variant="ghost" size="icon" onClick={() => {
                          setSelectedOrder(order)
                          reset({ status: order.status })
                          setIsStatusDialogOpen(true)
                        }}>
                          <Edit className="w-4 h-4 text-blue-600" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
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
                      <SelectItem value="pending">Pending</SelectItem>
                      <SelectItem value="confirmed">Confirmed</SelectItem>
                      <SelectItem value="cancelled">Cancelled</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsStatusDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || updateStatusMutation.isPending}>
                {(isSubmitting || updateStatusMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Status
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* View Items Dialog (Placeholder) */}
      <Dialog open={isItemsDialogOpen} onOpenChange={setIsItemsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Order Items</DialogTitle>
          </DialogHeader>
          <div className="py-6 text-center text-muted-foreground">
            Order items view implementation pending.
          </div>
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
