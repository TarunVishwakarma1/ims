import { notFound } from "next/navigation";
import { serverFetch } from "@/lib/api";

const SLUG_RE = /^[a-z0-9-]+$/;

// Validates the shop slug once for every storefront page. The backend
// ResolveShop middleware returns 404 {"error":"shop_not_found"} for an unknown
// slug on any /api/shop/s/<slug>/* route, so we probe the cheapest one. A
// backend outage (non-404) is not treated as "shop missing" — pages render
// their own empty shells in that case.
export default async function ShopLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ shop: string }>;
}) {
  const { shop } = await params;
  if (!SLUG_RE.test(shop)) notFound();

  // Probe the cheapest scoped route. Capture status inside try; call notFound()
  // OUTSIDE so its NEXT_NOT_FOUND throw is not swallowed by the catch. A
  // backend outage (status stays 0) renders the shell rather than 404-ing.
  let status = 0;
  try {
    status = (await serverFetch(`/api/shop/s/${shop}/banners/active`)).status;
  } catch {
    return <>{children}</>;
  }
  if (status === 404) notFound();

  return <>{children}</>;
}
