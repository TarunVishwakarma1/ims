"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { fetchOrderDetail, cancelOrder } from "@/lib/shop-api";
import type { OrderDetail } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";
import { toast } from "sonner";

type CancelState = "idle" | "confirming" | "cancelling";

export function OrderDetailShell({ id, placed }: { id: string; placed: boolean }) {
  const [data, setData] = useState<OrderDetail | null>(null);
  const [cancelState, setCancelState] = useState<CancelState>("idle");

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

  const onConfirmCancel = async () => {
    setCancelState("cancelling");
    try {
      await cancelOrder(id);
      toast.success("Cancellation requested");
      await load();
    } catch {
      toast.error("Could not cancel");
    } finally {
      setCancelState("idle");
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
      {canCancel && cancelState === "idle" && (
        <button
          type="button"
          onClick={() => setCancelState("confirming")}
          className="h-10 px-6 rounded border border-danger text-danger hover:bg-danger hover:text-white"
        >
          Cancel order
        </button>
      )}
      {canCancel && cancelState !== "idle" && (
        <div role="alertdialog" aria-label="Confirm cancellation" className="border border-danger rounded-lg p-4 flex flex-wrap items-center gap-3">
          <p className="text-sm flex-1 min-w-0">Cancel this order? This cannot be undone.</p>
          <button
            type="button"
            onClick={onConfirmCancel}
            disabled={cancelState === "cancelling"}
            className="h-9 px-4 rounded bg-danger text-white disabled:opacity-60"
          >
            {cancelState === "cancelling" ? "Cancelling…" : "Yes, cancel"}
          </button>
          <button
            type="button"
            onClick={() => setCancelState("idle")}
            disabled={cancelState === "cancelling"}
            className="h-9 px-4 rounded border border-border disabled:opacity-60"
          >
            Keep order
          </button>
        </div>
      )}
    </>
  );
}
