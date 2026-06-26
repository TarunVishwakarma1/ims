"use client";

import Link from "next/link";
import { X } from "lucide-react";
import { useCartStore, selectSubtotalPaise } from "@/lib/cart-store";
import { useCartDrawer } from "@/components/cart/cart-drawer-context";
import { CartLine } from "@/components/cart/cart-line";
import { formatPaise } from "@/lib/format";
import { toast } from "sonner";

export function CartDrawer() {
  const { isOpen, close } = useCartDrawer();
  const items = useCartStore((s) => s.items);
  const subtotal = useCartStore(selectSubtotalPaise);
  const setQty = useCartStore((s) => s.setQty);
  const remove = useCartStore((s) => s.remove);

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Cart"
      className="fixed inset-0 z-50 flex"
    >
      <button
        type="button"
        aria-label="Close cart"
        onClick={close}
        className="flex-1 bg-black/40"
      />
      <aside className="w-full sm:max-w-md bg-bg shadow-xl flex flex-col">
        <header className="h-14 px-4 border-b border-border flex items-center justify-between">
          <h2 className="font-semibold">Cart ({items.length})</h2>
          <button type="button" onClick={close} aria-label="Close" className="size-8 grid place-items-center rounded-full hover:bg-brand-50">
            <X className="size-4" />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto px-4">
          {items.length === 0 ? (
            <p className="text-text-muted text-center py-12">Your cart is empty.</p>
          ) : (
            items.map((it) => (
              <CartLine
                key={it.product_id}
                item={it}
                onQtyChange={(q) =>
                  setQty(it.product_id, q).catch(() => toast.error("Could not update cart"))
                }
                onRemove={() =>
                  remove(it.product_id).catch(() => toast.error("Could not remove item"))
                }
              />
            ))
          )}
        </div>
        {items.length > 0 && (
          <footer className="border-t border-border p-4 space-y-3">
            <div className="flex justify-between text-sm">
              <span>Subtotal</span>
              <span className="font-medium">{formatPaise(subtotal)}</span>
            </div>
            <p className="text-xs text-text-muted">GST + shipping at checkout</p>
            <div className="grid grid-cols-2 gap-2">
              <Link
                href="/cart"
                onClick={close}
                className="h-10 grid place-items-center rounded border border-border text-sm hover:bg-brand-50"
              >
                View cart
              </Link>
              <Link
                href="/checkout"
                onClick={close}
                className="h-10 grid place-items-center rounded bg-brand-600 text-white text-sm hover:bg-brand-700"
              >
                Checkout
              </Link>
            </div>
          </footer>
        )}
      </aside>
    </div>
  );
}
