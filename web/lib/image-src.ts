// Resolve an image URL for display. Absolute URLs pass through; relative
// /uploads/... paths (returned by the backend disk store) get the API origin
// prefixed so they load from the backend, not the admin app's own host.
const API_ORIGIN = (process.env.NEXT_PUBLIC_API_URL || '').replace(/\/api\/?$/, '')

export function imageSrc(url?: string): string {
  if (!url) return ''
  if (/^https?:\/\//.test(url)) return url
  return `${API_ORIGIN}${url.startsWith('/') ? '' : '/'}${url}`
}
