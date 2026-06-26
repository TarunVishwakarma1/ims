"use client";
import { ProductCard } from "@/components/home/product-card";
import { InfiniteGrid } from "@/components/catalog/infinite-grid";
import { fetchProductList } from "@/lib/shop-api";
import type { ProductListQuery, ProductListResult } from "@/lib/shop-types";

type Props = {
  initial: ProductListResult;
  baseQuery: ProductListQuery;
  emptyMessage?: string;
};

export function ProductGridPage({ initial, baseQuery, emptyMessage }: Props) {
  return (
    <InfiniteGrid
      initialItems={initial.items}
      initialCursor={initial.next_cursor}
      loadMore={async (cursor) => {
        const res = await fetchProductList({ ...baseQuery, cursor });
        return { items: res.items, next_cursor: res.next_cursor };
      }}
      renderItem={(p) => <ProductCard product={p} />}
      itemKey={(p) => p.id}
      emptyMessage={emptyMessage ?? "No products match."}
      doneMessage="That's all we have."
    />
  );
}
