"use client";
import { usePathname } from "next/navigation";
import { parseShopSlug, shopHref } from "./shop-path";

/** Current shop slug derived from the URL, or null on global pages. */
export function useShopSlug(): string | null {
  return parseShopSlug(usePathname() || "");
}

/** Returns a builder bound to the current shop slug: h("/c/snacks"). */
export function useShopHref(): (sub?: string) => string {
  const slug = useShopSlug();
  return (sub = "") => shopHref(slug, sub);
}
