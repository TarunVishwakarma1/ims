import { serverFetch, safeJson } from "@/lib/api";
import { SeasonalHero } from "@/components/home/seasonal-hero";
import { BannerHero } from "@/components/home/banner-hero";
import { BannerCarousel } from "@/components/home/banner-carousel";
import { CategoryRow } from "@/components/home/category-row";
import { InfiniteFeed } from "@/components/home/infinite-feed";
import type { ActiveBanners, Category, FeedPage } from "@/lib/shop-types";

export const dynamic = "force-dynamic";

async function loadHomeData(shop: string) {
  const base = `/api/shop/s/${shop}`;
  // Each fetch is independent — network throws absorbed so the home page
  // renders an empty shell instead of 500-ing when the backend is down.
  const [banners, categories, feed] = await Promise.all([
    safeJson<ActiveBanners>(serverFetch(`${base}/banners/active`), {
      hero: null,
      carousel: [],
    }),
    safeJson<Category[]>(serverFetch(`${base}/categories`), []),
    safeJson<FeedPage>(serverFetch(`${base}/feed?limit=24`), {
      items: [],
      page_info: { tier: "category", page: 1 },
    }),
  ]);
  return { banners, categories, feed };
}

export default async function ShopHomePage({
  params,
}: {
  params: Promise<{ shop: string }>;
}) {
  const { shop } = await params;
  const { banners, categories, feed } = await loadHomeData(shop);
  return (
    <div className="space-y-8">
      {/* Admin-scheduled banner (festive/promo) owns the hero slot during its
          window; otherwise the code-driven seasonal band is the default. */}
      {banners.hero ? <BannerHero banner={banners.hero} /> : <SeasonalHero shopSlug={shop} />}
      {banners.carousel.length > 0 && (
        <BannerCarousel banners={banners.carousel} />
      )}
      {categories.length > 0 && <CategoryRow categories={categories} shopSlug={shop} />}
      <InfiniteFeed initialPage={feed} />
    </div>
  );
}
