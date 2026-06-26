import Link from "next/link";
import { User } from "lucide-react";
import { HeaderSearch } from "@/components/search/header-search";
import { CartIconButton } from "@/components/cart/cart-icon-button";

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-40 bg-bg/95 backdrop-blur border-b border-border">
      <div className="max-w-(--spacing-shop-page-max) mx-auto px-4 h-16 flex items-center gap-6">
        <Link href="/" className="text-xl font-semibold text-brand-600">
          Shop
        </Link>
        <HeaderSearch />
        <nav className="ml-auto flex items-center gap-3">
          <Link
            href="/profile"
            aria-label="Account"
            className="size-10 grid place-items-center rounded-full hover:bg-brand-50"
          >
            <User className="size-5" />
          </Link>
          <CartIconButton />
        </nav>
      </div>
    </header>
  );
}
