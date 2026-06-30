export type UUID = string;
export type ISODate = string;

// Role is the user's role name. System roles ship as 'admin' | 'manager' |
// 'staff' but custom roles can be created at runtime via the roles admin
// page, so the runtime value is any non-empty string. The string union is
// kept only for badge styling lookups in older components.
export type Role = string;
export type OrderStatus = "pending" | "accepted" | "rejected" | "processing" | "ready" | "shipped" | "delivered" | "completed" | "refunded" | "confirmed" | "cancelled";

export interface Organization {
  id: UUID;
  name: string;
  slug: string;
  plan_type: string;
  is_active: boolean;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface User {
  id: UUID;
  org_id: UUID;
  name: string;
  email: string;
  role: Role;
  is_active: boolean;
  email_verified: boolean;
  last_login_at?: ISODate | null;
  totp_enabled?: boolean;
  totp_verified_at?: ISODate | null;
  email_2fa_enabled?: boolean;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface Category {
  id: UUID;
  name: string;
  description: string;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface Product {
  id: UUID;
  category_id: UUID;
  name: string;
  description: string;
  sku: string;
  price: number;
  gst_rate: number; // percent (0..28); 0 = exempt
  created_at: ISODate;
  updated_at: ISODate;
}

export interface Inventory {
  id: UUID;
  product_id: UUID;
  quantity: number;
  low_stock_threshold: number;
  updated_at: ISODate;
}

export type PaymentLifecycleStatus = 'unpaid' | 'paid' | 'refunded' | 'partial';

export interface Order {
  id: UUID;
  user_id: UUID;
  status: OrderStatus;
  total_amount: number;
  order_type: string;
  payment_status: PaymentLifecycleStatus;
  payment_id?: string | null;
  subtotal?: number;
  delivery_fee?: number;
  discount?: number;
  tax_amount?: number;
  tax_cgst?: number;
  tax_sgst?: number;
  tax_igst?: number;
  is_inter_state?: boolean;
  invoice_number?: string | null;
  buyer_org_id?: UUID | null;
  supplier_org_id?: UUID | null;
  buyer_org_name?: string;
  supplier_org_name?: string;
  accepted_at?: ISODate | null;
  shipped_at?: ISODate | null;
  delivered_at?: ISODate | null;
  completed_at?: ISODate | null;
  cancelled_at?: ISODate | null;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface OrderItem {
  id: UUID;
  order_id: UUID;
  product_id: UUID;
  product_name?: string;
  quantity: number;
  unit_price: number;
}

export interface AuditLog {
  id: UUID;
  user_id: UUID | null;
  action: string;
  entity: string;
  entity_id: UUID;
  ip_address: string;
  created_at: ISODate;
}

export interface LoginResponse {
  access_token?: string;
  refresh_token?: string;
  user?: User;
  organization?: Organization;
  // 2FA second-step plumbing.
  require_totp?: boolean;
  pending_token?: string;
  two_fa_method?: 'totp' | 'email' | string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface CreateUserRequest {
  name: string;
  email: string;
  password: string;
  role: Role;
}

export interface SignupRequest {
  org_name: string;
  org_slug: string;
  user_name: string;
  email: string;
  password: string;
}

export interface UpdateUserRequest {
  name?: string;
  email?: string;
  role?: Role;
  is_active?: boolean;
}

export interface CreateCategoryRequest {
  name: string;
  description: string;
}

export interface UpdateCategoryRequest {
  name?: string;
  description?: string;
}

export interface CreateProductRequest {
  category_id: UUID;
  name: string;
  description: string;
  sku: string;
  price: number; // in paise
  gst_rate?: number; // percent (0..28)
}

export interface UpdateProductRequest {
  category_id?: UUID;
  name?: string;
  description?: string;
  sku?: string;
  price?: number; // in paise
  gst_rate?: number;
}

export interface UpdateInventoryRequest {
  quantity?: number;
  low_stock_threshold?: number;
}

export interface CreateOrderItemRequest {
  product_id: UUID;
  quantity: number;
}

export interface CreateOrderRequest {
  items: CreateOrderItemRequest[];
}

export interface UpdateOrderStatusRequest {
  status: OrderStatus;
}

export interface ApiError {
  error: string;
}

// Org Location
export interface OrgLocation {
  id: UUID;
  org_id: UUID;
  name: string;
  address: string;
  city: string;
  state: string;
  country: string;
  postal_code: string;
  lat: number | null;
  lng: number | null;
  is_primary: boolean;
  is_active: boolean;
  created_at: ISODate;
  updated_at: ISODate;
}

// Marketplace
export interface MarketplaceListing {
  id: UUID;
  org_id: UUID;
  product_id: UUID;
  location_id: UUID | null;
  listing_price: number;  // paise
  min_order_qty: number;
  max_order_qty: number | null;
  is_active: boolean;
  created_at: ISODate;
  updated_at: ISODate;
  // joined fields
  product_name?: string;
  product_sku?: string;
  org_name?: string;
  org_slug?: string;
  location_name?: string;
  location_city?: string;
  location_lat?: number | null;
  location_lng?: number | null;
  stock_quantity?: number;
  distance_km?: number | null;
}

export interface CartItem {
  id: UUID;
  cart_id: UUID;
  listing_id: UUID;
  quantity: number;
  added_at: ISODate;
  listing?: MarketplaceListing;
}

export interface Cart {
  id: UUID;
  buyer_org_id: UUID | null;
  customer_id: UUID | null;
  expires_at: ISODate;
  created_at: ISODate;
  items?: CartItem[];
}

// Request types
export interface CreateListingRequest {
  product_id: UUID;
  location_id?: UUID;
  listing_price: number;  // rupees on frontend → paise on submit
  min_order_qty: number;
  max_order_qty?: number;
}

export interface AddToCartRequest {
  listing_id: UUID;
  quantity: number;
}

export interface MarketplaceSearchParams {
  q?: string;
  lat?: number;
  lng?: number;
  radius?: number;
  min_price?: number;
  max_price?: number;
}

export interface CreateLocationRequest {
  name: string;
  address?: string;
  city?: string;
  state?: string;
  country?: string;
  postal_code?: string;
  lat?: number;
  lng?: number;
  is_primary?: boolean;
}

// Payments
export type PaymentStatus = 'created' | 'authorized' | 'captured' | 'failed' | 'partially_refunded' | 'refunded';

export interface Payment {
  id: UUID;
  org_id: UUID;
  order_id: UUID | null;
  razorpay_order_id: string;
  razorpay_payment_id?: string | null;
  amount: number; // paise
  amount_refunded: number; // paise — cumulative across refunds
  currency: string;
  status: PaymentStatus;
  method?: string | null;
  failure_reason?: string | null;
  is_mock: boolean;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface Coupon {
  id: UUID;
  org_id: UUID;
  code: string;
  discount_type: 'percent' | 'fixed';
  discount_value: number;
  min_subtotal: number;
  max_uses: number | null;
  usage_count: number;
  expires_at: ISODate | null;
  is_active: boolean;
  description: string | null;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface CouponValidateResponse {
  coupon: Coupon;
  amount_off: number;
}

export interface Refund {
  id: UUID;
  payment_id: UUID;
  org_id: UUID;
  amount: number; // paise
  razorpay_refund_id: string | null;
  status: 'processed' | 'failed' | 'pending';
  reason: string | null;
  created_at: ISODate;
}

export interface CreatePaymentOrderResponse {
  payment: Payment;
  razorpay_order_id: string;
  amount: number;
  currency: string;
  mock: boolean;
}
// ── Banners (home CMS) ──────────────────────────────────────────────────
export type BannerStatus = "draft" | "published" | "archived";

export interface Banner {
  id: UUID;
  title: string;
  subtitle?: string;
  image_url?: string;
  cta_label?: string;
  cta_link?: string;
  event_key?: string;
  starts_at: ISODate;
  ends_at: ISODate;
  status: BannerStatus;
  sort_order: number;
  is_hero: boolean;
  category_slug?: string;
}

export interface BannerInput {
  title: string;
  subtitle?: string;
  image_url?: string;
  cta_label?: string;
  cta_link?: string;
  event_key?: string;
  starts_at: ISODate;
  ends_at: ISODate;
  sort_order?: number;
  is_hero?: boolean;
  category_slug?: string;
}
