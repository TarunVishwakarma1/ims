import { serverFetch } from "@/lib/api";
import { BannerHero } from "@/components/home/banner-hero";
import { BannerCarousel } from "@/components/home/banner-carousel";
import { CategoryRow } from "@/components/home/category-row";
import { InfiniteFeed } from "@/components/home/infinite-feed";
import type { ActiveBanners, Category, FeedPage } from "@/lib/shop-types";

export const dynamic = "force-dynamic";

async function loadHomeData() {
  const [bannersRes, catsRes, feedRes] = await Promise.all([
    serverFetch("/api/shop/banners/active"),
    serverFetch("/api/shop/categories"),
    serverFetch("/api/shop/feed?limit=24"),
  ]);
  const banners: ActiveBanners = bannersRes.ok
    ? await bannersRes.json()
    : { hero: null, carousel: [] };
  const categories: Category[] = catsRes.ok ? await catsRes.json() : [];
  const feed: FeedPage = feedRes.ok
    ? await feedRes.json()
    : { items: [], page_info: { tier: "category", page: 1 } };
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
