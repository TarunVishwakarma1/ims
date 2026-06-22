"use client";
import { useState } from "react";
import { ShoppingCart } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

type Props = { productSlug: string; qty: number; disabled?: boolean };

export function AddToCart({ productSlug, qty, disabled }: Props) {
  const [pending, setPending] = useState(false);

  async function onClick() {
    if (disabled || pending) return;
    setPending(true);
    try {
      const res = await fetch("/api/cart/add", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ slug: productSlug, qty }),
      });
      if (res.status === 501) {
        toast("Cart coming soon");
      } else if (res.ok) {
        toast.success("Added to cart");
      } else {
        toast.error("Could not add. Try again.");
      }
    } catch {
      toast.error("Could not add. Try again.");
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
