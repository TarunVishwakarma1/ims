'use client'

import { useState, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, AlertTriangle, CheckCircle2, Plus, XCircle } from 'lucide-react'
import { TableSkeleton } from '@/components/ui/table-skeleton'

import { inventoryApi } from '@/lib/api/inventory'
import { productsApi } from '@/lib/api/products'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import { useEventStream } from '@/hooks/useEventStream'
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

const inventoryUpdateSchema = z.object({
  quantity: z.number().min(0, 'Quantity cannot be negative'),
  low_stock_threshold: z.number().min(0, 'Threshold cannot be negative'),
})
type InventoryUpdateFormValues = z.infer<typeof inventoryUpdateSchema>

const inventoryCreateSchema = z.object({
  product_id: z.string().min(1, 'Product is required'),
  quantity: z.number().min(0, 'Quantity cannot be negative'),
  low_stock_threshold: z.number().min(0, 'Threshold cannot be negative'),
})
type InventoryCreateFormValues = z.infer<typeof inventoryCreateSchema>

function InventoryContent() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  const router = useRouter()
  const searchParams = useSearchParams()

  // Show-only-low-stock toggle. Honors ?lowOnly=1 in the URL so the toast
  // action button in the sidebar can deep-link straight to the filtered view.
  const currentSearchLowOnly = searchParams.get('lowOnly') === '1'
  const [lowOnly, setLowOnly] = useState(currentSearchLowOnly)
  const [prevSearchLowOnly, setPrevSearchLowOnly] = useState(currentSearchLowOnly)

  if (currentSearchLowOnly !== prevSearchLowOnly) {
    setPrevSearchLowOnly(currentSearchLowOnly)
    setLowOnly(currentSearchLowOnly)
  }

  // Live updates: invalidate the inventory query whenever the backend
  // emits an inventory.* event. Low-stock toast is handled globally by
  // the sidebar so it fires on every page, not just /inventory.
  useEventStream(['inventory'], (evt) => {
    if (evt.type === 'inventory.updated' || evt.type === 'inventory.low_stock') {
      queryClient.invalidateQueries({ queryKey: ['inventory'] })
    }
  })

  const [isUpdateDialogOpen, setIsUpdateDialogOpen] = useState(false)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [selectedItem, setSelectedItem] = useState<Inventory | null>(null)

  const { data: rawInventory, isLoading: isLoadingInventory } = useQuery({
    queryKey: ['inventory'],
    queryFn: inventoryApi.list,
  })

  const { data: rawProducts, isLoading: isLoadingProducts } = useQuery({
    queryKey: ['products'],
    queryFn: productsApi.list,
  })

  const fullInventory = rawInventory ?? []
  const lowStockTotal = fullInventory.filter(i => i.quantity <= i.low_stock_threshold).length
  const inventoryList = lowOnly
    ? fullInventory.filter(i => i.quantity <= i.low_stock_threshold)
    : fullInventory
  const products = rawProducts ?? []

  // Products that do not have an inventory record yet
  const uninventoriedProducts = products.filter(p => !inventoryList.some(inv => inv.product_id === p.id))

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof inventoryApi.update>[1] }) => inventoryApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inventory'] })
      setIsUpdateDialogOpen(false)
    },
  })

  const createMutation = useMutation({
    mutationFn: (data: InventoryCreateFormValues) => inventoryApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inventory'] })
      setIsCreateDialogOpen(false)
    },
  })

  const {
    register: registerUpdate,
    handleSubmit: handleUpdateSubmit,
    reset: resetUpdate,
    formState: { errors: updateErrors, isSubmitting: isUpdating }
  } = useForm<InventoryUpdateFormValues>({
    resolver: zodResolver(inventoryUpdateSchema),
    defaultValues: {
      quantity: 0,
      low_stock_threshold: 0,
    }
  })

  const {
    register: registerCreate,
    handleSubmit: handleCreateSubmit,
    reset: resetCreate,
    formState: { errors: createErrors, isSubmitting: isCreating }
  } = useForm<InventoryCreateFormValues>({
    resolver: zodResolver(inventoryCreateSchema),
    defaultValues: {
      product_id: '',
      quantity: 0,
      low_stock_threshold: 10,
    }
  })

  const onUpdateSubmit = (data: InventoryUpdateFormValues) => {
    if (selectedItem) {
      updateMutation.mutate({ id: selectedItem.id, data })
    }
  }

  const onCreateSubmit = (data: InventoryCreateFormValues) => {
    createMutation.mutate(data)
  }

  const isLoading = isLoadingInventory || isLoadingProducts

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Inventory</h2>
          <p className="text-muted-foreground">Monitor and update stock levels.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={lowOnly ? 'default' : 'outline'}
            onClick={() => setLowOnly(s => !s)}
          >
            <AlertTriangle className="mr-2 h-4 w-4" />
            {lowOnly ? `Showing low stock (${lowStockTotal})` : `Low stock (${lowStockTotal})`}
          </Button>
          {can(PERMISSIONS.INVENTORY_MANAGE) && uninventoriedProducts.length > 0 && (
            <Button onClick={() => {
              resetCreate({ product_id: uninventoriedProducts[0]?.id || '', quantity: 0, low_stock_threshold: 10 })
              setIsCreateDialogOpen(true)
            }}>
              <Plus className="mr-2 h-4 w-4" />
              Add Inventory
            </Button>
          )}
        </div>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Product Name</TableHead>
              <TableHead>SKU</TableHead>
              <TableHead className="text-right">Quantity</TableHead>
              <TableHead className="text-right">Low Stock Threshold</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[100px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeleton columns={6} rows={5} />
            ) : inventoryList.length > 0 ? (
              inventoryList.map((inv) => {
                const product = products.find(p => p.id === inv.product_id)
                const isOutOfStock = inv.quantity === 0
                const isLowStock = !isOutOfStock && inv.quantity <= inv.low_stock_threshold
                
                return (
                  <TableRow
                    key={inv.id}
                    className="cursor-pointer hover:bg-muted/50"
                    onClick={() => router.push(`/inventory/${inv.id}`)}
                  >
                    <TableCell className="font-medium">
                      {product?.name ?? 'Unknown Product'}
                    </TableCell>
                    <TableCell className="text-muted-foreground font-mono text-sm">
                      {product?.sku ?? 'N/A'}
                    </TableCell>
                    <TableCell className="text-right">{inv.quantity}</TableCell>
                    <TableCell className="text-right">{inv.low_stock_threshold}</TableCell>
                    <TableCell>
                      {isOutOfStock ? (
                        <Badge variant="destructive" className="flex w-fit items-center gap-1">
                          <XCircle className="w-3 h-3" /> Out of Stock
                        </Badge>
                      ) : isLowStock ? (
                        <Badge variant="outline" className="flex w-fit items-center gap-1 border-yellow-500 text-yellow-600 bg-yellow-50 dark:bg-yellow-900/20">
                          <AlertTriangle className="w-3 h-3" /> Low Stock
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="flex w-fit items-center gap-1 bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 hover:bg-green-100 dark:hover:bg-green-900">
                          <CheckCircle2 className="w-3 h-3" /> OK
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <div className="flex items-center justify-end">
                        {can(PERMISSIONS.INVENTORY_MANAGE) && (
                          <Button variant="ghost" size="icon" onClick={() => {
                            setSelectedItem(inv)
                            resetUpdate({
                              quantity: inv.quantity,
                              low_stock_threshold: inv.low_stock_threshold,
                            })
                            setIsUpdateDialogOpen(true)
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
                <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                  No inventory records found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Update Inventory Dialog */}
      <Dialog open={isUpdateDialogOpen} onOpenChange={setIsUpdateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Inventory</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpdateSubmit(onUpdateSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="quantity">Quantity</Label>
              <Input id="quantity" type="number" {...registerUpdate('quantity', { valueAsNumber: true })} />
              {updateErrors.quantity && <p className="text-xs text-red-500">{updateErrors.quantity.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="low_stock_threshold">Low Stock Threshold</Label>
              <Input id="low_stock_threshold" type="number" {...registerUpdate('low_stock_threshold', { valueAsNumber: true })} />
              {updateErrors.low_stock_threshold && <p className="text-xs text-red-500">{updateErrors.low_stock_threshold.message}</p>}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsUpdateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isUpdating || updateMutation.isPending}>
                {(isUpdating || updateMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Changes
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Create Inventory Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Inventory Record</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreateSubmit(onCreateSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="product_id">Product</Label>
              <select 
                id="product_id" 
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                {...registerCreate('product_id')}
              >
                {uninventoriedProducts.map(p => (
                  <option key={p.id} value={p.id}>{p.name} ({p.sku})</option>
                ))}
              </select>
              {createErrors.product_id && <p className="text-xs text-red-500">{createErrors.product_id.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="create_quantity">Initial Quantity</Label>
              <Input id="create_quantity" type="number" {...registerCreate('quantity', { valueAsNumber: true })} />
              {createErrors.quantity && <p className="text-xs text-red-500">{createErrors.quantity.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="create_threshold">Low Stock Threshold</Label>
              <Input id="create_threshold" type="number" {...registerCreate('low_stock_threshold', { valueAsNumber: true })} />
              {createErrors.low_stock_threshold && <p className="text-xs text-red-500">{createErrors.low_stock_threshold.message}</p>}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isCreating || createMutation.isPending}>
                {(isCreating || createMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Add Inventory
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function InventoryPage() {
  return (
    <Suspense fallback={
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold tracking-tight">Inventory</h2>
            <p className="text-muted-foreground">Manage your product stock levels and alerts.</p>
          </div>
        </div>
        <TableSkeleton columns={7} />
      </div>
    }>
      <InventoryContent />
    </Suspense>
  )
}
