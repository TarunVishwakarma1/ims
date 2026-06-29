"use client";

import Link from "next/link";
import { useCartStore, selectSubtotalPaise } from "@/lib/cart-store";
import { CartLine } from "@/components/cart/cart-line";
import { formatPaise } from "@/lib/format";
import { FREE_SHIP_THRESHOLD_PAISE } from "@/lib/shop-config";
import { toast } from "sonner";

export default function CartPage() {
  const items = useCartStore((s) => s.items);
  const subtotal = useCartStore(selectSubtotalPaise);
  const setQty = useCartStore((s) => s.setQty);
  const remove = useCartStore((s) => s.remove);

  if (items.length === 0) {
    return (
      <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-16 text-center">
        <h1 className="text-2xl font-semibold mb-3">Your cart is empty</h1>
        <p className="text-text-muted mb-6">Find something to start shopping.</p>
        <Link href="/" className="inline-block h-10 px-6 rounded bg-brand-600 text-white grid place-items-center hover:bg-brand-700">
          Browse shop
        </Link>
      </main>
    );
  }

  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8 grid gap-8 lg:grid-cols-[1fr_320px]">
      <section>
        <h1 className="text-2xl font-semibold mb-4">Cart ({items.length})</h1>
        <div className="border border-border rounded-lg px-4">
          {items.map((it) => (
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
          ))}
        </div>
      </section>
      <aside className="lg:sticky lg:top-20 self-start border border-border rounded-lg p-4 space-y-3">
        <h2 className="font-semibold">Summary</h2>
        <div className="flex justify-between text-sm">
          <span>Subtotal</span>
          <span>{formatPaise(subtotal)}</span>
        </div>
        {FREE_SHIP_THRESHOLD_PAISE > 0 && (
          <div className="space-y-1">
            {subtotal >= FREE_SHIP_THRESHOLD_PAISE ? (
              <p className="text-xs font-medium text-emerald-600">🎉 You’ve unlocked FREE delivery</p>
            ) : (
              <p className="text-xs text-emerald-600">
                Add {formatPaise(FREE_SHIP_THRESHOLD_PAISE - subtotal)} more for FREE delivery
              </p>
            )}
            <div className="h-1.5 rounded-full bg-brand-50 overflow-hidden">
              <div
                className="h-full bg-emerald-500 transition-all"
                style={{ width: `${Math.min(100, (subtotal / FREE_SHIP_THRESHOLD_PAISE) * 100)}%` }}
              />
            </div>
          </div>
        )}
        <p className="text-xs text-text-muted">GST + delivery calculated at checkout</p>
        <Link
          href="/checkout"
          className="block h-10 rounded bg-brand-600 text-white text-sm grid place-items-center hover:bg-brand-700"
        >
          Proceed to checkout
        </Link>
        <Link
          href="/"
          className="block h-10 rounded border border-border text-sm grid place-items-center hover:bg-brand-50"
        >
          Continue shopping
        </Link>
      </aside>
    </main>
  );
}
