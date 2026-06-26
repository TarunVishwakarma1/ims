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
  category_slug?: string;
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

// ── Cart ────────────────────────────────────────────────────────────────

export type CartItem = {
  product_id: string;
  slug: string;
  name: string;
  image: string;
  qty: number;
  unit_price_paise: number;
  max_qty: number;
};

export type Cart = {
  items: CartItem[];
  subtotal_paise: number;
  item_count: number;
  removed_items?: string[];
};

// ── Checkout ────────────────────────────────────────────────────────────

export type CheckoutSummary = {
  items: CartItem[];
  subtotal_paise: number;
  gst_paise: number;
  shipping_paise: number;
  total_payable_paise: number;
};

export type PaymentOption =
  | { id: "razorpay"; enabled: boolean; reason?: string }
  | {
      id: "cod";
      enabled: boolean;
      min_paise: number;
      max_paise: number;
      reason?: "min_value_below" | "max_value_exceeded" | "";
    };

export type PaymentOptionsResponse = { methods: PaymentOption[] };

export type PlaceOrderInput = {
  address_id: string;
  payment_method: "razorpay" | "cod";
  notes?: string;
};

export type PlaceOrderResult = {
  order_id: string;
  payable_paise: number;
  invoice_number: string;
  razorpay_order_id?: string;
  razorpay_key_id?: string;
};

export type VerifyRazorpayInput = {
  order_id: string;
  razorpay_payment_id: string;
  razorpay_order_id: string;
  razorpay_signature: string;
};

export type VerifyRazorpayResult = {
  order_id: string;
  status: string;
  payment_status: string;
  invoice_number: string;
};

// ── Address ─────────────────────────────────────────────────────────────

export type Address = {
  id: string;
  name: string;
  phone: string;
  line1: string;
  line2?: string;
  city: string;
  state: string;
  pincode: string;
  is_default: boolean;
};

export type AddressInput = Omit<Address, "id" | "is_default">;

// ── Orders ──────────────────────────────────────────────────────────────

export type OrderStatus =
  | "pending"
  | "confirmed"
  | "shipped"
  | "delivered"
  | "cancelling"
  | "cancelled";

export type PaymentStatus = "unpaid" | "paid" | "refunded";

export type OrderListItem = {
  id: string;
  invoice_number: string;
  status: OrderStatus;
  payment_status: PaymentStatus;
  payment_method?: "razorpay" | "cod";
  total_paise: number;
  item_count: number;
  created_at: string;
  first_item_name?: string;
  first_item_image?: string;
};

export type OrderListResult = {
  items: OrderListItem[];
  next_cursor?: string;
};

export type ChargeLine = {
  label: string;
  paise: number;
  /** What the customer would have paid if the line wasn't waived. */
  original_paise?: number;
  struck: boolean;
};

export type TimelineEvent = {
  at: string;
  status: string;
  note?: string;
};

export type OrderDetail = OrderListItem & {
  items: CartItem[];
  charges: ChargeLine[];
  timeline: TimelineEvent[];
  delivery_address: Address;
  notes?: string;
};
