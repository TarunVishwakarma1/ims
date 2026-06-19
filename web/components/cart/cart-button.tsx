'use client'

import { useQuery } from '@tanstack/react-query'
import { ShoppingBag } from 'lucide-react'

import { marketplaceApi } from '@/lib/api/marketplace'
import { useCartDrawer } from '@/lib/stores/cart-drawer-store'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'

export function CartButton() {
  const { can } = usePermission()
  const open = useCartDrawer((s) => s.setOpen)

  const { data: cart } = useQuery({
    queryKey: ['cart'],
    queryFn: marketplaceApi.getCart,
    retry: false,
    refetchOnWindowFocus: true,
    enabled: can(PERMISSIONS.ORDERS_CREATE),
  })

  if (!can(PERMISSIONS.ORDERS_CREATE)) return null

  const count = cart?.items?.reduce((s, it) => s + (it.quantity || 0), 0) ?? 0

  return (
    <button
      onClick={() => open(true)}
      aria-label="Open cart"
      className="relative inline-flex h-9 items-center gap-2 rounded-lg border border-zinc-200/70 bg-white/70 px-3 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-100 dark:border-zinc-800 dark:bg-zinc-900/50 dark:text-zinc-200 dark:hover:bg-zinc-800"
    >
      <ShoppingBag className="h-4 w-4" />
      <span className="hidden sm:inline">Cart</span>
      {count > 0 && (
        <span className="ml-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-pink-500 px-1.5 text-[10px] font-bold text-white tabular-nums shadow-sm">
          {count > 99 ? '99+' : count}
        </span>
      )}
    </button>
  )
}
