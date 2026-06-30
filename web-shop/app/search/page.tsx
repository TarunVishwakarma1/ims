import { serverFetch, safeJson } from "@/lib/api";
import { ProductGridPage } from "@/components/catalog/product-grid-page";
import { SortDropdown } from "@/components/catalog/sort-dropdown";
import { InStockToggle } from "@/components/catalog/in-stock-toggle";
import { isProductSort } from "@/lib/shop-types";
import type {
  ProductListResult,
  ProductListQuery,
  ProductSort,
} from "@/lib/shop-types";

export const dynamic = "force-dynamic";

type PageProps = {
  searchParams: Promise<{ q?: string; sort?: string; in_stock?: string }>;
};

export default async function SearchPage({ searchParams }: PageProps) {
  const sp = await searchParams;
  const q = (sp.q ?? "").trim();
  const sort: ProductSort = sp.sort && isProductSort(sp.sort) ? sp.sort : "newest";
  const inStock = sp.in_stock === "true";

  if (!q) {
    return (
      <div className="space-y-3">
        <h1 className="text-2xl font-semibold">Search</h1>
        <p className="text-text-muted">Type a product name in the search bar above.</p>
      </div>
    );
  }

  const initial = await safeJson<ProductListResult>(
    serverFetch(
      `/api/shop/products?search=${encodeURIComponent(q)}&sort=${sort}` +
        (inStock ? "&in_stock=true" : "") +
        "&limit=24",
    ),
    { items: [], total_count: 0, limit: 24 },
  );

  const baseQuery: ProductListQuery = {
    search: q,
    sort,
    in_stock: inStock || undefined,
    limit: 24,
  };

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <h1 className="text-2xl font-semibold">
          {initial.total_count} result{initial.total_count === 1 ? "" : "s"} for &ldquo;{q}&rdquo;
        </h1>
        <div className="flex items-center gap-4">
          <InStockToggle checked={inStock} />
          <SortDropdown value={sort} />
        </div>
      </header>
      <ProductGridPage
        initial={initial}
        baseQuery={baseQuery}
        emptyMessage={`No products match “${q}”.`}
      />
    </div>
  );
}
