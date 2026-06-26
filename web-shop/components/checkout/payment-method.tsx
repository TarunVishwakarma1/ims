"use client";

import type { PaymentOption } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";

type Method = "razorpay" | "cod";
type Props = {
  options: PaymentOption[];
  value: Method;
  onChange: (m: Method) => void;
};

function codReason(o: Extract<PaymentOption, { id: "cod" }>): string {
  if (o.enabled) return "";
  if (o.reason === "min_value_below") return `Cart total below ${formatPaise(o.min_paise)} minimum.`;
  if (o.reason === "max_value_exceeded") return `Cart total exceeds ${formatPaise(o.max_paise)} COD limit.`;
  return "Not available";
}

export function PaymentMethod({ options, value, onChange }: Props) {
  const rzp = options.find((o) => o.id === "razorpay");
  const cod = options.find((o): o is Extract<PaymentOption, { id: "cod" }> => o.id === "cod");
  return (
    <section aria-labelledby="pm-h">
      <h2 id="pm-h" className="font-semibold mb-3">Payment method</h2>
      <div role="radiogroup" aria-label="Payment method" className="space-y-2">
        {rzp && (
          <label className={`block p-3 rounded border ${value === "razorpay" ? "border-brand-600 bg-brand-50" : "border-border"} ${!rzp.enabled ? "opacity-50" : ""}`}>
            <input
              type="radio"
              name="pm"
              value="razorpay"
              checked={value === "razorpay"}
              disabled={!rzp.enabled}
              onChange={() => onChange("razorpay")}
              className="mr-2"
            />
            <span className="font-medium">Card / UPI / Wallet (Razorpay)</span>
            <div className="text-xs text-text-muted mt-1">Secure payment via Razorpay.</div>
          </label>
        )}
        {cod && (
          <label className={`block p-3 rounded border ${value === "cod" ? "border-brand-600 bg-brand-50" : "border-border"} ${!cod.enabled ? "opacity-50 cursor-not-allowed" : ""}`}>
            <input
              type="radio"
              name="pm"
              value="cod"
              checked={value === "cod"}
              disabled={!cod.enabled}
              onChange={() => onChange("cod")}
              className="mr-2"
            />
            <span className="font-medium">Cash on Delivery</span>
            <div className="text-xs text-text-muted mt-1">
              {cod.enabled ? `Pay in cash on delivery. ${formatPaise(cod.min_paise)}–${formatPaise(cod.max_paise)}.` : codReason(cod)}
            </div>
          </label>
        )}
      </div>
    </section>
  );
}
