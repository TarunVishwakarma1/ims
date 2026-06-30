import { notFound } from "next/navigation";
import Link from "next/link";
import { serverFetch, safeJson } from "@/lib/api";
import { ProductGridPage } from "@/components/catalog/product-grid-page";
import { SortDropdown } from "@/components/catalog/sort-dropdown";
import { InStockToggle } from "@/components/catalog/in-stock-toggle";
import { isProductSort } from "@/lib/shop-types";
import type {
  Category,
  ProductListResult,
  ProductListQuery,
  ProductSort,
} from "@/lib/shop-types";

export const dynamic = "force-dynamic";

const SLUG_RE = /^[a-z0-9-]+$/;

type PageProps = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ sort?: string; in_stock?: string }>;
};

export default async function CategoryPage({ params, searchParams }: PageProps) {
  const { slug } = await params;
  if (!SLUG_RE.test(slug)) notFound();
  const sp = await searchParams;

  const sort: ProductSort = sp.sort && isProductSort(sp.sort) ? sp.sort : "newest";
  const inStock = sp.in_stock === "true";

  const [categories, initial] = await Promise.all([
    safeJson<Category[]>(serverFetch("/api/shop/categories"), []),
    safeJson<ProductListResult>(
      serverFetch(
        `/api/shop/products?category=${encodeURIComponent(slug)}&sort=${sort}` +
          (inStock ? "&in_stock=true" : "") +
          "&limit=24",
      ),
      { items: [], total_count: 0, limit: 24 },
    ),
  ]);

  const category = categories.find((c) => c.slug === slug);
  if (!category) notFound();

  const baseQuery: ProductListQuery = {
    category: slug,
    sort,
    in_stock: inStock || undefined,
    limit: 24,
  };

  return (
    <div className="space-y-6">
      <nav className="text-sm text-text-muted">
        <Link href="/" className="hover:underline">Home</Link>
        <span className="mx-1">›</span>
        <span aria-current="page">{category.name}</span>
      </nav>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <h1 className="text-2xl font-semibold">{category.name}</h1>
        <div className="flex items-center gap-4">
          <InStockToggle checked={inStock} />
          <SortDropdown value={sort} />
        </div>
      </header>
      <ProductGridPage
        initial={initial}
        baseQuery={baseQuery}
        emptyMessage="No products in this category yet."
      />
    </div>
  );
}
