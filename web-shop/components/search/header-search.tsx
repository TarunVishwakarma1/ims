"use client";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Search } from "lucide-react";
import Image from "next/image";
import { fetchProductSuggestions } from "@/lib/shop-api";
import { useShopSlug, useShopHref } from "@/lib/use-shop-slug";
import { paiseToINR } from "@/lib/format";
import type { ProductCard } from "@/lib/shop-types";

const DEBOUNCE_MS = 300;

export function HeaderSearch() {
  const router = useRouter();
  const shop = useShopSlug();
  const shopHref = useShopHref();
  const [q, setQ] = useState("");
  const [items, setItems] = useState<ProductCard[]>([]);
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(-1);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (q.trim().length < 2) {
      setItems([]);
      return;
    }
    const ctl = new AbortController();
    const t = setTimeout(async () => {
      try {
        const result = await fetchProductSuggestions(shop, q, ctl.signal);
        setItems(result);
        setHighlight(-1);
      } catch {
        // Silent: aborts + transient errors should not toast in the header.
      }
    }, DEBOUNCE_MS);
    return () => {
      clearTimeout(t);
      ctl.abort();
    };
  }, [q, shop]);

  useEffect(() => {
    function onDocDown(e: MouseEvent) {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setOpen(false);
        inputRef.current?.blur();
      }
    }
    document.addEventListener("mousedown", onDocDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocDown);
      document.removeEventListener("keydown", onKey);
    };
  }, []);

  function submit(query: string) {
    const trimmed = query.trim();
    if (!trimmed) return;
    setOpen(false);
    setQ("");
    router.push(shopHref(`/search?q=${encodeURIComponent(trimmed)}`));
  }

  function pickItem(slug: string) {
    setOpen(false);
    setQ("");
    router.push(shopHref(`/p/${slug}`));
  }

  function onListKey(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(items.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(-1, h - 1));
    } else if (e.key === "Enter") {
      if (highlight >= 0 && items[highlight]) {
        e.preventDefault();
        pickItem(items[highlight].slug);
      }
    }
  }

  const showDropdown = open && q.trim().length >= 2;

  return (
    <div ref={wrapRef} className="relative flex-1 max-w-md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          submit(q);
        }}
      >
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-text-muted" aria-hidden />
          <input
            ref={inputRef}
            type="search"
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            onKeyDown={onListKey}
            placeholder="Search products…"
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={showDropdown}
            aria-controls={showDropdown ? "header-search-list" : undefined}
            className="w-full h-10 rounded-xl border border-border bg-bg pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-brand-600"
          />
        </div>
      </form>

      {showDropdown && (
        <div className="absolute left-0 right-0 top-12 z-50 bg-bg border border-border rounded-xl shadow-lg overflow-hidden">
          {items.length === 0 ? (
            <p className="p-4 text-sm text-text-muted">No matches.</p>
          ) : (
            <ul id="header-search-list" role="listbox">
              {items.map((p, i) => (
                <li
                  key={p.id}
                  role="option"
                  aria-selected={i === highlight}
                  onClick={() => pickItem(p.slug)}
                  className={`flex items-center gap-3 px-3 py-2 text-sm cursor-pointer ${
                    i === highlight ? "bg-brand-50" : "hover:bg-brand-50"
                  }`}
                >
                  <div className="relative size-10 rounded-lg overflow-hidden bg-brand-100 flex-shrink-0">
                    {p.image_url && (
                      <Image src={p.image_url} alt="" fill sizes="40px" className="object-cover" />
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="truncate">{p.name}</p>
                    <p className="text-xs text-brand-700 font-medium">
                      {paiseToINR(p.price_paise)}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          )}
          <button
            type="button"
            onClick={() => submit(q)}
            className="w-full px-3 py-2 text-left text-sm font-medium text-brand-700 border-t border-border hover:bg-brand-50"
          >
            See all results for &ldquo;{q.trim()}&rdquo;
          </button>
        </div>
      )}
    </div>
  );
}
