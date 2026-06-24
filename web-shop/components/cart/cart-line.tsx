"use client";

import Image from "next/image";
import Link from "next/link";
import { Minus, Plus, Trash2 } from "lucide-react";
import type { CartItem } from "@/lib/shop-types";
import { formatPaise } from "@/lib/format";

type Props = {
  item: CartItem;
  onQtyChange: (qty: number) => void;
  onRemove: () => void;
  disabled?: boolean;
};

export function CartLine({ item, onQtyChange, onRemove, disabled }: Props) {
  return (
    <div className="flex gap-3 py-3 border-b border-border last:border-0">
      <Link href={`/p/${item.slug}`} className="relative w-16 h-16 shrink-0 bg-bg-muted rounded overflow-hidden">
        {item.image && (
          <Image src={item.image} alt={item.name} fill className="object-cover" sizes="64px" />
        )}
      </Link>
      <div className="flex-1 min-w-0">
        <Link href={`/p/${item.slug}`} className="block text-sm font-medium line-clamp-2 hover:text-brand-600">
          {item.name}
        </Link>
        <div className="text-sm text-text-muted mt-1">{formatPaise(item.unit_price_paise)}</div>
        <div className="flex items-center gap-2 mt-2">
          <div className="inline-flex items-center border border-border rounded">
            <button
              type="button"
              onClick={() => onQtyChange(item.qty - 1)}
              disabled={disabled || item.qty <= 1}
              aria-label="Decrease quantity"
              className="size-7 grid place-items-center disabled:opacity-50"
            >
              <Minus className="size-3" />
            </button>
            <span aria-live="polite" className="px-2 text-sm min-w-[2rem] text-center">
              {item.qty}
            </span>
            <button
              type="button"
              onClick={() => onQtyChange(item.qty + 1)}
              disabled={disabled || item.qty >= item.max_qty}
              aria-label="Increase quantity"
              className="size-7 grid place-items-center disabled:opacity-50"
            >
              <Plus className="size-3" />
            </button>
          </div>
          <button
            type="button"
            onClick={onRemove}
            disabled={disabled}
            aria-label={`Remove ${item.name}`}
            className="size-7 grid place-items-center text-text-muted hover:text-danger"
          >
            <Trash2 className="size-3" />
          </button>
        </div>
      </div>
      <div className="text-sm font-medium shrink-0">
        {formatPaise(item.qty * item.unit_price_paise)}
      </div>
    </div>
  );
}
