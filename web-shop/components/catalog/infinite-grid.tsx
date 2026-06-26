"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { FeedSkeleton } from "@/components/home/feed-skeleton";

export type LoadResult<T> = { items: T[]; next_cursor?: string };

type Props<T> = {
  initialItems: T[];
  initialCursor?: string;
  loadMore: (cursor?: string) => Promise<LoadResult<T>>;
  renderItem: (item: T) => React.ReactNode;
  itemKey: (item: T) => string;
  emptyMessage?: string;
  doneMessage?: string;
  gridClassName?: string;
};

const DEFAULT_GRID = "grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4";

export function InfiniteGrid<T>({
  initialItems,
  initialCursor,
  loadMore,
  renderItem,
  itemKey,
  emptyMessage = "Nothing here yet.",
  doneMessage = "You've reached the end.",
  gridClassName = DEFAULT_GRID,
}: Props<T>) {
  const [items, setItems] = useState<T[]>(initialItems);
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(!initialCursor);
  const [error, setError] = useState(false);
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  const loadingRef = useRef(false);
  const doneRef = useRef(!initialCursor);
  // Cursor lives in a ref so handleLoadMore is not torn down + re-registered
  // every page; eliminates the stale-closure race where a fast-scroll after
  // setCursor could fire the old callback with the previous cursor.
  const cursorRef = useRef<string | undefined>(initialCursor);

  const handleLoadMore = useCallback(async () => {
    if (loadingRef.current || doneRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    setError(false);
    try {
      const next = await loadMore(cursorRef.current);
      setItems((prev) => [...prev, ...next.items]);
      if (next.next_cursor) {
        cursorRef.current = next.next_cursor;
      } else {
        cursorRef.current = undefined;
        doneRef.current = true;
        setDone(true);
      }
    } catch {
      setError(true);
      toast.error("Could not load more. Click Try again.");
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [loadMore]);

  const retry = useCallback(() => {
    setError(false);
    void handleLoadMore();
  }, [handleLoadMore]);

  useEffect(() => {
    if (error || done) return;
    const el = sentinelRef.current;
    if (!el) return;
    const obs = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) void handleLoadMore();
      },
      { rootMargin: "200px" },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [handleLoadMore, error, done]);

  return (
    <div>
      <div className={gridClassName}>{items.map((item) => (
        <div key={itemKey(item)}>{renderItem(item)}</div>
      ))}</div>
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
        <p className="text-center text-sm text-muted mt-8">{doneMessage}</p>
      )}
      {items.length === 0 && !loading && !error && (
        <p className="text-center text-sm text-muted mt-8">{emptyMessage}</p>
      )}
    </div>
  );
}
