export type Banner = {
  id: string;
  title: string;
  subtitle?: string;
  image_url?: string;
  cta_label?: string;
  cta_link?: string;
  event_key?: string;
  starts_at: string;
  ends_at: string;
  status: "draft" | "published" | "archived";
  sort_order: number;
  is_hero: boolean;
};

export type ActiveBanners = {
  hero: Banner | null;
  carousel: Banner[];
};

export type Category = {
  id: string;
  name: string;
  slug: string;
  icon_url?: string;
};

export type ProductCard = {
  id: string;
  slug: string;
  name: string;
  price_paise: number;
  image_url?: string;
  available_qty: number;
  category_slug?: string;
};

export type FeedPageInfo = {
  tier: "category" | "related" | "popular" | "random";
  page: number;
};

export type FeedPage = {
  items: ProductCard[];
  next_cursor?: string;
  page_info: FeedPageInfo;
};

export type ProductDetail = ProductCard & {
  description: string;
  image_urls: string[];
  gst_rate: number;
  category_name?: string;
};

export type ProductSort = "newest" | "price_asc" | "price_desc";

const PRODUCT_SORTS = ["newest", "price_asc", "price_desc"] as const;
export function isProductSort(s: string): s is ProductSort {
  return (PRODUCT_SORTS as readonly string[]).includes(s);
}

export type ProductListQuery = {
  category?: string;
  search?: string;
  sort?: ProductSort;
  in_stock?: boolean;
  cursor?: string;
  limit?: number;
};

export type ProductListResult = {
  items: ProductCard[];
  total_count: number;
  limit: number;
  next_cursor?: string;
};
