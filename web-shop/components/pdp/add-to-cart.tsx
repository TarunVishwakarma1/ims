"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { ShoppingCart } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { useCartStore } from "@/lib/cart-store";
import type { CartItem } from "@/lib/shop-types";

type Props = {
  item: Omit<CartItem, "qty">;
  qty: number;
  disabled?: boolean;
};

export function AddToCart({ item, qty, disabled }: Props) {
  const [pending, setPending] = useState(false);
  const add = useCartStore((s) => s.add);
  const router = useRouter();

  async function onClick() {
    if (disabled || pending) return;
    setPending(true);
    try {
      await add({ ...item, qty: 0 }, qty);
      toast.success("Added to cart", {
        action: { label: "View cart", onClick: () => router.push("/cart") },
      });
    } catch {
      toast.error("Could not add to cart");
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
