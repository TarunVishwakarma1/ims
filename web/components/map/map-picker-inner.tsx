'use client'

import { useEffect } from 'react'
import { MapContainer, TileLayer, Marker, useMapEvents, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { leafletDefaultIcon } from '@/lib/leaflet-icon'

// Leaflet doesn't bundle default marker images for SPAs — apply the shared
// icon config globally so every marker on this map renders.
L.Marker.prototype.options.icon = leafletDefaultIcon

interface MapPickerInnerProps {
  lat?: number
  lng?: number
  onChange: (lat: number, lng: number) => void
  height?: string
}

function ClickHandler({ onPick }: { onPick: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e) {
      onPick(e.latlng.lat, e.latlng.lng)
    },
  })
  return null
}

function ReCenter({ lat, lng }: { lat?: number; lng?: number }) {
  const map = useMap()
  useEffect(() => {
    if (typeof lat === 'number' && typeof lng === 'number') {
      map.flyTo([lat, lng], 14, { duration: 0.5 })
    }
  }, [lat, lng, map])
  return null
}

export function MapPickerInner({ lat, lng, onChange, height = '300px' }: MapPickerInnerProps) {
  const hasCoords = typeof lat === 'number' && typeof lng === 'number'
  const center: [number, number] = hasCoords ? [lat!, lng!] : [20.5937, 78.9629] // India default
  const zoom = hasCoords ? 14 : 5

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
        {hasCoords ? <Marker position={[lat!, lng!]} /> : null}
        <ClickHandler onPick={onChange} />
        <ReCenter lat={lat} lng={lng} />
      </MapContainer>
    </div>
  )
}
