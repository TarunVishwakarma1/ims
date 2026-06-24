"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { Cart, CartItem } from "@/lib/shop-types";
import { addCartItem, mergeCart, removeCartItem } from "@/lib/shop-api";

const STORAGE_KEY = "shop_cart_v1";

type CartStore = {
  items: CartItem[];
  serverHydrated: boolean;
  loading: boolean;
  add: (item: CartItem, qty: number) => Promise<void>;
  setQty: (productID: string, qty: number) => Promise<void>;
  remove: (productID: string) => Promise<void>;
  clear: () => void;
  hydrateFromServer: (cart: Cart) => void;
  mergeOnLogin: () => Promise<void>;
};

function clamp(qty: number, max: number): number {
  if (qty <= 0) return 0;
  if (max > 0 && qty > max) return max;
  return qty;
}

export const useCartStore = create<CartStore>()(
  persist(
    (set, get) => ({
      items: [],
      serverHydrated: false,
      loading: false,

      add: async (item, qty) => {
        const existing = get().items.find((i) => i.product_id === item.product_id);
        const nextQty = clamp((existing?.qty ?? 0) + qty, item.max_qty);
        if (nextQty <= 0) return;

        const prev = get().items;
        const next = existing
          ? prev.map((i) => (i.product_id === item.product_id ? { ...i, qty: nextQty } : i))
          : [...prev, { ...item, qty: nextQty }];

        set({ items: next });

        if (get().serverHydrated) {
          try {
            const cart = await addCartItem(item.product_id, nextQty);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev });
            throw e;
          }
        }
      },

      setQty: async (productID, qty) => {
        const prev = get().items;
        const existing = prev.find((i) => i.product_id === productID);
        if (!existing) return;
        const next = clamp(qty, existing.max_qty);

        if (next <= 0) {
          await get().remove(productID);
          return;
        }

        set({ items: prev.map((i) => (i.product_id === productID ? { ...i, qty: next } : i)) });

        if (get().serverHydrated) {
          try {
            const cart = await addCartItem(productID, next);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev });
            throw e;
          }
        }
      },

      remove: async (productID) => {
        const prev = get().items;
        set({ items: prev.filter((i) => i.product_id !== productID) });
        if (get().serverHydrated) {
          try {
            const cart = await removeCartItem(productID);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev });
            throw e;
          }
        }
      },

      clear: () => {
        set({ items: [] });
      },

      hydrateFromServer: (cart) => {
        set({ items: cart.items, serverHydrated: true });
      },

      mergeOnLogin: async () => {
        const local = get().items.map((i) => ({ product_id: i.product_id, qty: i.qty }));
        try {
          const cart = await mergeCart(local);
          set({ items: cart.items, serverHydrated: true });
        } catch (e) {
          // Keep local cart; let UI surface failure.
          throw e;
        }
      },
    }),
    {
      name: STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ items: s.items }), // never persist serverHydrated/loading
      version: 1,
    },
  ),
);

export function selectItemCount(s: CartStore): number {
  return s.items.reduce((sum, i) => sum + i.qty, 0);
}

export function selectSubtotalPaise(s: CartStore): number {
  return s.items.reduce((sum, i) => sum + i.qty * i.unit_price_paise, 0);
}
