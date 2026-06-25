"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { fetchOrderDetail, cancelOrder } from "@/lib/shop-api";
import type { OrderDetail } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";
import { toast } from "sonner";

export function OrderDetailShell({ id, placed }: { id: string; placed: boolean }) {
  const [data, setData] = useState<OrderDetail | null>(null);
  const [cancelling, setCancelling] = useState(false);

  const load = async () => {
    try {
      setData(await fetchOrderDetail(id));
    } catch {
      toast.error("Could not load order");
    }
  };

  useEffect(() => { load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [id]);

  if (!data) return <p>Loading…</p>;

  const canCancel = data.status === "pending" || data.status === "confirmed";

  const onCancel = async () => {
    if (!confirm("Cancel this order?")) return;
    setCancelling(true);
    try {
      await cancelOrder(id);
      toast.success("Cancellation requested");
      await load();
    } catch {
      toast.error("Could not cancel");
    } finally {
      setCancelling(false);
    }
  };

  return (
    <>
      {placed && (
        <div role="status" className="mb-4 p-3 rounded bg-brand-50 border border-brand-200 text-sm">
          ✓ Order placed. {data.payment_status === "paid" ? "Payment confirmed." : "Awaiting payment confirmation."}
        </div>
      )}
      <h1 className="text-2xl font-semibold mb-1">Order {data.invoice_number || id.slice(0, 8)}</h1>
      <p className="text-text-muted mb-6">{data.status} · {data.payment_status} · {new Date(data.created_at).toLocaleString()}</p>
      <section className="border border-border rounded-lg p-4 mb-4">
        <h2 className="font-semibold mb-2">Items</h2>
        <ul className="divide-y divide-border">
          {data.items.map((it) => (
            <li key={it.product_id} className="py-2 flex justify-between">
              <Link href={`/p/${it.slug}`} className="hover:text-brand-600">{it.name} × {it.qty}</Link>
              <span>{formatPaise(it.qty * it.unit_price_paise)}</span>
            </li>
          ))}
        </ul>
        <div className="border-t border-border mt-3 pt-2 flex justify-between font-semibold">
          <span>Total</span><span>{formatPaise(data.total_paise)}</span>
        </div>
      </section>
      <section className="border border-border rounded-lg p-4 mb-4">
        <h2 className="font-semibold mb-2">Delivery</h2>
        <p className="text-sm">{data.delivery_address.name} · {data.delivery_address.phone}</p>
        <p className="text-sm text-text-muted">
          {data.delivery_address.line1}{data.delivery_address.line2 ? `, ${data.delivery_address.line2}` : ""},{" "}
          {data.delivery_address.city}, {data.delivery_address.state} {data.delivery_address.pincode}
        </p>
      </section>
      {canCancel && (
        <button
          type="button"
          onClick={onCancel}
          disabled={cancelling}
          className="h-10 px-6 rounded border border-danger text-danger hover:bg-danger hover:text-white disabled:opacity-60"
        >
          {cancelling ? "Cancelling…" : "Cancel order"}
        </button>
      )}
    </>
  );
}
