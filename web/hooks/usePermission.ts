import { useAuthStore } from '@/lib/stores/auth-store'
import { hasPermission, type Permission } from '@/lib/rbac'

export function usePermission() {
  const { user } = useAuthStore()
  return {
    can: (perm: Permission) => hasPermission(user?.role, perm),
    role: user?.role,
  }
}
