import { api } from './client'
import type { Banner, BannerInput, UUID } from '@/types/api'

export const bannersApi = {
  list: (status?: string) =>
    api
      .get('admin/banners', { searchParams: status ? { status } : {} })
      .json<Banner[]>(),

  getById: (id: UUID) => api.get(`admin/banners/${id}`).json<Banner>(),

  create: (data: BannerInput) => api.post('admin/banners', { json: data }).json<Banner>(),

  update: (id: UUID, data: BannerInput) =>
    api.patch(`admin/banners/${id}`, { json: data }).json<Banner>(),

  publish: (id: UUID) => api.post(`admin/banners/${id}/publish`).json<Banner>(),

  archive: (id: UUID) => api.post(`admin/banners/${id}/archive`).json<Banner>(),

  // Uploads an image file and returns its served URL. FormData → ky sets the
  // multipart content-type automatically.
  upload: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return api.post('admin/banners/upload', { body: fd }).json<{ image_url: string }>()
  },
}
