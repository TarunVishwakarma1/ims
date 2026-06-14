'use client'

import dynamic from 'next/dynamic'
import type { MarketplaceListing } from '@/types/api'

interface MarketplaceMapProps {
  listings: MarketplaceListing[]
  userLat?: number
  userLng?: number
  height?: string
  onListingClick?: (listing: MarketplaceListing) => void
}

const MarketplaceMapInner = dynamic(
  () => import('./marketplace-map-inner').then(m => m.MarketplaceMapInner),
  {
    ssr: false,
    loading: () => (
      <div className="rounded-md border bg-muted/50 flex items-center justify-center text-sm text-muted-foreground h-[400px]">
        Loading map...
      </div>
    ),
  },
)

export function MarketplaceMap(props: MarketplaceMapProps) {
  return <MarketplaceMapInner {...props} />
}
