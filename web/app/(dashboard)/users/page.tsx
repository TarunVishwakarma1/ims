'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Edit, Loader2, Trash2, Shield, User as UserIcon, Plus } from 'lucide-react'
import { TableSkeleton } from '@/components/ui/table-skeleton'

import { usersApi } from '@/lib/api/users'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import type { User, Role } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
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

const createUserSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  email: z.string().email('Valid email is required'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  role: z.enum(['admin', 'manager', 'staff']),
})
type CreateUserFormValues = z.infer<typeof createUserSchema>

const editUserSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  email: z.string().email('Valid email is required'),
  role: z.enum(['admin', 'manager', 'staff']),
  is_active: z.boolean(),
})
type EditUserFormValues = z.infer<typeof editUserSchema>

export default function UsersPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)

  const { data: rawUsers, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: usersApi.list,
    enabled: can(PERMISSIONS.USERS_VIEW),
  })

  const users = rawUsers ?? []

  const createMutation = useMutation({
    mutationFn: usersApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setIsCreateDialogOpen(false)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof usersApi.update>[1] }) => usersApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setIsEditDialogOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: usersApi.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setIsDeleteDialogOpen(false)
    },
  })

  const createForm = useForm<CreateUserFormValues>({
    resolver: zodResolver(createUserSchema),
    defaultValues: {
      name: '',
      email: '',
      password: '',
      role: 'staff',
    }
  })

  const editForm = useForm<EditUserFormValues>({
    resolver: zodResolver(editUserSchema),
    defaultValues: {
      name: '',
      email: '',
      role: 'staff',
      is_active: true,
    }
  })

  const onCreateSubmit = (data: CreateUserFormValues) => {
    createMutation.mutate(data)
  }

  const onEditSubmit = (data: EditUserFormValues) => {
    if (selectedUser) {
      updateMutation.mutate({ id: selectedUser.id, data })
    }
  }

  const handleDelete = () => {
    if (selectedUser) {
      deleteMutation.mutate(selectedUser.id)
    }
  }

  const getRoleBadge = (role: Role) => {
    switch(role) {
      case 'admin':
        return <Badge className="bg-purple-100 text-purple-800 hover:bg-purple-100"><Shield className="w-3 h-3 mr-1" /> Admin</Badge>
      case 'manager':
        return <Badge className="bg-blue-100 text-blue-800 hover:bg-blue-100"><UserIcon className="w-3 h-3 mr-1" /> Manager</Badge>
      case 'staff':
        return <Badge variant="secondary"><UserIcon className="w-3 h-3 mr-1" /> Staff</Badge>
    }
  }

  if (!can(PERMISSIONS.USERS_VIEW)) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <p className="text-muted-foreground">You do not have permission to view this page.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Users</h2>
          <p className="text-muted-foreground">Manage team access and roles.</p>
        </div>
        {can(PERMISSIONS.USERS_CREATE) && (
          <Button onClick={() => {
            createForm.reset({ name: '', email: '', password: '', role: 'staff' })
            setIsCreateDialogOpen(true)
          }}>
            <Plus className="mr-2 h-4 w-4" /> Add User
          </Button>
        )}
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableSkeleton columns={5} rows={5} />
            ) : users.length > 0 ? (
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">{user.name}</TableCell>
                  <TableCell>{user.email}</TableCell>
                  <TableCell>{getRoleBadge(user.role)}</TableCell>
                  <TableCell>
                    {user.is_active ? (
                      <Badge variant="outline" className="border-green-200 text-green-700">Active</Badge>
                    ) : (
                      <Badge variant="outline" className="border-red-200 text-red-700">Inactive</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center justify-end gap-2">
                      {can(PERMISSIONS.USERS_EDIT) && (
                        <Button variant="ghost" size="icon" onClick={() => {
                          setSelectedUser(user)
                          editForm.reset({
                            name: user.name,
                            email: user.email,
                            role: user.role,
                            is_active: user.is_active,
                          })
                          setIsEditDialogOpen(true)
                        }}>
                          <Edit className="w-4 h-4 text-blue-600" />
                        </Button>
                      )}
                      {can(PERMISSIONS.USERS_DELETE) && (
                        <Button variant="ghost" size="icon" onClick={() => {
                          setSelectedUser(user)
                          setIsDeleteDialogOpen(true)
                        }}>
                          <Trash2 className="w-4 h-4 text-red-600" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No users found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Create Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add User</DialogTitle>
          </DialogHeader>
          <form onSubmit={createForm.handleSubmit(onCreateSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="create-name">Name</Label>
              <Input id="create-name" {...createForm.register('name')} />
              {createForm.formState.errors.name && <p className="text-xs text-red-500">{createForm.formState.errors.name.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="create-email">Email</Label>
              <Input id="create-email" type="email" {...createForm.register('email')} />
              {createForm.formState.errors.email && <p className="text-xs text-red-500">{createForm.formState.errors.email.message}</p>}
            </div>
            
            <div className="grid gap-2">
              <Label htmlFor="create-password">Password</Label>
              <Input id="create-password" type="password" {...createForm.register('password')} />
              {createForm.formState.errors.password && <p className="text-xs text-red-500">{createForm.formState.errors.password.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="create-role">Role</Label>
              <Controller
                name="role"
                control={createForm.control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select role" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">Admin</SelectItem>
                      <SelectItem value="manager">Manager</SelectItem>
                      <SelectItem value="staff">Staff</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
              {createForm.formState.errors.role && <p className="text-xs text-red-500">{createForm.formState.errors.role.message}</p>}
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createForm.formState.isSubmitting || createMutation.isPending}>
                {(createForm.formState.isSubmitting || createMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Create User
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit User</DialogTitle>
          </DialogHeader>
          <form onSubmit={editForm.handleSubmit(onEditSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="edit-name">Name</Label>
              <Input id="edit-name" {...editForm.register('name')} />
              {editForm.formState.errors.name && <p className="text-xs text-red-500">{editForm.formState.errors.name.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="edit-email">Email</Label>
              <Input id="edit-email" type="email" {...editForm.register('email')} />
              {editForm.formState.errors.email && <p className="text-xs text-red-500">{editForm.formState.errors.email.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="edit-role">Role</Label>
              <Controller
                name="role"
                control={editForm.control}
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select role" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">Admin</SelectItem>
                      <SelectItem value="manager">Manager</SelectItem>
                      <SelectItem value="staff">Staff</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
              {editForm.formState.errors.role && <p className="text-xs text-red-500">{editForm.formState.errors.role.message}</p>}
            </div>

            <div className="flex items-center space-x-2 pt-2">
              <Controller
                name="is_active"
                control={editForm.control}
                render={({ field }) => (
                  <Checkbox 
                    id="is_active" 
                    checked={field.value} 
                    onCheckedChange={field.onChange} 
                  />
                )}
              />
              <Label htmlFor="is_active" className="cursor-pointer">User is Active</Label>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsEditDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={editForm.formState.isSubmitting || updateMutation.isPending}>
                {(editForm.formState.isSubmitting || updateMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Save Changes
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Deactivate/Delete User</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <strong>{selectedUser?.name}</strong>? This action cannot be undone. You may want to just mark them as inactive instead.
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
