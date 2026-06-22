"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { ProductCard } from "./product-card";
import { FeedSkeleton } from "./feed-skeleton";
import { fetchFeedPage } from "@/lib/shop-api";
import type { FeedPage, ProductCard as ProductCardType } from "@/lib/shop-types";

type Props = { initialPage: FeedPage };

export function InfiniteFeed({ initialPage }: Props) {
  const [items, setItems] = useState<ProductCardType[]>(initialPage.items);
  const [cursor, setCursor] = useState<string | undefined>(initialPage.next_cursor);
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(!initialPage.next_cursor);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  const loadMore = useCallback(async () => {
    if (loading || done) return;
    setLoading(true);
    try {
      const next = await fetchFeedPage(cursor);
      setItems((prev) => [...prev, ...next.items]);
      if (next.next_cursor) setCursor(next.next_cursor);
      else setDone(true);
    } catch {
      toast.error("Could not load more. Scroll to retry.");
    } finally {
      setLoading(false);
    }
  }, [cursor, loading, done]);

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const obs = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) void loadMore();
      },
      { rootMargin: "200px" },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [loadMore]);

  return (
    <section>
      <h2 className="text-xl font-semibold mb-4">For you</h2>
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
        {items.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
      {loading && (
        <div className="mt-4">
          <FeedSkeleton count={4} />
        </div>
      )}
      {!done && <div ref={sentinelRef} className="h-px" aria-hidden />}
      {done && items.length > 0 && (
        <p className="text-center text-sm text-muted mt-8">You're all caught up.</p>
      )}
      {items.length === 0 && !loading && (
        <p className="text-center text-sm text-muted mt-8">No products yet.</p>
      )}
    </section>
  );
}
