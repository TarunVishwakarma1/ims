import { api } from './client'
import type { 
  MarketplaceListing, 
  Cart, 
  CartItem, 
  Order, 
  CreateListingRequest, 
  AddToCartRequest, 
  MarketplaceSearchParams 
} from '@/types/api'

export const marketplaceApi = {
  search: (params: MarketplaceSearchParams) =>
    api.get('marketplace/search', { searchParams: params as any }).json<MarketplaceListing[]>(),
  listByOrg: () => api.get('listings').json<MarketplaceListing[]>(),
  createListing: (data: CreateListingRequest) => api.post('listings', { json: data }).json<MarketplaceListing>(),
  updateListing: (id: string, data: Partial<CreateListingRequest>) => api.put(`listings/${id}`, { json: data }).json<MarketplaceListing>(),
  deleteListing: (id: string) => api.delete(`listings/${id}`),
  getCart: () => api.get('cart').json<Cart>(),
  addToCart: (data: AddToCartRequest) => api.post('cart/items', { json: data }).json<CartItem>(),
  updateCartItem: (listingId: string, quantity: number) => api.put(`cart/items/${listingId}`, { json: { quantity } }).json<CartItem>(),
  removeFromCart: (listingId: string) => api.delete(`cart/items/${listingId}`),
  checkout: (deliveryAddressId?: string) => api.post('cart/checkout', { json: { delivery_address_id: deliveryAddressId } }).json<Order[]>(),
}
