"use client";
import { ProductCard } from "./product-card";
import { InfiniteGrid } from "@/components/catalog/infinite-grid";
import { fetchFeedPage } from "@/lib/shop-api";
import { useShopSlug } from "@/lib/use-shop-slug";
import type { FeedPage } from "@/lib/shop-types";

type Props = { initialPage: FeedPage };

export function InfiniteFeed({ initialPage }: Props) {
  const shop = useShopSlug();
  return (
    <section>
      <h2 className="text-xl font-semibold mb-4">For you</h2>
      <InfiniteGrid
        initialItems={initialPage.items}
        initialCursor={initialPage.next_cursor}
        loadMore={async (cursor) => {
          const page = await fetchFeedPage(shop, cursor);
          return { items: page.items, next_cursor: page.next_cursor };
        }}
        renderItem={(p) => <ProductCard product={p} />}
        itemKey={(p) => p.id}
        emptyMessage="No products yet."
        doneMessage="You're all caught up."
      />
    </section>
  );
}
