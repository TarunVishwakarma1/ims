'use client'

import { useMemo } from 'react'
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { OrgLocation } from '@/types/api'

// Default Leaflet icon (CDN URLs so we don't ship bundled assets)
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

// Primary marker — gold star marker for is_primary location
const PrimaryIcon = L.divIcon({
  className: '',
  html: `<div style="
    background:#f59e0b;
    border:3px solid #fff;
    border-radius:50%;
    width:22px;height:22px;
    box-shadow:0 0 0 2px #f59e0b, 0 2px 4px rgba(0,0,0,0.3);
    display:flex;align-items:center;justify-content:center;
    color:white;font-size:12px;font-weight:bold;
  ">★</div>`,
  iconSize: [22, 22],
  iconAnchor: [11, 11],
})

interface LocationsMapInnerProps {
  locations: OrgLocation[]
  height?: string
}

export function LocationsMapInner({ locations, height = '400px' }: LocationsMapInnerProps) {
  // Only keep locations that actually have coordinates
  const pinned = useMemo(
    () =>
      locations.filter(
        l => typeof l.lat === 'number' && typeof l.lng === 'number' && l.lat !== 0 && l.lng !== 0,
      ),
    [locations],
  )

  // Center the map: primary location wins, else first pinned, else India default
  const center: [number, number] = useMemo(() => {
    if (pinned.length === 0) return [20.5937, 78.9629]
    const primary = pinned.find(l => l.is_primary)
    const target = primary ?? pinned[0]
    return [target.lat!, target.lng!]
  }, [pinned])

  // Compute a sensible zoom: if many pins, zoom out; single pin, zoom in
  const zoom = pinned.length === 0 ? 5 : pinned.length === 1 ? 13 : 6

  if (pinned.length === 0) {
    return (
      <div
        className="rounded-md border bg-muted/40 flex items-center justify-center text-sm text-muted-foreground"
        style={{ height }}
      >
        No locations have coordinates yet. Add lat/lng to see them on the map.
      </div>
    )
  }

  return (
    <div className="rounded-md border overflow-hidden" style={{ height }}>
      <MapContainer
        center={center}
        zoom={zoom}
        style={{ height: '100%', width: '100%' }}
        scrollWheelZoom
      >
        <TileLayer
          attribution='© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {pinned.map(l => (
          <Marker
            key={l.id}
            position={[l.lat!, l.lng!]}
            icon={l.is_primary ? PrimaryIcon : DefaultIcon}
          >
            <Popup>
              <div className="text-xs space-y-1 min-w-[150px]">
                <div className="font-semibold flex items-center gap-1">
                  {l.name}
                  {l.is_primary ? <span className="text-amber-500">★</span> : null}
                </div>
                {l.address ? <div>{l.address}</div> : null}
                <div className="text-muted-foreground">
                  {[l.city, l.state, l.country].filter(Boolean).join(', ') || '—'}
                </div>
                <div className="text-muted-foreground">
                  {l.lat!.toFixed(4)}, {l.lng!.toFixed(4)}
                </div>
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  )
}
