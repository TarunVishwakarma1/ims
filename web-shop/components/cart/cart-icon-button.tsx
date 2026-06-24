"use client";

import { ShoppingCart } from "lucide-react";
import { useCartStore, selectItemCount } from "@/lib/cart-store";
import { useCartDrawer } from "@/components/cart/cart-drawer-context";

export function CartIconButton() {
  const count = useCartStore(selectItemCount);
  const { open } = useCartDrawer();
  return (
    <button
      type="button"
      onClick={open}
      aria-label={`Cart ${count > 0 ? `with ${count} items` : "empty"}`}
      className="relative size-10 grid place-items-center rounded-full hover:bg-brand-50"
    >
      <ShoppingCart className="size-5" />
      {count > 0 && (
        <span
          aria-hidden
          className="absolute -top-1 -right-1 min-w-5 h-5 px-1 rounded-full bg-brand-600 text-white text-xs grid place-items-center"
        >
          {count > 99 ? "99+" : count}
        </span>
      )}
    </button>
  );
}
