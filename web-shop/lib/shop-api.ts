import type {
  Address,
  AddressInput,
  Cart,
  CheckoutSummary,
  CustomerProfile,
  FeedPage,
  OrderDetail,
  OrderListResult,
  PaymentOption,
  PaymentOptionsResponse,
  PlaceOrderInput,
  PlaceOrderResult,
  ProductCard,
  ProductListQuery,
  ProductListResult,
  ProfileInput,
  ShopSummary,
  VerifyRazorpayInput,
  VerifyRazorpayResult,
} from "@/lib/shop-types";
import { shopApiBase } from "@/lib/shop-path";

/**
 * Browser-side fetch for feed pages. Scoped to a shop slug when given
 * (/api/shop/s/<slug>/feed); a null slug uses the legacy default-org feed.
 * Next rewrites same-origin /api/shop/* to backend per next.config.ts.
 */
export async function fetchFeedPage(
  shop: string | null,
  cursor?: string,
  limit = 24,
): Promise<FeedPage> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set("cursor", cursor);
  const res = await fetch(`${shopApiBase(shop)}/feed?${params.toString()}`, {
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
  shop: string | null,
  q: ProductListQuery,
  signal?: AbortSignal,
): Promise<ProductListResult> {
  const res = await fetch(
    `${shopApiBase(shop)}/products?${buildProductParams(q).toString()}`,
    { credentials: "include", signal },
  );
  if (!res.ok) throw new Error(`products fetch failed: ${res.status}`);
  return res.json();
}

/**
 * Header-search suggestions. Returns at most 5 ProductCards. AbortController
 * cancels prior in-flight requests on each keystroke. Empty / short queries
 * (< 2 chars) return [] without hitting the network.
 */
export async function fetchProductSuggestions(
  shop: string | null,
  query: string,
  signal?: AbortSignal,
): Promise<ProductCard[]> {
  const trimmed = query.trim();
  if (trimmed.length < 2) return [];
  const result = await fetchProductList(shop, { search: trimmed, limit: 5 }, signal);
  return result.items;
}

async function jsonOrThrow<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let code = `http_${res.status}`;
    let detail = "";
    try {
      const body = (await res.json()) as { error?: string; message?: string };
      if (body.error) code = body.error;
      if (body.message) detail = body.message;
    } catch {}
    const err = new Error(code) as Error & { status: number; code: string; detail: string };
    err.status = res.status;
    err.code = code;
    err.detail = detail;
    throw err;
  }
  return (await res.json()) as T;
}

// ── Cart ────────────────────────────────────────────────────────────────

export async function fetchCart(): Promise<Cart> {
  return jsonOrThrow<Cart>(
    await fetch("/api/shop/cart", { credentials: "include" }),
  );
}

export async function addCartItem(productID: string, qty: number): Promise<Cart> {
  return jsonOrThrow<Cart>(
    await fetch("/api/shop/cart/items", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ product_id: productID, qty }),
    }),
  );
}

export async function removeCartItem(productID: string): Promise<Cart> {
  return jsonOrThrow<Cart>(
    await fetch(`/api/shop/cart/items/${encodeURIComponent(productID)}`, {
      method: "DELETE",
      credentials: "include",
    }),
  );
}

export async function mergeCart(
  items: { product_id: string; qty: number }[],
): Promise<Cart> {
  return jsonOrThrow<Cart>(
    await fetch("/api/shop/cart/merge", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ items }),
    }),
  );
}

// ── Checkout ────────────────────────────────────────────────────────────

export async function fetchCheckoutSummary(
  addressID: string,
  couponCode?: string,
): Promise<CheckoutSummary> {
  const qs = new URLSearchParams({ address_id: addressID });
  if (couponCode) qs.set("coupon", couponCode);
  return jsonOrThrow<CheckoutSummary>(
    await fetch(`/api/shop/checkout/summary?${qs.toString()}`, {
      credentials: "include",
    }),
  );
}

export async function fetchPaymentOptions(): Promise<PaymentOption[]> {
  const r = await jsonOrThrow<PaymentOptionsResponse>(
    await fetch("/api/shop/checkout/payment-options", { credentials: "include" }),
  );
  return r.methods;
}

export async function placeOrder(
  input: PlaceOrderInput,
  idempotencyKey?: string,
): Promise<PlaceOrderResult> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;
  return jsonOrThrow<PlaceOrderResult>(
    await fetch("/api/shop/checkout/place", {
      method: "POST",
      credentials: "include",
      headers,
      body: JSON.stringify(input),
    }),
  );
}

export async function verifyRazorpayPayment(
  input: VerifyRazorpayInput,
): Promise<VerifyRazorpayResult> {
  return jsonOrThrow<VerifyRazorpayResult>(
    await fetch("/api/shop/payments/razorpay/verify", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  );
}

// ── Orders ──────────────────────────────────────────────────────────────

export async function fetchOrders(cursor?: string): Promise<OrderListResult> {
  const qs = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return jsonOrThrow<OrderListResult>(
    await fetch(`/api/shop/orders${qs}`, { credentials: "include" }),
  );
}

export async function fetchOrderDetail(id: string): Promise<OrderDetail> {
  return jsonOrThrow<OrderDetail>(
    await fetch(`/api/shop/orders/${encodeURIComponent(id)}`, { credentials: "include" }),
  );
}

export async function cancelOrder(id: string): Promise<{ status: string }> {
  return jsonOrThrow<{ status: string }>(
    await fetch(`/api/shop/orders/${encodeURIComponent(id)}/cancel`, {
      method: "POST",
      credentials: "include",
    }),
  );
}

// ── Shop directory ──────────────────────────────────────────────────────

export async function fetchShops(pincode?: string): Promise<ShopSummary[]> {
  const qs = pincode ? `?pincode=${encodeURIComponent(pincode)}` : "";
  const r = await jsonOrThrow<{ shops: ShopSummary[] }>(
    await fetch(`/api/shop/shops${qs}`, { credentials: "include" }),
  );
  return r.shops;
}

// ── Profile ─────────────────────────────────────────────────────────────

export async function getMe(): Promise<CustomerProfile> {
  return jsonOrThrow<CustomerProfile>(
    await fetch("/api/shop/me", { credentials: "include" }),
  );
}

export async function updateMe(input: ProfileInput): Promise<void> {
  await jsonOrThrow<{ status: string }>(
    await fetch("/api/shop/me", {
      method: "PATCH",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  );
}

// ── Addresses ───────────────────────────────────────────────────────────

export async function listAddresses(): Promise<Address[]> {
  return jsonOrThrow<Address[]>(
    await fetch("/api/shop/addresses", { credentials: "include" }),
  );
}

export async function addAddress(input: AddressInput): Promise<Address> {
  return jsonOrThrow<Address>(
    await fetch("/api/shop/addresses", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  );
}

export async function updateAddress(
  id: string,
  input: Partial<AddressInput>,
): Promise<Address> {
  return jsonOrThrow<Address>(
    await fetch(`/api/shop/addresses/${encodeURIComponent(id)}`, {
      method: "PATCH",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  );
}

export async function deleteAddress(id: string): Promise<void> {
  const r = await fetch(`/api/shop/addresses/${encodeURIComponent(id)}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!r.ok) throw new Error(`delete_failed: ${r.status}`);
}

export async function setDefaultAddress(id: string): Promise<void> {
  const r = await fetch(`/api/shop/addresses/${encodeURIComponent(id)}/default`, {
    method: "POST",
    credentials: "include",
  });
  if (!r.ok) throw new Error(`set_default_failed: ${r.status}`);
}
