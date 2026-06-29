"use client";

import { type ReactNode } from "react";
import type { CheckoutSummary } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";

type Props = { summary: CheckoutSummary; coupon?: ReactNode; action?: ReactNode };

export function OrderSummary({ summary, coupon, action }: Props) {
  return (
    <aside aria-labelledby="sum-h" className="lg:sticky lg:top-20 self-start border border-border rounded-lg p-4 space-y-3">
      <h2 id="sum-h" className="font-semibold">Order summary</h2>
      <ul className="text-sm divide-y divide-border">
        {summary.items.map((it) => (
          <li key={it.product_id} className="py-2 flex justify-between gap-3">
            <span className="line-clamp-1">{it.name} × {it.qty}</span>
            <span className="shrink-0">{formatPaise(it.qty * it.unit_price_paise)}</span>
          </li>
        ))}
      </ul>
      {coupon && <div className="pt-2 border-t border-border">{coupon}</div>}
      <dl className="text-sm space-y-1 pt-2 border-t border-border">
        <div className="flex justify-between"><dt>Subtotal</dt><dd>{formatPaise(summary.subtotal_paise)}</dd></div>
        <div className="flex justify-between"><dt>GST</dt><dd>{formatPaise(summary.gst_paise)}</dd></div>
        {summary.platform_paise > 0 && (
          <div className="flex justify-between"><dt>Platform fee</dt><dd>{formatPaise(summary.platform_paise)}</dd></div>
        )}
        <div className="flex justify-between"><dt>Shipping</dt><dd>{summary.shipping_paise === 0 ? "Free" : formatPaise(summary.shipping_paise)}</dd></div>
        {summary.discount_paise > 0 && (
          <div className="flex justify-between text-emerald-600">
            <dt>Discount{summary.coupon ? ` (${summary.coupon.code})` : ""}</dt>
            <dd>−{formatPaise(summary.discount_paise)}</dd>
          </div>
        )}
        <div className="flex justify-between font-semibold pt-2 border-t border-border"><dt>Total</dt><dd>{formatPaise(summary.total_payable_paise)}</dd></div>
      </dl>
      {action}
    </aside>
  );
}
