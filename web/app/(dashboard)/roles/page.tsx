'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Loader2, Plus, RefreshCw, Save } from 'lucide-react'

import { rolesApi } from '@/lib/api/roles'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Badge } from '@/components/ui/badge'

const createRoleSchema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  description: z.string().optional(),
})
type CreateRoleFormValues = z.infer<typeof createRoleSchema>

export default function RolesPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [pendingPermissions, setPendingPermissions] = useState<Record<string, string[]>>({})

  const { data: rawRoles, isLoading: isLoadingRoles } = useQuery({
    queryKey: ['roles'],
    queryFn: rolesApi.listRoles,
    enabled: can(PERMISSIONS.ROLES_MANAGE),
  })

  const { data: rawPermissions, isLoading: isLoadingPermissions } = useQuery({
    queryKey: ['permissions'],
    queryFn: rolesApi.listPermissions,
    enabled: can(PERMISSIONS.ROLES_MANAGE),
  })

  const roles = rawRoles ?? []
  const permissions = rawPermissions ?? []

  const createMutation = useMutation({
    mutationFn: rolesApi.createRole,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
      setIsCreateDialogOpen(false)
    },
  })

  const updatePermsMutation = useMutation({
    mutationFn: ({ roleId, permissionIds }: { roleId: string, permissionIds: string[] }) => 
      rolesApi.updatePermissions(roleId, permissionIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })

  const reloadCacheMutation = useMutation({
    mutationFn: rolesApi.reloadCache,
  })

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm<CreateRoleFormValues>({
    resolver: zodResolver(createRoleSchema),
    defaultValues: {
      name: '',
      description: '',
    }
  })

  if (!can(PERMISSIONS.ROLES_MANAGE)) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <p className="text-muted-foreground">You do not have permission to view this page.</p>
      </div>
    )
  }

  const onCreateSubmit = (data: CreateRoleFormValues) => {
    createMutation.mutate({
      name: data.name,
      description: data.description || '',
    })
  }

  // Handle local toggle before save
  const togglePermission = (roleId: string, permId: string) => {
    const role = roles.find(r => r.id === roleId)
    if (!role) return

    // Initialize from existing if not yet modified
    const current = pendingPermissions[roleId] ?? role.permissions.map(p => p.id)
    
    let next: string[]
    if (current.includes(permId)) {
      next = current.filter(id => id !== permId)
    } else {
      next = [...current, permId]
    }
    
    setPendingPermissions(prev => ({ ...prev, [roleId]: next }))
  }

  const handleSavePermissions = (roleId: string) => {
    const permissionIds = pendingPermissions[roleId]
    if (permissionIds) {
      updatePermsMutation.mutate({ roleId, permissionIds }, {
        onSuccess: () => {
          // Clear pending state after successful save
          setPendingPermissions(prev => {
            const next = { ...prev }
            delete next[roleId]
            return next
          })
          // Auto-reload cache after a permission save
          reloadCacheMutation.mutate()
        }
      })
    }
  }

  const hasUnsavedChanges = (roleId: string) => {
    return pendingPermissions[roleId] !== undefined
  }

  const isLoading = isLoadingRoles || isLoadingPermissions

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Roles & Permissions</h2>
          <p className="text-muted-foreground">Manage system roles and their access levels.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => reloadCacheMutation.mutate()} disabled={reloadCacheMutation.isPending}>
            {reloadCacheMutation.isPending ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <RefreshCw className="w-4 h-4 mr-2" />}
            Reload Cache
          </Button>
          <Button onClick={() => {
            reset({ name: '', description: '' })
            setIsCreateDialogOpen(true)
          }}>
            <Plus className="mr-2 h-4 w-4" /> Add Role
          </Button>
        </div>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950 overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">Role</TableHead>
              {permissions.map(p => (
                <TableHead key={p.id} className="text-center min-w-[120px]">
                  <div className="font-medium">{p.resource}</div>
                  <div className="text-xs text-muted-foreground font-normal">{p.action}</div>
                </TableHead>
              ))}
              <TableHead className="text-right min-w-[120px]">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={permissions.length + 2} className="h-24 text-center">
                  <Loader2 className="w-6 h-6 animate-spin mx-auto text-muted-foreground" />
                </TableCell>
              </TableRow>
            ) : roles.length > 0 ? (
              roles.map((role) => {
                // Use pending permissions if available, otherwise fallback to saved permissions
                const activePerms = pendingPermissions[role.id] ?? role.permissions.map(p => p.id)
                const isDirty = hasUnsavedChanges(role.id)

                return (
                  <TableRow key={role.id}>
                    <TableCell>
                      <div className="font-medium">{role.name}</div>
                      {role.is_system && <Badge variant="secondary" className="mt-1 text-[10px]">System</Badge>}
                    </TableCell>
                    {permissions.map(p => (
                      <TableCell key={p.id} className="text-center">
                        <Checkbox 
                          checked={activePerms.includes(p.id)}
                          onCheckedChange={() => togglePermission(role.id, p.id)}
                          disabled={role.name === 'admin'} // Admin role is hardcoded full access conceptually, though DB might reflect it
                        />
                      </TableCell>
                    ))}
                    <TableCell className="text-right">
                      {isDirty && (
                        <Button 
                          size="sm" 
                          onClick={() => handleSavePermissions(role.id)}
                          disabled={updatePermsMutation.isPending}
                        >
                          {updatePermsMutation.isPending ? (
                            <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                          ) : (
                            <Save className="w-4 h-4 mr-1" />
                          )}
                          Save
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })
            ) : (
              <TableRow>
                <TableCell colSpan={permissions.length + 2} className="h-24 text-center text-muted-foreground">
                  No roles found.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Role</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onCreateSubmit)} className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="name">Role Name (e.g. accountant)</Label>
              <Input id="name" {...register('name')} />
              {errors.name && <p className="text-xs text-red-500">{errors.name.message}</p>}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="description">Description</Label>
              <Textarea id="description" {...register('description')} />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting || createMutation.isPending}>
                {(isSubmitting || createMutation.isPending) && (
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                )}
                Create Role
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
