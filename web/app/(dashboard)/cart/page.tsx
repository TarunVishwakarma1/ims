'use client'

import { CartContent } from '@/components/cart/cart-content'

export default function CartPage() {
  return (
    <div className="space-y-4 max-w-3xl mx-auto h-[calc(100vh-10rem)]">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Shopping Cart</h2>
        <p className="text-muted-foreground">Review items before checkout.</p>
      </div>
      <div className="h-[calc(100%-4rem)] rounded-xl border bg-white dark:bg-zinc-950 overflow-hidden">
        <CartContent variant="page" />
      </div>
    </div>
  )
}
