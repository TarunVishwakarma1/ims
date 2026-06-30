"use client";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { useCartStore } from "@/lib/cart-store";
import { useShopSlug } from "@/lib/use-shop-slug";
import { CartShopConflictError } from "@/lib/shop-api";
import type { CartItem } from "@/lib/shop-types";

/**
 * Adds an item to the single-shop cart from within a storefront. On a cross-shop
 * conflict (the cart holds items from another shop) it surfaces a "start a new
 * cart?" toast whose action replaces the cart with this shop (Zomato-style).
 */
export function useAddToCart(): (item: CartItem, qty?: number) => Promise<void> {
  const add = useCartStore((s) => s.add);
  const replaceCart = useCartStore((s) => s.replaceCart);
  const shop = useShopSlug();
  const router = useRouter();

  return async function addToCart(item, qty = 1) {
    if (!shop) {
      toast.error("Open a shop to add items");
      return;
    }
    try {
      await add(item, qty, shop);
      toast.success("Added to cart", {
        description: item.name,
        action: { label: "View cart", onClick: () => router.push("/cart") },
      });
    } catch (e) {
      if (e instanceof CartShopConflictError) {
        toast(`Your cart has items from ${e.currentName}.`, {
          description: "Start a new cart with this shop?",
          duration: 8000,
          action: {
            label: "Replace",
            onClick: () => {
              void replaceCart(item, qty, shop)
                .then(() => toast.success("Started a new cart", { description: item.name }))
                .catch(() => toast.error("Could not switch cart"));
            },
          },
        });
      } else {
        toast.error("Could not add to cart");
      }
    }
  };
}
