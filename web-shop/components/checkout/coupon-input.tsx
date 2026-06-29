"use client";

import { useState } from "react";
import { Tag, X } from "lucide-react";
import type { AppliedCoupon } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";

type Props = {
  applied?: AppliedCoupon;
  busy?: boolean;
  error?: string;
  onApply: (code: string) => void;
  onRemove: () => void;
};

export function CouponInput({ applied, busy, error, onApply, onRemove }: Props) {
  const [code, setCode] = useState("");

  if (applied) {
    return (
      <div className="flex items-center justify-between gap-2 rounded-md border border-emerald-300 bg-emerald-50 px-3 py-2 text-sm">
        <span className="flex items-center gap-2 font-medium text-emerald-700">
          <Tag className="size-4" />
          {applied.code}
          <span className="font-normal text-emerald-600">
            −{formatPaise(applied.amount_off_paise)} applied
          </span>
        </span>
        <button
          type="button"
          onClick={onRemove}
          aria-label="Remove coupon"
          className="rounded p-1 text-emerald-700 hover:bg-emerald-100"
        >
          <X className="size-4" />
        </button>
      </div>
    );
  }

  const submit = () => {
    const c = code.trim().toUpperCase();
    if (c) onApply(c);
  };

  return (
    <div className="space-y-1">
      <div className="flex gap-2">
        <input
          value={code}
          onChange={(e) => setCode(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && submit()}
          placeholder="Coupon code"
          autoCapitalize="characters"
          className="h-9 flex-1 rounded-md border border-border px-3 text-sm uppercase placeholder:normal-case"
        />
        <button
          type="button"
          onClick={submit}
          disabled={busy || !code.trim()}
          className="h-9 rounded-md bg-brand-600 px-4 text-sm font-medium text-white disabled:opacity-50"
        >
          {busy ? "…" : "Apply"}
        </button>
      </div>
      {error && <p className="text-xs text-red-600">{error}</p>}
    </div>
  );
}
