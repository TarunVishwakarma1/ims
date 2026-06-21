import Link from "next/link";

export function SiteFooter() {
  return (
    <footer className="border-t border-border mt-12">
      <div className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8 text-sm text-muted flex flex-wrap gap-6">
        <span>© {new Date().getFullYear()} Shop</span>
        <Link href="/privacy" className="hover:text-fg">Privacy</Link>
        <Link href="/terms" className="hover:text-fg">Terms</Link>
        <Link href="/contact" className="hover:text-fg">Contact</Link>
      </div>
    </footer>
  );
}
