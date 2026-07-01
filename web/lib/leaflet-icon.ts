import L from 'leaflet'

// Leaflet doesn't bundle its default marker images for SPA bundlers, so the
// pin renders blank unless the URLs are set explicitly. Shared by every
// Leaflet map in the admin (location-picker, map-picker) so the config can't
// drift between them.
export const leafletDefaultIcon = L.icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
})
