'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { inventoryApi } from '@/lib/api/inventory'
import { useEventStream } from '@/hooks/useEventStream'
import { useAuthStore } from '@/lib/stores/auth-store'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS, type Permission } from '@/lib/rbac'
import { cn } from '@/lib/utils'

import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
  Undo2,
  Mail,
  ScrollText,
  KeyRound,
  ChevronRight,
  Settings,
  ShieldCheck,
  ChevronsUpDown,
  Webhook,
  CreditCard,
  Ticket,
  type LucideIcon,
} from 'lucide-react'

// ── Nav model ──────────────────────────────────────────────────────────────

interface NavItem {
  href: string
  label: string
  icon: LucideIcon
  permission?: Permission | null
}

interface NavGroup {
  id: string
  label: string
  icon: LucideIcon
  items: NavItem[]
}

// Flat items live outside groups (always visible if user has permission).
const FLAT_ITEMS: NavItem[] = [
  { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permission: null },
]

const GROUPS: NavGroup[] = [
  {
    id: 'catalog',
    label: 'Catalog',
    icon: Package,
    items: [
      { href: '/products',   label: 'Products',   icon: Package, permission: PERMISSIONS.PRODUCTS_VIEW },
      { href: '/categories', label: 'Categories', icon: Tags,    permission: PERMISSIONS.CATEGORIES_VIEW },
      { href: '/inventory',  label: 'Inventory',  icon: Boxes,   permission: PERMISSIONS.INVENTORY_VIEW },
    ],
  },
  {
    id: 'sales',
    label: 'Sales',
    icon: ShoppingCart,
    items: [
      { href: '/orders',      label: 'Orders',      icon: ShoppingCart, permission: PERMISSIONS.ORDERS_VIEW },
      { href: '/payments',    label: 'Payments',    icon: CreditCard,   permission: PERMISSIONS.PAYMENTS_VIEW },
      { href: '/coupons',     label: 'Coupons',     icon: Ticket,       permission: PERMISSIONS.COUPONS_VIEW },
      { href: '/returns',     label: 'Returns',     icon: Undo2,        permission: PERMISSIONS.ORDERS_VIEW },
      { href: '/partners',    label: 'Partners',    icon: Handshake,    permission: PERMISSIONS.ORDERS_VIEW },
      { href: '/marketplace', label: 'Marketplace', icon: Store,        permission: PERMISSIONS.PRODUCTS_VIEW },
      { href: '/cart',        label: 'Cart',        icon: ShoppingBag,  permission: PERMISSIONS.ORDERS_CREATE },
    ],
  },
  {
    id: 'admin',
    label: 'Administration',
    icon: Shield,
    items: [
      { href: '/users',     label: 'Users',     icon: Users,  permission: PERMISSIONS.USERS_VIEW },
      { href: '/roles',     label: 'Roles',     icon: Shield, permission: PERMISSIONS.ROLES_MANAGE },
      { href: '/locations', label: 'Locations', icon: MapPin, permission: PERMISSIONS.LOCATIONS_MANAGE },
    ],
  },
  {
    id: 'ops',
    label: 'Operations',
    icon: Webhook,
    items: [
      { href: '/webhooks/events',  label: 'Webhook events',  icon: Inbox,      permission: PERMISSIONS.WEBHOOKS_VIEW },
      { href: '/webhooks/dlq',     label: 'Webhook DLQ',     icon: Inbox,      permission: PERMISSIONS.WEBHOOKS_VIEW },
      { href: '/webhooks/secrets', label: 'Webhook secrets', icon: KeyRound,   permission: PERMISSIONS.WEBHOOKS_MANAGE },
      { href: '/notifications',    label: 'Notifications',   icon: Mail,       permission: PERMISSIONS.NOTIFICATIONS_VIEW },
      { href: '/audit',            label: 'Audit log',       icon: ScrollText, permission: PERMISSIONS.AUDIT_VIEW },
    ],
  },
]

// ── Component ──────────────────────────────────────────────────────────────

export function Sidebar() {
  const pathname = usePathname()
  const { user, logout } = useAuthStore()
  const { can } = usePermission()
  const queryClient = useQueryClient()

  const allowed = (perm: Permission | null | undefined) => perm == null || can(perm)
  const visibleGroups = GROUPS
    .map(g => ({ ...g, items: g.items.filter(i => allowed(i.permission)) }))
    .filter(g => g.items.length > 0)

  // Persist per-group open state. Default: any group containing the active
  // page is open; others closed. localStorage takes precedence after first
  // mount so the user's collapse choices stick.
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({})
  useEffect(() => {
    if (typeof window === 'undefined') return
    const stored = window.localStorage.getItem('ims:sidebar:groups')
    const fromStorage: Record<string, boolean> = stored ? JSON.parse(stored) : {}
    const initial: Record<string, boolean> = {}
    for (const g of GROUPS) {
      if (g.id in fromStorage) initial[g.id] = fromStorage[g.id]
      else initial[g.id] = g.items.some(i => pathname.startsWith(i.href))
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOpenGroups(initial)
    // Intentionally pathname-only on mount — we don't want to forcibly
    // expand groups every nav, only sync stored state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const toggleGroup = (id: string) => {
    setOpenGroups(prev => {
      const next = { ...prev, [id]: !prev[id] }
      if (typeof window !== 'undefined') {
        window.localStorage.setItem('ims:sidebar:groups', JSON.stringify(next))
      }
      return next
    })
  }

  // Live low-stock count drives the badge next to the Inventory item.
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
      const data = evt.data as { direction?: string } | undefined
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
      toast.warning(`Low stock: ${data?.quantity ?? 0} units remaining`, {
        action: { label: 'View', onClick: () => window.location.assign('/inventory?lowOnly=1') },
      })
      queryClient.invalidateQueries({ queryKey: ['inventory', 'low-stock'] })
    } else if (evt.type === 'inventory.updated') {
      queryClient.invalidateQueries({ queryKey: ['inventory', 'low-stock'] })
    }
  })

  // ── Renderers ────────────────────────────────────────────────────────────

  const renderItem = (item: NavItem, depth: 'top' | 'nested' = 'nested') => {
    const isActive = pathname.startsWith(item.href)
    const Icon = item.icon
    const isInventory = item.href === '/inventory'
    return (
      <Link
        key={item.href}
        href={item.href}
        className={cn(
          'group relative flex items-center gap-3 rounded-md text-sm font-medium transition-all',
          depth === 'nested' ? 'ml-3 pl-2 pr-2 py-1.5' : 'px-3 py-2',
          isActive
            ? 'bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900 shadow-sm'
            : 'text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/60 dark:hover:text-zinc-100',
        )}
      >
        {/* Active indicator strip on nested items */}
        {depth === 'nested' && isActive && (
          <span className="absolute -left-3 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-r bg-zinc-900 dark:bg-zinc-100" />
        )}
        <Icon className="h-4 w-4 shrink-0" />
        <span className="flex-1 truncate">{item.label}</span>
        {isInventory && lowStockCount > 0 && (
          <Badge
            variant="outline"
            className={cn(
              'h-5 px-1.5 text-[10px] font-semibold',
              isActive
                ? 'bg-white/20 text-white border-white/30 dark:bg-zinc-900/20 dark:text-zinc-900 dark:border-zinc-900/30'
                : 'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-900',
            )}
          >
            {lowStockCount}
          </Badge>
        )}
      </Link>
    )
  }

  const renderGroup = (group: typeof visibleGroups[number]) => {
    const isOpen = openGroups[group.id] ?? false
    const hasActive = group.items.some(i => pathname.startsWith(i.href))
    const Icon = group.icon
    return (
      <Collapsible key={group.id} open={isOpen} onOpenChange={() => toggleGroup(group.id)}>
        <CollapsibleTrigger
          className={cn(
            'group flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
            'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800/60',
            hasActive && !isOpen && 'text-zinc-900 dark:text-zinc-100',
          )}
        >
          <Icon className="h-4 w-4 shrink-0" />
          <span className="flex-1 text-left">{group.label}</span>
          <ChevronRight
            className={cn(
              'h-3.5 w-3.5 text-zinc-400 transition-transform duration-200',
              isOpen && 'rotate-90',
            )}
          />
        </CollapsibleTrigger>
        <CollapsibleContent className="overflow-hidden data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:slide-in-from-top-1 data-[state=closed]:animate-out data-[state=closed]:fade-out-0">
          <div className="mt-1 mb-1 space-y-0.5 border-l border-zinc-200/70 dark:border-zinc-800/70 ml-5">
            {group.items.map(item => renderItem(item))}
          </div>
        </CollapsibleContent>
      </Collapsible>
    )
  }

  const initials = (user?.name ?? user?.email ?? '?')
    .split(' ')
    .map(s => s[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()

  return (
    <aside
      className={cn(
        // Floating shape: detached from edges, rounded, with a soft shadow.
        'relative z-10 m-3 flex w-64 flex-col rounded-2xl',
        // Glass surface: semi-transparent + backdrop blur. Supports-backdrop
        // fallback covers older browsers (Firefox without flag).
        'border border-white/40 bg-white/60 backdrop-blur-2xl backdrop-saturate-150',
        'dark:border-white/10 dark:bg-zinc-900/50',
        // Inner highlight + drop shadow give the floating impression.
        'shadow-[0_1px_0_0_rgba(255,255,255,0.6)_inset,0_8px_30px_-12px_rgba(15,23,42,0.18)]',
        'dark:shadow-[0_1px_0_0_rgba(255,255,255,0.06)_inset,0_8px_30px_-12px_rgba(0,0,0,0.6)]',
        // Fallback for browsers without backdrop-filter — opaque pane.
        'supports-[not(backdrop-filter:blur(1px))]:bg-white supports-[not(backdrop-filter:blur(1px))]:dark:bg-zinc-900',
      )}
    >
      {/* Brand header */}
      <div className="flex h-16 items-center gap-2.5 border-b border-white/30 dark:border-white/10 px-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 via-purple-500 to-pink-500 text-white shadow-sm">
          <Boxes className="h-5 w-5" />
        </div>
        <div className="flex flex-col leading-tight">
          <span className="text-base font-bold tracking-tight">IMS</span>
          <span className="text-[10px] uppercase tracking-wider text-zinc-500">
            Inventory Suite
          </span>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-3 py-4 space-y-1">
        {FLAT_ITEMS.filter(i => allowed(i.permission)).map(i => renderItem(i, 'top'))}
        {visibleGroups.length > 0 && (
          <div className="mt-3 mb-1 px-3 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
            Workspace
          </div>
        )}
        {visibleGroups.map(renderGroup)}
      </nav>

      {/* User card + menu */}
      <div className="border-t border-white/30 dark:border-white/10 p-3">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              className={cn(
                'flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left transition-colors',
                'hover:bg-zinc-100 dark:hover:bg-zinc-800/60',
              )}
            >
              <Avatar className="h-9 w-9">
                <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-xs font-semibold text-white">
                  {initials}
                </AvatarFallback>
              </Avatar>
              <div className="flex flex-1 flex-col leading-tight overflow-hidden">
                <span className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {user?.name ?? 'User'}
                </span>
                <span className="truncate text-xs text-zinc-500">{user?.email}</span>
              </div>
              <ChevronsUpDown className="h-4 w-4 text-zinc-400" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="end" className="w-56">
            <DropdownMenuLabel className="font-normal">
              <div className="flex flex-col space-y-1">
                <p className="text-sm font-semibold">{user?.name}</p>
                <p className="text-xs text-muted-foreground truncate">{user?.email}</p>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/settings" className="cursor-pointer">
                <Settings className="mr-2 h-4 w-4" />
                Settings
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link href="/settings/2fa" className="cursor-pointer">
                <ShieldCheck className="mr-2 h-4 w-4" />
                Two-factor auth
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={logout}
              className="cursor-pointer text-red-600 focus:text-red-600"
            >
              <LogOut className="mr-2 h-4 w-4" />
              Log out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </aside>
  )
}
