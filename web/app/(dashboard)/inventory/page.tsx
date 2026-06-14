'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, AlertTriangle, CheckCircle2 } from 'lucide-react'

import { inventoryApi } from '@/lib/api/inventory'
import { productsApi } from '@/lib/api/products'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import type { Inventory } from '@/types/api'

import { Button } from '@/components/ui/button'
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

const inventorySchema = z.object({
  quantity: z.number().min(0, 'Quantity cannot be negative'),
  low_stock_threshold: z.number().min(0, 'Threshold cannot be negative'),
})
type InventoryFormValues = z.infer<typeof inventorySchema>

export default function InventoryPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [selectedItem, setSelectedItem] = useState<Inventory | null>(null)

  const { data: rawInventory, isLoading: isLoadingInventory } = useQuery({
    queryKey: ['inventory'],
    queryFn: inventoryApi.list,
  })

  const { data: rawProducts, isLoading: isLoadingProducts } = useQuery({
    queryKey: ['products'],
    queryFn: productsApi.list,
  })

  const inventoryList = rawInventory ?? []
  const products = rawProducts ?? []

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof inventoryApi.update>[1] }) => inventoryApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inventory'] })
      setIsDialogOpen(false)
    },
  })

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<InventoryFormValues>({
    resolver: zodResolver(inventorySchema),
    defaultValues: {
      quantity: 0,
      low_stock_threshold: 0,
    }
  })

  const onSubmit = (data: InventoryFormValues) => {
    if (selectedItem) {
      updateMutation.mutate({ id: selectedItem.id, data })
    }
  }

  const isLoading = isLoadingInventory || isLoadingProducts

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Inventory</h2>
          <p className="text-muted-foreground">Monitor and update stock levels.</p>
        </div>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Product Name</TableHead>
              <TableHead className="text-right">Quantity</TableHead>
              <TableHead className="text-right">Low Stock Threshold</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[100px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  <Loader2 className="w-6 h-6 animate-spin mx-auto text-muted-foreground" />
                </TableCell>
              </TableRow>
            ) : inventoryList.length > 0 ? (
              inventoryList.map((inv) => {
                const product = products.find(p => p.id === inv.product_id)
                const isLowStock = inv.quantity <= inv.low_stock_threshold
                
                return (
                  <TableRow key={inv.id}>
                    <TableCell className="font-medium">
                      {product?.name ?? 'Unknown Product'}
                    </TableCell>
                    <TableCell className="text-right">{inv.quantity}</TableCell>
                    <TableCell className="text-right">{inv.low_stock_threshold}</TableCell>
                    <TableCell>
                      {isLowStock ? (
                        <Badge variant="destructive" className="flex w-fit items-center gap-1">
                          <AlertTriangle className="w-3 h-3" /> Low Stock
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="flex w-fit items-center gap-1 bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 hover:bg-green-100 dark:hover:bg-green-900">
                          <CheckCircle2 className="w-3 h-3" /> OK
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end">
                        {can(PERMISSIONS.INVENTORY_MANAGE) && (
                          <Button variant="ghost" size="icon" onClick={() => {
                            setSelectedItem(inv)
                            reset({
                              quantity: inv.quantity,
                              low_stock_threshold: inv.low_stock_threshold,
                            })
                            setIsDialogOpen(true)
                          }}>
                            <Edit className="w-4 h-4 text-blue-600" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            ) : (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No inventory records found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Inventory</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="quantity">Quantity</Label>
              <Input id="quantity" type="number" {...register('quantity', { valueAsNumber: true })} />
              {errors.quantity && <p className="text-xs text-red-500">{errors.quantity.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="low_stock_threshold">Low Stock Threshold</Label>
              <Input id="low_stock_threshold" type="number" {...register('low_stock_threshold', { valueAsNumber: true })} />
              {errors.low_stock_threshold && <p className="text-xs text-red-500">{errors.low_stock_threshold.message}</p>}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || updateMutation.isPending}>
                {(isSubmitting || updateMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Changes
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
