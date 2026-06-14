'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  LayoutDashboard,
  Package,
  Tags,
  Boxes,
  ShoppingCart,
  Users,
  Shield,
  LogOut
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/lib/stores/auth-store'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import { Button } from '@/components/ui/button'

export function Sidebar() {
  const pathname = usePathname()
  const { user, logout } = useAuthStore()
  const { can } = usePermission()

  const dynamicNavItems = [
    { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permission: null },
    { href: '/products',  label: 'Products',  icon: Package,         permission: PERMISSIONS.PRODUCTS_VIEW },
    { href: '/categories',label: 'Categories',icon: Tags,            permission: PERMISSIONS.CATEGORIES_VIEW },
    { href: '/inventory', label: 'Inventory', icon: Boxes,           permission: PERMISSIONS.INVENTORY_VIEW },
    { href: '/orders',    label: 'Orders',    icon: ShoppingCart,    permission: PERMISSIONS.ORDERS_VIEW },
    { href: '/users',     label: 'Users',     icon: Users,           permission: PERMISSIONS.USERS_VIEW },
    { href: '/roles',     label: 'Roles',     icon: Shield,          permission: PERMISSIONS.ROLES_MANAGE },
  ].filter(item => item.permission === null || can(item.permission))

  return (
    <aside className="w-64 border-r bg-white dark:bg-zinc-950 flex flex-col">
      <div className="h-16 flex items-center px-6 border-b">
        <span className="font-bold text-xl tracking-tight">IMS</span>
      </div>
      
      <nav className="flex-1 overflow-y-auto py-4 px-3 space-y-1">
        {dynamicNavItems.map((item) => {
          const isActive = pathname.startsWith(item.href)
          const Icon = item.icon
          
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors",
                isActive 
                  ? "bg-primary text-primary-foreground" 
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              <Icon className="w-4 h-4" />
              {item.label}
            </Link>
          )
        })}
      </nav>

      <div className="p-4 border-t">
        <div className="flex items-center justify-between">
          <div className="flex flex-col truncate pr-2">
            <span className="text-sm font-medium truncate">{user?.name || 'User'}</span>
            <span className="text-xs text-muted-foreground truncate">{user?.email || ''}</span>
          </div>
          <Button variant="ghost" size="icon" onClick={logout} title="Log out">
            <LogOut className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </aside>
  )
}
