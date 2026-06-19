'use client'

import dynamic from 'next/dynamic'

interface MapPickerProps {
  lat?: number
  lng?: number
  onChange: (lat: number, lng: number) => void
  height?: string
}

const MapPickerInner = dynamic(() => import('./map-picker-inner').then(m => m.MapPickerInner), {
  ssr: false,
  loading: () => (
    <div className="rounded-md border bg-muted/50 flex items-center justify-center text-sm text-muted-foreground h-[300px]">
      Loading map...
    </div>
  ),
})

export function MapPicker(props: MapPickerProps) {
  return <MapPickerInner {...props} />
}
