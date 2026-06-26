"use client";
import { useState } from "react";
import { QtyStepper } from "./qty-stepper";
import { AddToCart } from "./add-to-cart";

type Props = { productSlug: string; max: number; outOfStock: boolean };

export function PdpBuyBox({ productSlug, max, outOfStock }: Props) {
  const [qty, setQty] = useState(1);
  return (
    <div className="flex flex-wrap items-center gap-4">
      <QtyStepper
        value={qty}
        onChange={setQty}
        max={Math.max(1, max)}
        disabled={outOfStock}
      />
      <AddToCart productSlug={productSlug} qty={qty} disabled={outOfStock} />
    </div>
  );
}
