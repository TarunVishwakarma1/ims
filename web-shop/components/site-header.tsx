"use client";
import Link from "next/link";
import { Package, User, ShoppingBasket } from "lucide-react";
import { HeaderSearch } from "@/components/search/header-search";
import { CartIconButton } from "@/components/cart/cart-icon-button";
import { ThemeToggle } from "@/components/theme-toggle";
import { useShopSlug } from "@/lib/use-shop-slug";
import { shopHref } from "@/lib/shop-path";

export function SiteHeader() {
  const shop = useShopSlug();
  // In a storefront the logo returns to that shop's home; on global pages
  // (orders, profile, picker) it goes to the shop directory.
  const home = shop ? shopHref(shop) : "/shops";
  return (
    <header className="sticky top-0 z-40 bg-bg/80 backdrop-blur-md border-b border-border">
      <div className="max-w-(--spacing-shop-page-max) mx-auto px-4 h-16 flex items-center gap-6">
        <Link href={home} aria-label="Shop home" className="flex items-center gap-2 shrink-0">
          <span className="size-8 grid place-items-center rounded-lg bg-brand-600 text-white shadow-sm">
            <ShoppingBasket className="size-5" />
          </span>
          <span className="text-xl font-semibold tracking-tight text-fg">Shop</span>
        </Link>
        <HeaderSearch />
        <nav className="ml-auto flex items-center gap-1">
          <ThemeToggle />
          <Link
            href="/orders"
            aria-label="My orders"
            className="size-9 grid place-items-center rounded-full text-fg/80 hover:bg-surface-2 hover:text-fg transition-colors"
          >
            <Package className="size-5" />
          </Link>
          <Link
            href="/profile"
            aria-label="Account"
            className="size-9 grid place-items-center rounded-full text-fg/80 hover:bg-surface-2 hover:text-fg transition-colors"
          >
            <User className="size-5" />
          </Link>
          <CartIconButton />
        </nav>
      </div>
    </header>
  );
}
