"use client";
import Link from "next/link";
import Image from "next/image";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { ProductCard as ProductCardType } from "@/lib/shop-types";

function paiseToINR(paise: number) {
  return `₹${(paise / 100).toLocaleString("en-IN", { maximumFractionDigits: 0 })}`;
}

export function ProductCard({ product }: { product: ProductCardType }) {
  const outOfStock = product.available_qty <= 0;

  function handleAdd(e: React.MouseEvent) {
    e.preventDefault();
    if (outOfStock) return;
    // Plan 3e wires real cart action via /api/cart/add.
    toast("Cart coming soon", { description: product.name });
  }

  return (
    <Link
      href={`/p/${product.slug}`}
      className={cn(
        "group block rounded-2xl bg-bg border border-border overflow-hidden hover:shadow-md transition-shadow",
        outOfStock && "opacity-60",
      )}
    >
      <div className="relative aspect-square bg-brand-50">
        {product.image_url ? (
          <Image
            src={product.image_url}
            alt={product.name}
            fill
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 25vw"
            className="object-cover"
          />
        ) : (
          <div className="size-full bg-brand-100" />
        )}
        <button
          type="button"
          onClick={handleAdd}
          disabled={outOfStock}
          aria-label="Add to cart"
          className={cn(
            "absolute bottom-2 right-2 size-9 rounded-full bg-brand-600 text-white",
            "grid place-items-center shadow-md hover:bg-brand-700",
            "disabled:bg-muted disabled:cursor-not-allowed",
          )}
        >
          <Plus className="size-4" />
        </button>
      </div>
      <div className="p-3 space-y-1">
        <h3 className="text-sm font-medium line-clamp-2 min-h-[2.5rem]">
          {product.name}
        </h3>
        <p className="text-base font-semibold text-brand-700">
          {paiseToINR(product.price_paise)}
        </p>
        {outOfStock && <p className="text-xs text-muted">Out of stock</p>}
      </div>
    </Link>
  );
}
