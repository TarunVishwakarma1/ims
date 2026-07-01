export interface GoLiveFields {
  display_name: string
  lat: number | null
  lng: number | null
  pincodes: string[]
}

// Mirrors the backend go-live guard: name + map location + ≥1 pincode.
export function canGoLive(f: GoLiveFields): boolean {
  return (
    f.display_name.trim().length > 0 &&
    f.lat != null &&
    f.lng != null &&
    f.pincodes.length > 0
  )
}
