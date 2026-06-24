"use client";

import { useState } from "react";
import type { Address } from "@/lib/shop-types";

type Props = {
  addresses: Address[];
  selected: Address | null;
  onSelect: (a: Address) => void;
  onAddNew: () => void;
};

export function AddressPicker({ addresses, selected, onSelect, onAddNew }: Props) {
  return (
    <section aria-labelledby="addr-h">
      <h2 id="addr-h" className="font-semibold mb-3">Delivery address</h2>
      <ul className="space-y-2">
        {addresses.map((a) => {
          const isSel = selected?.id === a.id;
          return (
            <li key={a.id}>
              <button
                type="button"
                onClick={() => onSelect(a)}
                aria-pressed={isSel}
                className={`w-full text-left p-3 rounded border ${isSel ? "border-brand-600 bg-brand-50" : "border-border hover:bg-brand-50"}`}
              >
                <div className="font-medium text-sm">{a.name} · {a.phone}</div>
                <div className="text-sm text-text-muted">
                  {a.line1}{a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state} {a.pincode}
                </div>
                {a.is_default && <span className="inline-block mt-1 text-xs text-brand-600">Default</span>}
              </button>
            </li>
          );
        })}
      </ul>
      <button
        type="button"
        onClick={onAddNew}
        className="mt-3 text-sm text-brand-600 hover:underline"
      >
        + Add new address
      </button>
    </section>
  );
}
