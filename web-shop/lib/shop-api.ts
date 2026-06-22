import type { FeedPage } from "@/lib/shop-types";

/**
 * Browser-side fetch for feed pages. Uses same-origin /api/shop/feed which
 * Next rewrites to backend per next.config.ts. Cookies forwarded automatically.
 */
export async function fetchFeedPage(cursor?: string, limit = 24): Promise<FeedPage> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set("cursor", cursor);
  const res = await fetch(`/api/shop/feed?${params.toString()}`, {
    credentials: "include",
  });
  if (!res.ok) throw new Error(`feed fetch failed: ${res.status}`);
  return res.json();
}
