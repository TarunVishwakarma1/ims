import { notFound } from "next/navigation";
import Link from "next/link";
import { serverFetch } from "@/lib/api";
import { ProductGallery } from "@/components/pdp/product-gallery";
import { PdpBuyBox } from "@/components/pdp/pdp-buy-box";
import { paiseToINR } from "@/lib/format";
import type { ProductDetail } from "@/lib/shop-types";

export const dynamic = "force-dynamic";

const SLUG_RE = /^[a-z0-9-]+$/;

type PageProps = { params: Promise<{ slug: string }> };

function FailShell() {
  return (
    <div className="space-y-4 text-center py-12">
      <p className="text-lg">Unable to load product.</p>
      <Link href="/" className="text-brand-600 hover:underline">
        Back to home
      </Link>
    </div>
  );
}

export default async function ProductPage({ params }: PageProps) {
  const { slug } = await params;
  if (!SLUG_RE.test(slug)) notFound();

  let res: Response | null = null;
  try {
    res = await serverFetch(`/api/shop/products/${slug}`);
  } catch {
    res = null;
  }

  if (!res) return <FailShell />;
  if (res.status === 404) notFound();
  if (!res.ok) return <FailShell />;

  let product: ProductDetail;
  try {
    product = (await res.json()) as ProductDetail;
  } catch {
    return <FailShell />;
  }

  const outOfStock = product.available_qty <= 0;
  const lowStock = product.available_qty > 0 && product.available_qty <= 5;
  const images = product.image_urls?.length
    ? product.image_urls
    : product.image_url
      ? [product.image_url]
      : [];

  return (
    <div className="space-y-6">
      <nav className="text-sm text-muted">
        <Link href="/" className="hover:underline">Home</Link>
        {product.category_slug &&
          SLUG_RE.test(product.category_slug) &&
          product.category_name && (
            <>
              <span className="mx-1">›</span>
              <Link href={`/c/${product.category_slug}`} className="hover:underline">
                {product.category_name}
              </Link>
            </>
          )}
        <span className="mx-1">›</span>
        <span aria-current="page">{product.name}</span>
      </nav>

      <section className="grid gap-8 lg:grid-cols-2">
        <ProductGallery images={images} alt={product.name} />

        <div className="space-y-4">
          <h1 className="text-3xl font-semibold">{product.name}</h1>
          <p className="text-2xl font-semibold text-brand-700">
            {paiseToINR(product.price_paise)}
          </p>

          <p className="text-sm">
            {outOfStock ? (
              <span className="text-red-600 font-medium">Out of stock</span>
            ) : lowStock ? (
              <span className="text-amber-700 font-medium">
                Only {product.available_qty} left
              </span>
            ) : (
              <span className="text-green-700 font-medium">In stock</span>
            )}
          </p>

          <PdpBuyBox
            item={{
              product_id: product.id,
              slug: product.slug,
              name: product.name,
              image: images[0] ?? "",
              unit_price_paise: product.price_paise,
              max_qty: product.available_qty,
            }}
            outOfStock={outOfStock}
          />

          <p className="text-xs text-muted">
            Inclusive of GST ({product.gst_rate}%)
          </p>

          {product.description && (
            <article className="prose prose-sm max-w-none whitespace-pre-line pt-4 border-t border-border">
              {product.description}
            </article>
          )}
        </div>
      </section>
    </div>
  );
}
