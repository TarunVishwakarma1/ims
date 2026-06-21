import ky from "ky";

/**
 * Browser-side ky client. Uses same-origin /api/shop/* (rewritten to backend
 * by next.config.ts). httpOnly auth cookie travels with same-origin requests.
 */
export const api = ky.create({
  prefix: "/api/shop",
  credentials: "include",
  retry: { limit: 1 },
  timeout: 15000,
});

/**
 * Server-side fetch helper. Reads `shop_session` cookie via next/headers and
 * forwards it as `Authorization: Bearer <token>`.
 *
 * IMPORTANT: `path` MUST be a FULL backend path including the `/api/shop`
 * prefix (e.g. "/api/shop/orders"). Unlike the browser `api` client, this
 * helper does NOT prefix automatically — it bypasses the Next rewrite and
 * hits BACKEND_URL directly.
 *
 * Plan 3b implements the login route handler that mints the `shop_session`
 * cookie value (= raw shop JWT). Cookie value = bearer token, by contract.
 */
export async function serverFetch(path: string, init?: RequestInit) {
  const { cookies } = await import("next/headers");
  const cookieStore = await cookies();
  const token = cookieStore.get("shop_session")?.value;
  const headers = new Headers(init?.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const base = process.env.BACKEND_URL || "http://localhost:8080";
  return fetch(`${base}${path}`, { ...init, headers, cache: "no-store" });
}
