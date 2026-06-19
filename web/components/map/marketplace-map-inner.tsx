'use client'

import { useMemo } from 'react'
import { MapContainer, TileLayer, Marker, Popup, Circle } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { MarketplaceListing } from '@/types/api'
import { formatPrice } from '@/lib/utils'

const DefaultIcon = L.icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
})
L.Marker.prototype.options.icon = DefaultIcon

const UserIcon = L.divIcon({
  className: '',
  html: `<div style="background:#10b981;border:3px solid #fff;border-radius:50%;width:16px;height:16px;box-shadow:0 0 0 2px #10b981;"></div>`,
  iconSize: [16, 16],
  iconAnchor: [8, 8],
})

interface MarketplaceMapInnerProps {
  listings: MarketplaceListing[]
  userLat?: number
  userLng?: number
  height?: string
  onListingClick?: (listing: MarketplaceListing) => void
}

export function MarketplaceMapInner({
  listings,
  userLat,
  userLng,
  height = '400px',
  onListingClick,
}: MarketplaceMapInnerProps) {
  const hasUser = typeof userLat === 'number' && typeof userLng === 'number'

  type Pinned = MarketplaceListing & { _lat: number; _lng: number }
  const pinned = useMemo<Pinned[]>(() => {
    return listings.flatMap(l => {
      if (typeof l.location_lat === 'number' && typeof l.location_lng === 'number') {
        return [{ ...l, _lat: l.location_lat, _lng: l.location_lng }]
      }
      return []
    })
  }, [listings])

  const center: [number, number] = hasUser
    ? [userLat!, userLng!]
    : pinned.length > 0
    ? [pinned[0]._lat, pinned[0]._lng]
    : [20.5937, 78.9629] // India default

  return (
    <div className="rounded-md border overflow-hidden" style={{ height }}>
      <MapContainer center={center} zoom={hasUser ? 12 : 5} style={{ height: '100%', width: '100%' }} scrollWheelZoom>
        <TileLayer
          attribution='© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />

        {hasUser ? (
          <>
            <Marker position={[userLat!, userLng!]} icon={UserIcon}>
              <Popup>You are here</Popup>
            </Marker>
            <Circle
              center={[userLat!, userLng!]}
              radius={5000}
              pathOptions={{ color: '#10b981', fillOpacity: 0.05 }}
            />
          </>
        ) : null}

        {pinned.map(l => (
          <Marker
            key={l.id}
            position={[l._lat, l._lng]}
            eventHandlers={{
              click: () => onListingClick?.(l),
            }}
          >
            <Popup>
              <div className="text-xs space-y-1">
                <div className="font-semibold">{l.product_name}</div>
                <div className="text-muted-foreground">{l.org_name}</div>
                <div>{formatPrice(l.listing_price)}</div>
                {typeof l.distance_km === 'number' ? (
                  <div className="text-muted-foreground">{l.distance_km.toFixed(1)} km away</div>
                ) : null}
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  )
}
