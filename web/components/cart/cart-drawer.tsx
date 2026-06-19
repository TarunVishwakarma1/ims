'use client'

import { ShoppingBag } from 'lucide-react'

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import { useCartDrawer } from '@/lib/stores/cart-drawer-store'
import { CartContent } from './cart-content'

export function CartDrawer() {
  const { open, setOpen } = useCartDrawer()

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetContent
        side="right"
        className="w-full sm:max-w-md p-0 flex flex-col gap-0"
      >
        <SheetHeader className="px-4 py-3 border-b bg-white dark:bg-zinc-950 gap-0.5">
          <SheetTitle className="text-sm font-semibold flex items-center gap-2">
            <ShoppingBag className="h-4 w-4" />
            Your cart
          </SheetTitle>
          <SheetDescription className="text-[11px]">
            Items grouped by seller. Coupons apply per seller.
          </SheetDescription>
        </SheetHeader>
        <CartContent variant="drawer" />
      </SheetContent>
    </Sheet>
  )
}
