"use client";
import { ProductCard } from "@/components/home/product-card";
import { InfiniteGrid } from "@/components/catalog/infinite-grid";
import { fetchProductList } from "@/lib/shop-api";
import { useShopSlug } from "@/lib/use-shop-slug";
import type { ProductListQuery, ProductListResult } from "@/lib/shop-types";

type Props = {
  initial: ProductListResult;
  baseQuery: ProductListQuery;
  emptyMessage?: string;
};

export function ProductGridPage({ initial, baseQuery, emptyMessage }: Props) {
  const shop = useShopSlug();
  // InfiniteGrid captures initialItems in useState; without a key it keeps
  // stale state when the parent re-renders with a different sort/filter.
  // Encode the query shape so a change forces a fresh mount.
  const resetKey = `${baseQuery.category ?? ""}|${baseQuery.search ?? ""}|${baseQuery.sort ?? "newest"}|${baseQuery.in_stock ? "1" : "0"}`;
  return (
    <InfiniteGrid
      key={resetKey}
      initialItems={initial.items}
      initialCursor={initial.next_cursor}
      loadMore={async (cursor) => {
        const res = await fetchProductList(shop, { ...baseQuery, cursor });
        return { items: res.items, next_cursor: res.next_cursor };
      }}
      renderItem={(p) => <ProductCard product={p} />}
      itemKey={(p) => p.id}
      emptyMessage={emptyMessage ?? "No products match."}
      doneMessage="That's all we have."
    />
  );
}
