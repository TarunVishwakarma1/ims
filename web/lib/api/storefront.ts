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

  // Uploads a logo image and returns its served URL. FormData → ky sets the
  // multipart content-type automatically.
  uploadLogo: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return api.post('admin/storefront/logo', { body: fd }).json<{ logo_url: string }>()
  },
}
