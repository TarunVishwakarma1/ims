'use client'

import dynamic from 'next/dynamic'
import type { OrgLocation } from '@/types/api'

interface LocationsMapProps {
  locations: OrgLocation[]
  height?: string
}

const LocationsMapInner = dynamic(
  () => import('./locations-map-inner').then(m => m.LocationsMapInner),
  {
    ssr: false,
    loading: () => (
      <div className="rounded-md border bg-muted/50 flex items-center justify-center text-sm text-muted-foreground h-[400px]">
        Loading map...
      </div>
    ),
  },
)

export function LocationsMap(props: LocationsMapProps) {
  return <LocationsMapInner {...props} />
}
