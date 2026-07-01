import { HTTPError } from 'ky'
import { api } from './client'
import type { ShopProfile, ShopProfileInput } from '@/types/api'

export const storefrontApi = {
  // Returns null when the org has no storefront yet (404).
  get: async (): Promise<ShopProfile | null> => {
    try {
      return await api.get('admin/storefront').json<ShopProfile>()
    } catch (e) {
      if (e instanceof HTTPError && e.response.status === 404) return null
      throw e
    }
  },

  upsert: (input: ShopProfileInput) =>
    api.put('admin/storefront', { json: input }).json<ShopProfile>(),
}
