'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, Edit, Trash2, Loader2 } from 'lucide-react'

import { productsApi } from '@/lib/api/products'
import { categoriesApi } from '@/lib/api/categories'
import { formatPrice } from '@/lib/utils'
import type { Product, Category } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// Form schema expects price in Rupees
const productSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  sku: z.string().min(1, 'SKU is required'),
  category_id: z.string().min(1, 'Category is required'),
  description: z.string().optional(),
  price: z.number().min(0, 'Price must be a positive number'),
})
type ProductFormValues = z.infer<typeof productSchema>

export default function ProductsPage() {
  const queryClient = useQueryClient()
  
  // State for dialogs
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null)

  // Fetch Data
  const { data: rawProducts, isLoading: isLoadingProducts } = useQuery({
    queryKey: ['products'],
    queryFn: productsApi.list,
  })

  const { data: rawCategories, isLoading: isLoadingCategories } = useQuery({
    queryKey: ['categories'],
    queryFn: categoriesApi.list,
  })

  const products = rawProducts ?? []
  const categories = rawCategories ?? []

  // Mutations
  const createMutation = useMutation({
    mutationFn: productsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['products'] })
      setIsDialogOpen(false)
      reset()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof productsApi.update>[1] }) => productsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['products'] })
      setIsDialogOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: productsApi.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['products'] })
      setIsDeleteDialogOpen(false)
    },
  })

  // Form Setup
  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<ProductFormValues>({
    resolver: zodResolver(productSchema),
    defaultValues: {
      name: '',
      sku: '',
      category_id: '',
      description: '',
      price: 0,
    }
  })

  const categoryIdValue = watch('category_id')


  // Handlers
  const handleOpenCreate = () => {
    setSelectedProduct(null)
    reset({
      name: '',
      sku: '',
      category_id: '',
      description: '',
      price: 0,
    })
    setIsDialogOpen(true)
  }

  const onSubmit = (data: ProductFormValues) => {
    // Convert rupees back to paise for backend
    const payload = {
      ...data,
      description: data.description || '',
      price: Math.round(data.price * 100),
    }

    if (selectedProduct) {
      updateMutation.mutate({ id: selectedProduct.id, data: payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  const handleDelete = () => {
    if (selectedProduct) {
      deleteMutation.mutate(selectedProduct.id)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Products</h2>
          <p className="text-muted-foreground">Manage your product catalog.</p>
        </div>
        <Button onClick={handleOpenCreate}>
          <Plus className="mr-2 h-4 w-4" /> Add Product
        </Button>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>SKU</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Price</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoadingProducts || isLoadingCategories ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  <Loader2 className="w-6 h-6 animate-spin mx-auto text-muted-foreground" />
                </TableCell>
              </TableRow>
            ) : products.length > 0 ? (
              products.map((product) => (
                <TableRow key={product.id}>
                  <TableCell>{product.name}</TableCell>
                  <TableCell>{product.sku}</TableCell>
                  <TableCell>
                    {categories.find(c => c.id === product.category_id)?.name ?? 'Unknown'}
                  </TableCell>
                  <TableCell>{formatPrice(product.price)}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Button variant="ghost" size="icon" onClick={() => {
                        setSelectedProduct(product)
                        reset({
                          name: product.name,
                          sku: product.sku,
                          category_id: product.category_id,
                          description: product.description,
                          price: product.price / 100,
                        })
                        setIsDialogOpen(true)
                      }}>
                        <Edit className="w-4 h-4 text-blue-600" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => {
                        setSelectedProduct(product)
                        setIsDeleteDialogOpen(true)
                      }}>
                        <Trash2 className="w-4 h-4 text-red-600" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No products found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Create/Edit Dialog */}
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {selectedProduct ? 'Edit Product' : 'Add Product'}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Name</Label>
              <Input id="name" {...register('name')} />
              {errors.name && <p className="text-xs text-red-500">{errors.name.message}</p>}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="sku">SKU</Label>
                <Input id="sku" {...register('sku')} />
                {errors.sku && <p className="text-xs text-red-500">{errors.sku.message}</p>}
              </div>

              <div className="grid gap-2">
                <Label htmlFor="price">Price (₹)</Label>
                <Input id="price" type="number" step="0.01" {...register('price', { valueAsNumber: true })} />
                {errors.price && <p className="text-xs text-red-500">{errors.price.message}</p>}
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="category">Category</Label>
              <Select
                value={categoryIdValue}
                onValueChange={(val) => setValue('category_id', val)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a category" />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((cat) => (
                    <SelectItem key={cat.id} value={cat.id}>
                      {cat.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.category_id && <p className="text-xs text-red-500">{errors.category_id.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="description">Description (optional)</Label>
              <Textarea id="description" {...register('description')} />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || createMutation.isPending || updateMutation.isPending}>
                {(isSubmitting || createMutation.isPending || updateMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                {selectedProduct ? 'Save Changes' : 'Create Product'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Product</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <strong>{selectedProduct?.name}</strong>? This action cannot be undone.
          </p>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>
              Cancel
            </Button>
            <Button 
              type="button" 
              variant="destructive" 
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
