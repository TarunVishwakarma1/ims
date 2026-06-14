import { api } from './client'

export interface Permission {
  id: string
  resource: string
  action: string
  description: string
}

export interface Role {
  id: string
  name: string
  description: string
  is_system: boolean
  created_at: string
  updated_at: string
  permissions: Permission[]
}

export const rolesApi = {
  listRoles: () => api.get('roles').json<Role[]>(),
  
  listPermissions: () => api.get('permissions').json<Permission[]>(),
  
  createRole: (data: { name: string; description: string }) => 
    api.post('roles', { json: data }).json<Role>(),

  updateRole: (id: string, data: { name: string; description: string }) =>
    api.put(`roles/${id}`, { json: data }).json<{ message: string }>(),

  deleteRole: (id: string) =>
    api.delete(`roles/${id}`).json<{ message: string }>(),
    
  updatePermissions: (roleId: string, permissionIds: string[]) => 
    api.put(`roles/${roleId}/permissions`, {
      json: { permission_ids: permissionIds },
    }).json<{ message: string }>(),
    
  reloadCache: () => 
    api.post('roles/reload').json<{ message: string }>(),
}
