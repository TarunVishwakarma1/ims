"use client";
import { useState } from "react";
import { ShoppingCart } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAddToCart } from "@/lib/use-add-to-cart";
import type { CartItem } from "@/lib/shop-types";

type Props = {
  item: Omit<CartItem, "qty">;
  qty: number;
  disabled?: boolean;
};

export function AddToCart({ item, qty, disabled }: Props) {
  const [pending, setPending] = useState(false);
  const addToCart = useAddToCart();

  async function onClick() {
    if (disabled || pending) return;
    setPending(true);
    try {
      await addToCart({ ...item, qty: 0 }, qty);
    } finally {
      setPending(false);
    }
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || pending}
      className={cn(
        "inline-flex items-center justify-center gap-2 h-11 px-6 rounded-xl",
        "bg-brand-600 text-white font-medium hover:bg-brand-700",
        "disabled:bg-muted disabled:cursor-not-allowed",
      )}
    >
      <ShoppingCart className="size-4" aria-hidden />
      {pending ? "Adding…" : "Add to cart"}
    </button>
  );
}
