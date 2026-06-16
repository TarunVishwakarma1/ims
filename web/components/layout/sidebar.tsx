'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { inventoryApi } from '@/lib/api/inventory'
import { useEventStream } from '@/hooks/useEventStream'
import { Badge } from '@/components/ui/badge'
import {
  LayoutDashboard,
  Package,
  Tags,
  Boxes,
  ShoppingCart,
  Users,
  Shield,
  LogOut,
  Store,
  ShoppingBag,
  MapPin,
  Handshake,
  Inbox,
  Undo2
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
  const queryClient = useQueryClient()

  // Live count of low-stock items — drives the badge next to Inventory.
  // Background refresh on inventory.* events so the count stays current
  // across all pages, not just /inventory.
  const lowStockQ = useQuery({
    queryKey: ['inventory', 'low-stock'],
    queryFn: inventoryApi.listLowStock,
    enabled: can(PERMISSIONS.INVENTORY_VIEW),
    refetchOnWindowFocus: true,
    staleTime: 30_000,
  })
  const lowStockCount = lowStockQ.data?.length ?? 0

  useEventStream(['inventory', 'return'], (evt) => {
    if (evt.type === 'return.created') {
      const data = evt.data as { direction?: string; order_id?: string } | undefined
      if (data?.direction === 'incoming') {
        toast.warning('New return request from a buyer', {
          action: { label: 'View', onClick: () => window.location.assign('/returns') },
        })
        queryClient.invalidateQueries({ queryKey: ['returns'] })
      }
      return
    }
    if (evt.type === 'inventory.low_stock') {
      const data = evt.data as { quantity?: number; threshold?: number } | undefined
      // Global toast — fires regardless of which page you're on.
      toast.warning(`Low stock: ${data?.quantity ?? 0} units remaining`, {
        action: { label: 'View', onClick: () => window.location.assign('/inventory?lowOnly=1') },
      })
      queryClient.invalidateQueries({ queryKey: ['inventory', 'low-stock'] })
    } else if (evt.type === 'inventory.updated') {
      queryClient.invalidateQueries({ queryKey: ['inventory', 'low-stock'] })
    }
  })

  const dynamicNavItems = [
    { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permission: null },
    { href: '/products',  label: 'Products',  icon: Package,         permission: PERMISSIONS.PRODUCTS_VIEW },
    { href: '/categories',label: 'Categories',icon: Tags,            permission: PERMISSIONS.CATEGORIES_VIEW },
    { href: '/inventory', label: 'Inventory', icon: Boxes,           permission: PERMISSIONS.INVENTORY_VIEW },
    { href: '/orders',    label: 'Orders',    icon: ShoppingCart,    permission: PERMISSIONS.ORDERS_VIEW },
    { href: '/partners',  label: 'Partners',  icon: Handshake,       permission: PERMISSIONS.ORDERS_VIEW },
    { href: '/returns',   label: 'Returns',   icon: Undo2,           permission: PERMISSIONS.ORDERS_VIEW },
    { href: '/users',     label: 'Users',     icon: Users,           permission: PERMISSIONS.USERS_VIEW },
    { href: '/roles',     label: 'Roles',     icon: Shield,          permission: PERMISSIONS.ROLES_MANAGE },
    { href: '/marketplace', label: 'Marketplace', icon: Store, permission: PERMISSIONS.PRODUCTS_VIEW },
    { href: '/cart',        label: 'Cart',        icon: ShoppingBag, permission: PERMISSIONS.ORDERS_CREATE },
    { href: '/locations',   label: 'Locations',   icon: MapPin, permission: PERMISSIONS.LOCATIONS_MANAGE },
    { href: '/webhooks/dlq', label: 'Webhook DLQ', icon: Inbox, permission: PERMISSIONS.USERS_DELETE },
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
              <span className="flex-1">{item.label}</span>
              {item.href === '/inventory' && lowStockCount > 0 && (
                <Badge
                  variant="outline"
                  className={cn(
                    "px-1.5 py-0 h-5 text-[10px] font-semibold",
                    isActive
                      ? "bg-white/20 text-white border-white/30"
                      : "bg-amber-100 text-amber-800 border-amber-200"
                  )}
                >
                  {lowStockCount}
                </Badge>
              )}
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
