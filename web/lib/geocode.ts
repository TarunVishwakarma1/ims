export interface GeocodeResult {
  area: string
  city: string
  pincode: string
}

interface NominatimResponse {
  address?: Record<string, string>
}

// Pure mapper: pick the best-available area/city/pincode from Nominatim's
// address object. Kept separate from the fetch so it's unit-testable.
export function mapNominatim(json: NominatimResponse): GeocodeResult {
  const a = json.address ?? {}
  const area = a.suburb || a.neighbourhood || a.village || a.hamlet || a.road || ''
  const city = a.city || a.town || a.municipality || a.county || a.state_district || ''
  const pincode = a.postcode || ''
  return { area, city, pincode }
}

// Reverse-geocode a coordinate. Goes through our /api/geocode/reverse proxy
// (which sets the Nominatim-required User-Agent server-side — the browser can't)
// and returns the already-mapped fields. Caller debounces (≥1s per Nominatim
// usage policy). Returns empty fields on any failure.
export async function reverseGeocode(lat: number, lng: number): Promise<GeocodeResult> {
  try {
    const res = await fetch(`/api/geocode/reverse?lat=${lat}&lng=${lng}`)
    if (!res.ok) return { area: '', city: '', pincode: '' }
    return (await res.json()) as GeocodeResult
  } catch {
    return { area: '', city: '', pincode: '' }
  }
}
