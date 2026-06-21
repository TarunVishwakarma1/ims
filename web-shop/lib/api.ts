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
 * Server-side fetch helper. Reads cookie via next/headers, forwards token to
 * backend directly (skips Next rewrite). Used by Server Components + Route
 * Handlers.
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
