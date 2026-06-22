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
  const [error, setError] = useState(false);
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // Refs to avoid forcing observer re-registration when only loading flips.
  const loadingRef = useRef(false);
  const doneRef = useRef(!initialPage.next_cursor);

  const loadMore = useCallback(async () => {
    if (loadingRef.current || doneRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(false);
    try {
      const next = await fetchFeedPage(cursor);
      setItems((prev) => [...prev, ...next.items]);
      if (next.next_cursor) {
        setCursor(next.next_cursor);
      } else {
        doneRef.current = true;
        setDone(true);
      }
    } catch {
      // Surface error and stop the observer until the user retries explicitly,
      // else a sentinel still in view would re-fire fetch on every state churn.
      setError(true);
      toast.error("Could not load more. Tap Try again.");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [cursor]);

  const retry = useCallback(() => {
    setError(false);
    void loadMore();
  }, [loadMore]);

  useEffect(() => {
    if (error || done) return;
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
  }, [loadMore, error, done]);

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
      {error && (
        <div className="mt-6 text-center">
          <button
            type="button"
            onClick={retry}
            className="rounded-xl bg-brand-600 text-white px-5 py-2 text-sm font-medium hover:bg-brand-700"
          >
            Try again
          </button>
        </div>
      )}
      {!done && !error && <div ref={sentinelRef} className="h-px" aria-hidden />}
      {done && items.length > 0 && (
        <p className="text-center text-sm text-muted mt-8">You&apos;re all caught up.</p>
      )}
      {items.length === 0 && !loading && !error && (
        <p className="text-center text-sm text-muted mt-8">No products yet.</p>
      )}
    </section>
  );
}
