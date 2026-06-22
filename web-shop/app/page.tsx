import { serverFetch, safeJson } from "@/lib/api";
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

export default async function HomePage() {
  const { banners, categories, feed } = await loadHomeData();
  return (
    <div className="space-y-8">
      {banners.hero && <BannerHero banner={banners.hero} />}
      {banners.carousel.length > 0 && (
        <BannerCarousel banners={banners.carousel} />
      )}
      {categories.length > 0 && <CategoryRow categories={categories} />}
      <InfiniteFeed initialPage={feed} />
    </div>
  );
}
