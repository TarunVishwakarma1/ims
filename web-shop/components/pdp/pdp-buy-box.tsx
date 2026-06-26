"use client";
import { useState } from "react";
import { QtyStepper } from "./qty-stepper";
import { AddToCart } from "./add-to-cart";
import type { CartItem } from "@/lib/shop-types";

type Props = {
  item: Omit<CartItem, "qty">;
  outOfStock: boolean;
};

export function PdpBuyBox({ item, outOfStock }: Props) {
  const [qty, setQty] = useState(1);
  const max = item.max_qty;
  return (
    <div className="flex flex-wrap items-center gap-4">
      <QtyStepper
        value={qty}
        onChange={setQty}
        max={Math.max(1, max)}
        disabled={outOfStock}
      />
      <AddToCart item={item} qty={qty} disabled={outOfStock} />
    </div>
  );
}
