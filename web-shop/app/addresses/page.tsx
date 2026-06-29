import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { AddressesShell } from "@/components/account/addresses-shell";

export const dynamic = "force-dynamic";

export default function AddressesPage() {
  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8">
      <Link href="/profile" className="inline-flex items-center gap-1 text-sm text-text-muted hover:text-brand-600 mb-4">
        <ChevronLeft className="size-4" /> Account
      </Link>
      <h1 className="text-2xl font-semibold mb-4">Saved addresses</h1>
      <AddressesShell />
    </main>
  );
}
