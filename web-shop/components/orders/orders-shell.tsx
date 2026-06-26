"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Package } from "lucide-react";
import { fetchOrders } from "@/lib/shop-api";
import type { OrderListItem, OrderStatus } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";
import { LoginModal } from "@/components/auth/login-modal";
import { toast } from "sonner";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("en-IN", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function statusClasses(status: OrderStatus): string {
  switch (status) {
    case "delivered": return "bg-emerald-100 text-emerald-700";
    case "cancelled":
    case "cancelling": return "bg-red-100 text-red-700";
    case "shipped": return "bg-sky-100 text-sky-700";
    case "pending":
    case "confirmed":
    default: return "bg-amber-100 text-amber-700";
  }
}

export function OrdersShell() {
  const router = useRouter();
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [items, setItems] = useState<OrderListItem[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [done, setDone] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = async (c?: string) => {
    setLoading(true);
    try {
      const r = await fetchOrders(c);
      setItems((prev) => (c ? [...prev, ...r.items] : r.items));
      setCursor(r.next_cursor);
      if (!r.next_cursor) setDone(true);
      setAuthed(true);
    } catch (e) {
      const status = (e as { status?: number }).status;
      if (status === 401) setAuthed(false);
      else toast.error("Could not load orders");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, []);

  if (authed === false) {
    return <LoginModal open onClose={() => router.push("/")} onSuccess={() => { setAuthed(null); load(); }} />;
  }

  if (authed === null) return <p>Loading…</p>;

  if (items.length === 0) {
    return (
      <div className="text-center py-16 space-y-4">
        <Package className="size-12 mx-auto text-text-muted" aria-hidden />
        <div>
          <p className="font-medium">No orders yet</p>
          <p className="text-sm text-text-muted">Your placed orders show up here.</p>
        </div>
        <Link
          href="/"
          className="inline-block h-10 px-6 rounded bg-brand-600 text-white text-sm font-medium grid place-items-center hover:bg-brand-700"
        >
          Browse shop
        </Link>
      </div>
    );
  }

  return (
    <>
      <ul className="space-y-3">
        {items.map((o) => {
          const secondary = o.first_item_name
            ? `${o.first_item_name}${o.item_count > 1 ? ` + ${o.item_count - 1} more` : ""}`
            : `${o.item_count} item${o.item_count === 1 ? "" : "s"}`;
          return (
            <li key={o.id}>
              <Link
                href={`/orders/${o.id}`}
                className="flex items-center gap-3 p-4 border border-border rounded-lg hover:border-brand-600 hover:bg-brand-50/40"
              >
                <div className="relative size-14 shrink-0 rounded overflow-hidden bg-brand-50">
                  {o.first_item_image ? (
                    <Image src={o.first_item_image} alt="" fill sizes="56px" className="object-cover" />
                  ) : (
                    <Package className="size-6 m-auto text-text-muted" aria-hidden />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-baseline gap-2 flex-wrap">
                    <span className="font-medium">{o.invoice_number || o.id.slice(0, 8)}</span>
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${statusClasses(o.status)}`}>
                      {o.status}
                    </span>
                  </div>
                  <p className="text-sm text-text-muted line-clamp-1">{secondary}</p>
                  <p className="text-xs text-text-muted">{formatDate(o.created_at)}</p>
                </div>
                <div className="text-right shrink-0">
                  <p className="font-medium text-sm">{formatPaise(o.total_paise)}</p>
                  <p className="text-xs text-text-muted">{o.payment_status}</p>
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
      {!done && (
        <button
          type="button"
          onClick={() => load(cursor)}
          disabled={loading}
          className="mt-4 h-10 px-6 rounded border border-border hover:bg-brand-50 disabled:opacity-60"
        >
          {loading ? "Loading…" : "Load more"}
        </button>
      )}
    </>
  );
}
