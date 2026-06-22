export function FeedSkeleton({ count = 8 }: { count?: number }) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Loading products"
      className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          aria-hidden
          className="rounded-2xl border border-border overflow-hidden"
        >
          <div className="aspect-square bg-brand-50 animate-pulse" />
          <div className="p-3 space-y-2">
            <div className="h-4 bg-border rounded animate-pulse" />
            <div className="h-4 w-1/2 bg-border rounded animate-pulse" />
          </div>
        </div>
      ))}
      <span className="sr-only">Loading products…</span>
    </div>
  );
}
