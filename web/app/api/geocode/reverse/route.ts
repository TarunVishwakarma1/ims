import { NextResponse } from 'next/server'
import { mapNominatim } from '@/lib/geocode'

// Server-side proxy for OSM Nominatim reverse-geocoding.
//
// Nominatim's usage policy asks for an identifiable User-Agent, but the browser
// forbids setting that header on fetch() — so the request has to originate here
// (server), not from the client. lat/lng are coerced to finite, in-range
// numbers before being placed in the upstream URL to avoid query injection.
export async function GET(req: Request) {
  const { searchParams } = new URL(req.url)
  const lat = Number(searchParams.get('lat'))
  const lng = Number(searchParams.get('lng'))

  if (
    !Number.isFinite(lat) || !Number.isFinite(lng) ||
    lat < -90 || lat > 90 || lng < -180 || lng > 180
  ) {
    return NextResponse.json({ error: 'invalid coordinates' }, { status: 400 })
  }

  const url = `https://nominatim.openstreetmap.org/reverse?format=json&lat=${lat}&lon=${lng}&zoom=16&addressdetails=1`
  try {
    const res = await fetch(url, {
      headers: {
        'User-Agent': 'Kirana-Admin/1.0 (storefront onboarding; +https://kirana.local)',
        'Accept-Language': 'en',
      },
    })
    if (!res.ok) {
      return NextResponse.json({ area: '', city: '', pincode: '' }, { status: 502 })
    }
    return NextResponse.json(mapNominatim(await res.json()))
  } catch {
    return NextResponse.json({ area: '', city: '', pincode: '' }, { status: 502 })
  }
}
