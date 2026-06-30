"use client";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { paiseToINR } from "@/lib/format";
import { useCartStore } from "@/lib/cart-store";
import type { ProductCard as ProductCardType } from "@/lib/shop-types";

export function ProductCard({ product }: { product: ProductCardType }) {
  const outOfStock = product.available_qty <= 0;
  const href = `/p/${product.slug}`;
  const add = useCartStore((s) => s.add);
  const router = useRouter();

  async function handleAdd() {
    if (outOfStock) return;
    try {
      await add(
        {
          product_id: product.id,
          slug: product.slug,
          name: product.name,
          image: product.image_url ?? "",
          unit_price_paise: product.price_paise,
          max_qty: product.available_qty,
          qty: 0,
        },
        1,
      );
      toast.success("Added to cart", {
        description: product.name,
        action: { label: "View cart", onClick: () => router.push("/cart") },
      });
    } catch {
      toast.error("Could not add to cart");
    }
  }

  return (
    <article
      className={cn(
        "group rounded-2xl bg-bg border border-border overflow-hidden hover:shadow-md transition-shadow",
        outOfStock && "opacity-60",
      )}
    >
      <div className="relative aspect-square bg-brand-50">
        <Link href={href} className="block absolute inset-0" aria-label={product.name}>
          {product.image_url ? (
            <Image
              src={product.image_url}
              alt=""
              fill
              sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 25vw"
              className="object-cover"
            />
          ) : (
            <div className="size-full bg-brand-100" />
          )}
        </Link>
        <button
          type="button"
          onClick={handleAdd}
          disabled={outOfStock}
          aria-label={`Add ${product.name} to cart`}
          className={cn(
            "absolute bottom-2 right-2 z-10 size-9 rounded-full bg-brand-600 text-white",
            "grid place-items-center shadow-md hover:bg-brand-700",
            "disabled:bg-muted disabled:cursor-not-allowed",
          )}
        >
          <Plus className="size-4" aria-hidden />
        </button>
      </div>
      <Link href={href} className="block p-3 space-y-1">
        <h3 className="text-sm font-medium line-clamp-2 min-h-[2.5rem]">
          {product.name}
        </h3>
        <p className="text-base font-semibold text-brand-700">
          {paiseToINR(product.price_paise)}
        </p>
        {outOfStock && <p className="text-xs text-text-muted">Out of stock</p>}
      </Link>
    </article>
  );
}
