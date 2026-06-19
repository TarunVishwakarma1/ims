'use client'

import { useAuthStore } from '@/lib/stores/auth-store'
import { Building2 } from 'lucide-react'

import { CartButton } from '@/components/cart/cart-button'

export function Header() {
  const { organization } = useAuthStore()

  return (
    <header className="h-14 mb-3 rounded-2xl border border-white/40 bg-white/60 backdrop-blur-2xl backdrop-saturate-150 dark:border-white/10 dark:bg-zinc-900/50 flex items-center justify-between px-6 shadow-[0_1px_0_0_rgba(255,255,255,0.6)_inset,0_8px_30px_-12px_rgba(15,23,42,0.18)] dark:shadow-[0_1px_0_0_rgba(255,255,255,0.06)_inset,0_8px_30px_-12px_rgba(0,0,0,0.6)]">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Building2 className="h-4 w-4" />
        <span>{organization?.name ?? 'Loading…'}</span>
      </div>
      <div className="flex items-center gap-2">
        <CartButton />
      </div>
    </header>
  )
}
