"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Store, MapPin, Search, Loader2, ArrowRight } from "lucide-react";
import { fetchShops } from "@/lib/shop-api";
import type { ShopSummary } from "@/lib/shop-types";
import { toast } from "sonner";

const PIN_RE = /^[1-9]\d{5}$/;

export default function ShopsPage() {
  const router = useRouter();
  const [pincode, setPincode] = useState("");
  const [shops, setShops] = useState<ShopSummary[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  // Show all live shops on first load.
  useEffect(() => {
    (async () => {
      try {
        setShops(await fetchShops());
      } catch {
        setShops([]);
      }
    })();
  }, []);

  const search = async () => {
    if (pincode && !PIN_RE.test(pincode)) {
      toast.error("Enter a valid 6-digit pincode");
      return;
    }
    setLoading(true);
    setSearched(true);
    try {
      setShops(await fetchShops(pincode || undefined));
    } catch {
      toast.error("Could not load shops");
    } finally {
      setLoading(false);
    }
  };

  const visit = (slug: string) => {
    try {
      localStorage.setItem("shop_slug", slug);
    } catch {
      /* ignore */
    }
    // Per-shop storefront routing arrives in P4 phase 2; for now enter the shop.
    router.push("/");
  };

  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-10 space-y-8">
      <header className="text-center space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">Shops near you</h1>
        <p className="text-text-muted">Enter your pincode to find shops that deliver to you.</p>
      </header>

      <div className="flex gap-2 max-w-md mx-auto">
        <div className="relative flex-1">
          <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-text-muted" aria-hidden />
          <input
            value={pincode}
            inputMode="numeric"
            maxLength={6}
            placeholder="e.g. 411001"
            onChange={(e) => setPincode(e.target.value.replace(/\D/g, ""))}
            onKeyDown={(e) => e.key === "Enter" && search()}
            className="w-full h-11 pl-9 pr-3 rounded-lg border border-border bg-surface"
          />
        </div>
        <button
          type="button"
          onClick={search}
          disabled={loading}
          className="h-11 px-5 rounded-lg bg-brand-600 text-white font-medium inline-flex items-center gap-2 hover:bg-brand-700 disabled:opacity-60"
        >
          {loading ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
          Find
        </button>
      </div>

      {shops === null ? (
        <p className="text-center text-text-muted">Loading…</p>
      ) : shops.length === 0 ? (
        <div className="text-center py-12 space-y-2">
          <Store className="size-10 mx-auto text-text-muted" aria-hidden />
          <p className="font-medium">No shops deliver here yet</p>
          <p className="text-sm text-text-muted">
            {searched ? "Try a different pincode." : "Check back soon."}
          </p>
        </div>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 max-w-4xl mx-auto">
          {shops.map((s) => (
            <li key={s.slug}>
              <button
                type="button"
                onClick={() => visit(s.slug)}
                className="group w-full text-left rounded-2xl border border-border bg-surface p-5 transition-all hover:-translate-y-0.5 hover:shadow-lg hover:border-brand-300"
              >
                <div className="flex items-center gap-3">
                  <span className="size-12 grid place-items-center rounded-xl bg-brand-50 text-brand-700 shrink-0">
                    {s.logo_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={s.logo_url} alt="" className="size-full rounded-xl object-cover" />
                    ) : (
                      <Store className="size-6" />
                    )}
                  </span>
                  <div className="min-w-0">
                    <h2 className="font-semibold truncate group-hover:text-brand-700 transition-colors">{s.name}</h2>
                    {(s.area || s.city) && (
                      <p className="text-xs text-text-muted truncate">{[s.area, s.city].filter(Boolean).join(", ")}</p>
                    )}
                  </div>
                </div>
                {s.tagline && <p className="mt-3 text-sm text-text-muted line-clamp-2">{s.tagline}</p>}
                <span className="mt-4 inline-flex items-center gap-1 text-sm font-medium text-brand-700">
                  Visit shop <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" />
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
