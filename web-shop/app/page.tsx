import { serverFetch, safeJson } from "@/lib/api";
import { SeasonalHero } from "@/components/home/seasonal-hero";
import { FestiveBanner } from "@/components/home/festive-banner";
import { activeFestival } from "@/lib/festivals";
import { BannerHero } from "@/components/home/banner-hero";
import { BannerCarousel } from "@/components/home/banner-carousel";
import { CategoryRow } from "@/components/home/category-row";
import { InfiniteFeed } from "@/components/home/infinite-feed";
import type { ActiveBanners, Category, FeedPage } from "@/lib/shop-types";

export const dynamic = "force-dynamic";

async function loadHomeData() {
  // Each fetch is independent — network throws absorbed so the home page
  // renders an empty shell instead of 500-ing when the backend is down.
  const [banners, categories, feed] = await Promise.all([
    safeJson<ActiveBanners>(serverFetch("/api/shop/banners/active"), {
      hero: null,
      carousel: [],
    }),
    safeJson<Category[]>(serverFetch("/api/shop/categories"), []),
    safeJson<FeedPage>(serverFetch("/api/shop/feed?limit=24"), {
      items: [],
      page_info: { tier: "category", page: 1 },
    }),
  ]);
  return { banners, categories, feed };
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ festival?: string }>;
}) {
  const [{ banners, categories, feed }, sp] = await Promise.all([
    loadHomeData(),
    searchParams,
  ]);
  // A live festival takes over the hero; otherwise the seasonal band shows.
  // ?festival=<id> force-previews a specific festival banner.
  const festival = activeFestival(new Date(), sp.festival);
  return (
    <div className="space-y-8">
      {festival ? <FestiveBanner preview={sp.festival} /> : <SeasonalHero />}
      {banners.hero && <BannerHero banner={banners.hero} />}
      {banners.carousel.length > 0 && (
        <BannerCarousel banners={banners.carousel} />
      )}
      {categories.length > 0 && <CategoryRow categories={categories} />}
      <InfiniteFeed initialPage={feed} />
    </div>
  );
}
