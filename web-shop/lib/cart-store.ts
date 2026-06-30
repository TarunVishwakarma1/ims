"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { Cart, CartItem } from "@/lib/shop-types";
import {
  addCartItem,
  mergeCart,
  removeCartItem,
  CartShopConflictError,
} from "@/lib/shop-api";

const STORAGE_KEY = "shop_cart_v1";

type CartStore = {
  items: CartItem[];
  // The single shop the cart is bound to (Zomato-style). null when empty.
  shopSlug: string | null;
  shopName: string | null;
  serverHydrated: boolean;
  add: (item: CartItem, qty: number, shopSlug: string, shopName?: string) => Promise<void>;
  replaceCart: (item: CartItem, qty: number, shopSlug: string, shopName?: string) => Promise<void>;
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
      shopSlug: null,
      shopName: null,
      serverHydrated: false,

      add: async (item, qty, shopSlug, shopName) => {
        const { items: prev, shopSlug: boundSlug, shopName: boundName, serverHydrated } = get();

        // Guest (not server-backed): enforce the single-shop rule locally so the
        // UI can prompt before we ever reach the server. The server enforces it
        // too and answers 409 once hydrated.
        if (!serverHydrated && prev.length > 0 && boundSlug && boundSlug !== shopSlug) {
          throw new CartShopConflictError(boundSlug, boundName ?? boundSlug);
        }

        const existing = prev.find((i) => i.product_id === item.product_id);
        const nextQty = clamp((existing?.qty ?? 0) + qty, item.max_qty);
        if (nextQty <= 0) return;

        const next = existing
          ? prev.map((i) => (i.product_id === item.product_id ? { ...i, qty: nextQty } : i))
          : [...prev, { ...item, qty: nextQty }];

        set({ items: next, shopSlug, shopName: shopName ?? boundName });

        if (serverHydrated) {
          try {
            const cart = await addCartItem(shopSlug, item.product_id, nextQty);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev, shopSlug: boundSlug, shopName: boundName });
            throw e;
          }
        }
      },

      // Switch the cart to a new shop, discarding whatever it held. Used after
      // the customer confirms "start a new cart?" on a cross-shop add.
      replaceCart: async (item, qty, shopSlug, shopName) => {
        const prev = get();
        const nextQty = clamp(qty, item.max_qty);
        if (nextQty <= 0) return;

        set({ items: [{ ...item, qty: nextQty }], shopSlug, shopName: shopName ?? null });

        if (prev.serverHydrated) {
          try {
            const cart = await addCartItem(shopSlug, item.product_id, nextQty, true);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev.items, shopSlug: prev.shopSlug, shopName: prev.shopName });
            throw e;
          }
        }
      },

      setQty: async (productID, qty) => {
        const { items: prev, shopSlug } = get();
        const existing = prev.find((i) => i.product_id === productID);
        if (!existing) return;
        const next = clamp(qty, existing.max_qty);

        if (next <= 0) {
          await get().remove(productID);
          return;
        }

        set({ items: prev.map((i) => (i.product_id === productID ? { ...i, qty: next } : i)) });

        if (get().serverHydrated && shopSlug) {
          try {
            const cart = await addCartItem(shopSlug, productID, next);
            get().hydrateFromServer(cart);
          } catch (e) {
            set({ items: prev });
            throw e;
          }
        }
      },

      remove: async (productID) => {
        const prev = get().items;
        const next = prev.filter((i) => i.product_id !== productID);
        // Dropping the last item unbinds the shop.
        set(next.length === 0 ? { items: next, shopSlug: null, shopName: null } : { items: next });
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
        set({ items: [], shopSlug: null, shopName: null });
      },

      hydrateFromServer: (cart) => {
        set({
          items: cart.items,
          shopSlug: cart.shop?.slug ?? null,
          shopName: cart.shop?.name ?? null,
          serverHydrated: true,
        });
      },

      mergeOnLogin: async () => {
        const { items, shopSlug } = get();
        // Nothing to merge, or no shop bound — just adopt server-backed mode.
        if (items.length === 0 || !shopSlug) {
          set({ serverHydrated: true });
          return;
        }
        const local = items.map((i) => ({ product_id: i.product_id, qty: i.qty }));
        try {
          const cart = await mergeCart(shopSlug, local);
          get().hydrateFromServer(cart);
        } catch (e) {
          // Keep local cart; let UI surface failure.
          throw e;
        }
      },
    }),
    {
      name: STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ items: s.items, shopSlug: s.shopSlug, shopName: s.shopName }),
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
