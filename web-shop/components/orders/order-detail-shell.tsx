"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { Printer } from "lucide-react";
import { fetchOrderDetail, cancelOrder } from "@/lib/shop-api";
import type { ChargeLine, OrderDetail } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";
import { toast } from "sonner";

type CancelState = "idle" | "confirming" | "cancelling";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("en-IN", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function ChargeRow({ line }: { line: ChargeLine }) {
  const negative = line.paise < 0;
  const display = line.struck ? "Free" : formatPaise(line.paise);
  return (
    <div className="flex justify-between text-sm">
      <dt className="text-text-muted">{line.label}</dt>
      <dd className={line.struck ? "line-through text-text-muted" : negative ? "text-emerald-700" : ""}>
        {display}
      </dd>
    </div>
  );
}

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
    <article className="space-y-6 print:max-w-2xl print:mx-auto">
      {placed && (
        <div role="status" className="p-3 rounded bg-brand-50 border border-brand-200 text-sm print:hidden">
          ✓ Order placed.{" "}
          {data.payment_status === "paid" ? "Payment confirmed." : "Awaiting payment confirmation."}
        </div>
      )}

      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-4">
        <div className="space-y-1">
          <p className="text-xs uppercase tracking-wider text-text-muted">Invoice</p>
          <h1 className="text-2xl font-semibold">{data.invoice_number || id.slice(0, 8)}</h1>
          <p className="text-sm text-text-muted">{formatDate(data.created_at)}</p>
        </div>
        <div className="text-right space-y-1">
          <span
            className={`inline-block px-2 py-1 rounded text-xs font-medium ${
              data.status === "delivered"
                ? "bg-emerald-100 text-emerald-700"
                : data.status === "cancelled"
                  ? "bg-red-100 text-red-700"
                  : "bg-amber-100 text-amber-700"
            }`}
          >
            {data.status}
          </span>
          <p className="text-xs text-text-muted">Payment: {data.payment_status}</p>
          <button
            type="button"
            onClick={() => window.print()}
            aria-label="Print invoice"
            className="mt-1 inline-flex items-center gap-1 text-xs text-brand-600 hover:underline print:hidden"
          >
            <Printer className="size-3" /> Print
          </button>
        </div>
      </header>

      <section aria-labelledby="items-h" className="border border-border rounded-lg overflow-hidden">
        <h2 id="items-h" className="font-semibold px-4 py-3 border-b border-border bg-brand-50/50">
          Items
        </h2>
        <ul className="divide-y divide-border">
          {data.items.map((it) => {
            const lineTotal = it.qty * it.unit_price_paise;
            return (
              <li key={it.product_id} className="flex items-center gap-3 p-3">
                <Link
                  href={`/p/${it.slug}`}
                  className="relative size-14 shrink-0 rounded overflow-hidden bg-brand-50 print:size-12"
                >
                  {it.image ? (
                    <Image src={it.image} alt={it.name} fill sizes="56px" className="object-cover" />
                  ) : null}
                </Link>
                <div className="flex-1 min-w-0">
                  <Link href={`/p/${it.slug}`} className="font-medium text-sm line-clamp-2 hover:text-brand-600">
                    {it.name}
                  </Link>
                  <p className="text-xs text-text-muted">
                    {formatPaise(it.unit_price_paise)} × {it.qty}
                  </p>
                </div>
                <span className="font-medium text-sm shrink-0">{formatPaise(lineTotal)}</span>
              </li>
            );
          })}
        </ul>
      </section>

      <section aria-labelledby="price-h" className="border border-border rounded-lg p-4">
        <h2 id="price-h" className="font-semibold mb-3">Price breakdown</h2>
        <dl className="space-y-1">
          {data.charges.map((c) => <ChargeRow key={c.label} line={c} />)}
          <div className="border-t border-border mt-3 pt-3 flex justify-between font-semibold">
            <dt>Total</dt>
            <dd>{formatPaise(data.total_paise)}</dd>
          </div>
        </dl>
      </section>

      <section aria-labelledby="deliv-h" className="border border-border rounded-lg p-4">
        <h2 id="deliv-h" className="font-semibold mb-2">Delivery</h2>
        <p className="text-sm">{data.delivery_address.name} · {data.delivery_address.phone}</p>
        <p className="text-sm text-text-muted">
          {data.delivery_address.line1}
          {data.delivery_address.line2 ? `, ${data.delivery_address.line2}` : ""}, {data.delivery_address.city}, {data.delivery_address.state} {data.delivery_address.pincode}
        </p>
      </section>

      {canCancel && cancelState === "idle" && (
        <button
          type="button"
          onClick={() => setCancelState("confirming")}
          className="h-10 px-6 rounded border border-danger text-danger hover:bg-danger hover:text-white print:hidden"
        >
          Cancel order
        </button>
      )}
      {canCancel && cancelState !== "idle" && (
        <div role="alertdialog" aria-label="Confirm cancellation" className="border border-danger rounded-lg p-4 flex flex-wrap items-center gap-3 print:hidden">
          <p className="text-sm flex-1 min-w-0">Cancel this order? This cannot be undone.</p>
          <button
            type="button"
            onClick={() => setCancelState("idle")}
            disabled={cancelState === "cancelling"}
            autoFocus
            className="h-9 px-4 rounded border border-border disabled:opacity-60"
          >
            Keep order
          </button>
          <button
            type="button"
            onClick={onConfirmCancel}
            disabled={cancelState === "cancelling"}
            className="h-9 px-4 rounded bg-danger text-white disabled:opacity-60"
          >
            {cancelState === "cancelling" ? "Cancelling…" : "Yes, cancel"}
          </button>
        </div>
      )}
    </article>
  );
}
