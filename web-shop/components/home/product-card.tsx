"use client";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { paiseToINR } from "@/lib/format";
import { useCartStore } from "@/lib/cart-store";
import { useShopHref } from "@/lib/use-shop-slug";
import type { ProductCard as ProductCardType } from "@/lib/shop-types";

export function ProductCard({ product }: { product: ProductCardType }) {
  const outOfStock = product.available_qty <= 0;
  const shopHref = useShopHref();
  const href = shopHref(`/p/${product.slug}`);
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
        "group rounded-2xl bg-surface border border-border overflow-hidden transition-all duration-200",
        "hover:-translate-y-0.5 hover:shadow-lg hover:border-brand-300",
        outOfStock && "opacity-70",
      )}
    >
      <div className="relative aspect-square bg-brand-50 overflow-hidden">
        <Link href={href} className="block absolute inset-0" aria-label={product.name}>
          {product.image_url ? (
            <Image
              src={product.image_url}
              alt=""
              fill
              sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 25vw"
              className="object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="size-full bg-brand-100" />
          )}
        </Link>
        {outOfStock && (
          <span className="absolute top-2 left-2 z-10 rounded-full bg-fg/80 px-2 py-0.5 text-[10px] font-medium text-bg">
            Out of stock
          </span>
        )}
        <button
          type="button"
          onClick={handleAdd}
          disabled={outOfStock}
          aria-label={`Add ${product.name} to cart`}
          className={cn(
            "absolute bottom-2 right-2 z-10 size-9 rounded-full bg-brand-600 text-white",
            "grid place-items-center shadow-md transition-transform hover:bg-brand-700 hover:scale-105 active:scale-95",
            "disabled:bg-muted disabled:text-text-muted disabled:cursor-not-allowed disabled:shadow-none",
          )}
        >
          <Plus className="size-4" aria-hidden />
        </button>
      </div>
      <Link href={href} className="block p-3 space-y-1">
        <h3 className="text-sm font-medium line-clamp-2 min-h-[2.5rem] group-hover:text-brand-700 transition-colors">
          {product.name}
        </h3>
        <p className="text-base font-semibold text-brand-700">
          {paiseToINR(product.price_paise)}
        </p>
      </Link>
    </article>
  );
}
