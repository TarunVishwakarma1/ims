'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, Edit, Trash2, Loader2 } from 'lucide-react'
import { TableSkeleton } from '@/components/ui/table-skeleton'

import { categoriesApi } from '@/lib/api/categories'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import type { Category } from '@/types/api'

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

const categorySchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional(),
})
type CategoryFormValues = z.infer<typeof categorySchema>

export default function CategoriesPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
  // State for dialogs
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null)

  // Fetch Data
  const { data: rawCategories, isLoading: isLoadingCategories } = useQuery({
    queryKey: ['categories'],
    queryFn: categoriesApi.list,
  })

  const categories = rawCategories ?? []

  // Mutations
  const createMutation = useMutation({
    mutationFn: categoriesApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['categories'] })
      setIsDialogOpen(false)
      reset()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof categoriesApi.update>[1] }) => categoriesApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['categories'] })
      setIsDialogOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: categoriesApi.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['categories'] })
      setIsDeleteDialogOpen(false)
    },
  })

  // Form Setup
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<CategoryFormValues>({
    resolver: zodResolver(categorySchema),
    defaultValues: {
      name: '',
      description: '',
    }
  })

  // Handlers
  const handleOpenCreate = () => {
    setSelectedCategory(null)
    reset({
      name: '',
      description: '',
    })
    setIsDialogOpen(true)
  }

  const onSubmit = (data: CategoryFormValues) => {
    const payload = {
      ...data,
      description: data.description || '',
    }

    if (selectedCategory) {
      updateMutation.mutate({ id: selectedCategory.id, data: payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  const handleDelete = () => {
    if (selectedCategory) {
      deleteMutation.mutate(selectedCategory.id)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Categories</h2>
          <p className="text-muted-foreground">Manage your product categories.</p>
        </div>
        {can(PERMISSIONS.CATEGORIES_MANAGE) && (
          <Button onClick={handleOpenCreate}>
            <Plus className="mr-2 h-4 w-4" /> Add Category
          </Button>
        )}
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead className="w-[100px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoadingCategories ? (
              <TableSkeleton columns={3} rows={5} />
            ) : categories.length > 0 ? (
              categories.map((category) => (
                <TableRow key={category.id}>
                  <TableCell className="font-medium">{category.name}</TableCell>
                  <TableCell className="text-muted-foreground truncate max-w-md">
                    {category.description || '-'}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center justify-end gap-2">
                      {can(PERMISSIONS.CATEGORIES_MANAGE) && (
                        <>
                          <Button variant="ghost" size="icon" onClick={() => {
                            setSelectedCategory(category)
                            reset({
                              name: category.name,
                              description: category.description,
                            })
                            setIsDialogOpen(true)
                          }}>
                            <Edit className="w-4 h-4 text-blue-600" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => {
                            setSelectedCategory(category)
                            setIsDeleteDialogOpen(true)
                          }}>
                            <Trash2 className="w-4 h-4 text-red-600" />
                          </Button>
                        </>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={3} className="h-24 text-center text-muted-foreground">
                  No categories found.
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
              {selectedCategory ? 'Edit Category' : 'Add Category'}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Name</Label>
              <Input id="name" {...register('name')} />
              {errors.name && <p className="text-xs text-red-500">{errors.name.message}</p>}
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
                {selectedCategory ? 'Save Changes' : 'Create Category'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Category</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <strong>{selectedCategory?.name}</strong>? This action cannot be undone.
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
