export type UUID = string;
export type ISODate = string;

export type Role = "admin" | "manager" | "staff";
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

export interface Order {
  id: UUID;
  user_id: UUID;
  status: OrderStatus;
  total_amount: number;
  order_type: string;
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
  access_token: string;
  refresh_token: string;
  user: User;
  organization: Organization;
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
}

export interface UpdateProductRequest {
  category_id?: UUID;
  name?: string;
  description?: string;
  sku?: string;
  price?: number; // in paise
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
export type PaymentStatus = 'created' | 'authorized' | 'captured' | 'failed' | 'refunded';

export interface Payment {
  id: UUID;
  org_id: UUID;
  order_id: UUID | null;
  razorpay_order_id: string;
  razorpay_payment_id?: string | null;
  amount: number; // paise
  currency: string;
  status: PaymentStatus;
  method?: string | null;
  failure_reason?: string | null;
  is_mock: boolean;
  created_at: ISODate;
  updated_at: ISODate;
}

export interface CreatePaymentOrderResponse {
  payment: Payment;
  razorpay_order_id: string;
  amount: number;
  currency: string;
  mock: boolean;
}