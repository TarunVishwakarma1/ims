export type UUID = string;
export type ISODate = string;

export type Role = "admin" | "manager" | "staff";
export type OrderStatus = "pending" | "confirmed" | "cancelled";

export interface User {
  id: UUID;
  name: string;
  email: string;
  role: Role;
  is_active: boolean;
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
  created_at: ISODate;
  updated_at: ISODate;
}

export interface OrderItem {
  id: UUID;
  order_id: UUID;
  product_id: UUID;
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