'use client'

import { useEffect, useMemo, useRef } from 'react'
import { MapContainer, TileLayer, Marker, useMap } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { reverseGeocode, type GeocodeResult } from '@/lib/geocode'

// Leaflet's default marker icons break under bundlers; point them at the CDN.
const icon = L.icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41], iconAnchor: [12, 41],
})

// Default centre: Pune. Used when the shop has no saved location yet.
const DEFAULT: [number, number] = [18.5204, 73.8567]

interface Props {
  lat: number | null
  lng: number | null
  onChange: (lat: number, lng: number, geo?: GeocodeResult) => void
}

function Recenter({ lat, lng }: { lat: number; lng: number }) {
  const map = useMap()
  useEffect(() => { map.setView([lat, lng]) }, [lat, lng, map])
  return null
}

export function LocationPicker({ lat, lng, onChange }: Props) {
  const center = useMemo<[number, number]>(
    () => (lat != null && lng != null ? [lat, lng] : DEFAULT),
    [lat, lng],
  )
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounced reverse-geocode after the pin settles.
  function settle(newLat: number, newLng: number) {
    onChange(newLat, newLng)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(async () => {
      const geo = await reverseGeocode(newLat, newLng)
      onChange(newLat, newLng, geo)
    }, 1100)
  }

  function useMyLocation() {
    if (!navigator.geolocation) return
    navigator.geolocation.getCurrentPosition((pos) =>
      settle(pos.coords.latitude, pos.coords.longitude),
    )
  }

  return (
    <div className="space-y-2">
      <div className="h-72 w-full overflow-hidden rounded-md border">
        <MapContainer center={center} zoom={14} className="h-full w-full">
          <TileLayer
            attribution='&copy; OpenStreetMap contributors'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <Recenter lat={center[0]} lng={center[1]} />
          <Marker
            position={center}
            icon={icon}
            draggable
            eventHandlers={{
              dragend: (e) => {
                const p = e.target.getLatLng()
                settle(p.lat, p.lng)
              },
            }}
          />
        </MapContainer>
      </div>
      <button
        type="button"
        onClick={useMyLocation}
        className="text-sm text-primary hover:underline"
      >
        Use my current location
      </button>
    </div>
  )
}
