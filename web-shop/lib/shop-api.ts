import type { FeedPage } from "@/lib/shop-types";
import type {
  ProductCard,
  ProductListQuery,
  ProductListResult,
} from "@/lib/shop-types";

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

function buildProductParams(q: ProductListQuery): URLSearchParams {
  const params = new URLSearchParams();
  if (q.category) params.set("category", q.category);
  if (q.search) params.set("search", q.search);
  if (q.sort) params.set("sort", q.sort);
  if (q.in_stock) params.set("in_stock", "true");
  if (q.cursor) params.set("cursor", q.cursor);
  params.set("limit", String(q.limit ?? 24));
  return params;
}

/**
 * Browser-side product list fetch. Same-origin /api/shop/products. Throws on
 * non-2xx so the caller decides whether to surface an error (used by
 * InfiniteGrid loadMore).
 */
export async function fetchProductList(
  q: ProductListQuery,
  signal?: AbortSignal,
): Promise<ProductListResult> {
  const res = await fetch(`/api/shop/products?${buildProductParams(q).toString()}`, {
    credentials: "include",
    signal,
  });
  if (!res.ok) throw new Error(`products fetch failed: ${res.status}`);
  return res.json();
}

/**
 * Header-search suggestions. Returns at most 5 ProductCards. AbortController
 * cancels prior in-flight requests on each keystroke. Empty / short queries
 * (< 2 chars) return [] without hitting the network.
 */
export async function fetchProductSuggestions(
  query: string,
  signal?: AbortSignal,
): Promise<ProductCard[]> {
  const trimmed = query.trim();
  if (trimmed.length < 2) return [];
  const result = await fetchProductList({ search: trimmed, limit: 5 }, signal);
  return result.items;
}
