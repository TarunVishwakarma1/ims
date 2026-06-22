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
