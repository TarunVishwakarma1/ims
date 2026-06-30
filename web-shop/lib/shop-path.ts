// Storefront path helpers. All browse pages live under /s/<shop-slug>/…; the
// shop slug is the single source of which seller's catalog is shown. These are
// pure functions usable from both server and client components — the matching
// client hook lives in lib/use-shop-slug.ts.

const SHOP_SLUG_IN_PATH = /^\/s\/([a-z0-9-]+)(?:\/|$)/;

/** Extract the shop slug from a pathname, or null on global (non-shop) pages. */
export function parseShopSlug(pathname: string): string | null {
  const m = pathname.match(SHOP_SLUG_IN_PATH);
  return m ? m[1] : null;
}

/**
 * Build a storefront href. With a slug, `sub` is scoped under /s/<slug>
 * (e.g. shopHref("kirana", "/c/snacks") → "/s/kirana/c/snacks"); the bare
 * shop home is shopHref("kirana") → "/s/kirana". Without a slug it falls back
 * to the global path, or "/" for the empty home.
 */
export function shopHref(slug: string | null | undefined, sub = ""): string {
  if (!slug) return sub || "/";
  return `/s/${slug}${sub}`;
}

/**
 * Backend API base for a shop. Browser fetchers hit same-origin /api/shop/s/
 * /<slug> (rewritten to backend). A null slug uses the legacy default-org path
 * so global/un-scoped callers keep working.
 */
export function shopApiBase(slug?: string | null): string {
  return slug ? `/api/shop/s/${slug}` : "/api/shop";
}
