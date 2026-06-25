"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { fetchOrders } from "@/lib/shop-api";
import type { OrderListItem } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";
import { LoginModal } from "@/components/auth/login-modal";
import { toast } from "sonner";

export function OrdersShell() {
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
    return <LoginModal open onClose={() => {}} onSuccess={() => { setAuthed(null); load(); }} />;
  }

  if (authed === null) return <p>Loading…</p>;
  if (items.length === 0) return <p className="text-text-muted">No orders yet.</p>;

  return (
    <>
      <ul className="space-y-3">
        {items.map((o) => (
          <li key={o.id}>
            <Link
              href={`/orders/${o.id}`}
              className="block p-4 border border-border rounded hover:bg-brand-50"
            >
              <div className="flex justify-between items-baseline gap-3">
                <span className="font-medium">{o.invoice_number || o.id.slice(0, 8)}</span>
                <span className="text-sm">{formatPaise(o.total_paise)}</span>
              </div>
              <div className="text-sm text-text-muted mt-1">
                {o.item_count} item{o.item_count === 1 ? "" : "s"} · {o.status} · {o.payment_status}
              </div>
              <div className="text-xs text-text-muted">{new Date(o.created_at).toLocaleString()}</div>
            </Link>
          </li>
        ))}
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
