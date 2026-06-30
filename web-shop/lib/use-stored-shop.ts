"use client";
import { useEffect, useState } from "react";
import { shopHref } from "./shop-path";

const KEY = "shop_slug";

/**
 * The last shop the customer entered (set by the /shops picker and on visit).
 * Read after mount to avoid hydration mismatch; null until known.
 *
 * Bridge for global pages (cart, orders) that show products but are not yet
 * shop-scoped — Phase 3 will carry the shop on the cart/order itself.
 */
export function useStoredShopSlug(): string | null {
  const [slug, setSlug] = useState<string | null>(null);
  useEffect(() => {
    try {
      setSlug(localStorage.getItem(KEY));
    } catch {
      /* ignore */
    }
  }, []);
  return slug;
}

/** Builds a product/browse href under the stored shop, or the picker if unknown. */
export function useStoredShopHref(): (sub?: string) => string {
  const slug = useStoredShopSlug();
  return (sub = "") => (slug ? shopHref(slug, sub) : "/shops");
}
