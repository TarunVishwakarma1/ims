import Link from "next/link";
import { Search, ShoppingCart, User } from "lucide-react";

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-40 bg-bg/95 backdrop-blur border-b border-border">
      <div className="max-w-(--spacing-shop-page-max) mx-auto px-4 h-16 flex items-center gap-6">
        <Link href="/" className="text-xl font-semibold text-brand-600">
          Shop
        </Link>
        <Link
          href="/search"
          className="flex-1 max-w-md h-10 rounded-xl border border-border px-4 text-left text-muted flex items-center gap-2 hover:bg-brand-50"
        >
          <Search className="size-4" />
          <span>Search products…</span>
        </Link>
        <nav className="ml-auto flex items-center gap-3">
          <Link
            href="/profile"
            aria-label="Account"
            className="size-10 grid place-items-center rounded-full hover:bg-brand-50"
          >
            <User className="size-5" />
          </Link>
          <Link
            href="/cart"
            aria-label="Cart"
            className="size-10 grid place-items-center rounded-full hover:bg-brand-50"
          >
            <ShoppingCart className="size-5" />
          </Link>
        </nav>
      </div>
    </header>
  );
}
