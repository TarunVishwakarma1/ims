/* eslint-disable react-hooks/incompatible-library */
'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, Edit, Trash2, Loader2, Star, Navigation } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { locationsApi } from '@/lib/api/locations'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'
import type { OrgLocation } from '@/types/api'
import { MapPicker } from '@/components/map/map-picker'
import { LocationsMap } from '@/components/map/locations-map'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

const locationSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  address: z.string().optional(),
  city: z.string().optional(),
  state: z.string().optional(),
  country: z.string().optional(),
  postal_code: z.string().optional(),
  lat: z.number().optional(),
  lng: z.number().optional(),
  is_primary: z.boolean().optional(),
})
type LocationFormValues = z.infer<typeof locationSchema>

export default function LocationsPage() {
  const queryClient = useQueryClient()
  const { can } = usePermission()
  
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedLocation, setSelectedLocation] = useState<OrgLocation | null>(null)
  const [isLocating, setIsLocating] = useState(false)

  const { data: rawLocations, isLoading } = useQuery({
    queryKey: ['locations'],
    queryFn: locationsApi.list,
  })
  const locations = rawLocations ?? []

  const createMutation = useMutation({
    mutationFn: locationsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['locations'] })
      setIsDialogOpen(false)
      reset()
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const errorData = await err.response.json().catch(() => ({}))
        toast.error(errorData.error || errorData.message || 'Failed to create location')
      } else {
        toast.error(err.message || 'Failed to create location')
      }
    }
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof locationsApi.update>[1] }) => locationsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['locations'] })
      setIsDialogOpen(false)
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const errorData = await err.response.json().catch(() => ({}))
        toast.error(errorData.error || errorData.message || 'Failed to update location')
      } else {
        toast.error(err.message || 'Failed to update location')
      }
    }
  })

  const deleteMutation = useMutation({
    mutationFn: locationsApi.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['locations'] })
      setIsDeleteDialogOpen(false)
    },
  })

  const { register, handleSubmit, reset, setValue, watch, formState: { errors, isSubmitting } } = useForm<LocationFormValues>({
    resolver: zodResolver(locationSchema),
    defaultValues: {
      name: '',
      address: '',
      city: '',
      state: '',
      country: '',
      postal_code: '',
      lat: undefined,
      lng: undefined,
      is_primary: false,
    }
  })

  const isPrimary = watch('is_primary')
  const currentLat = watch('lat')
  const currentLng = watch('lng')

  const handleOpenCreate = () => {
    setSelectedLocation(null)
    reset({
      name: '', address: '', city: '', state: '', country: '', postal_code: '', lat: undefined, lng: undefined, is_primary: false
    })
    setIsDialogOpen(true)
  }

  const handleOpenEdit = (location: OrgLocation) => {
    setSelectedLocation(location)
    reset({
      name: location.name,
      address: location.address || '',
      city: location.city || '',
      state: location.state || '',
      country: location.country || '',
      postal_code: location.postal_code || '',
      lat: location.lat ?? undefined,
      lng: location.lng ?? undefined,
      is_primary: location.is_primary,
    })
    setIsDialogOpen(true)
  }

  const onSubmit = (data: LocationFormValues) => {
    const payload = { ...data }
    if (payload.lat === 0) payload.lat = undefined
    if (payload.lng === 0) payload.lng = undefined

    if (selectedLocation) {
      updateMutation.mutate({ id: selectedLocation.id, data: payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Locations</h2>
          <p className="text-muted-foreground">Manage your organization&apos;s physical locations.</p>
        </div>
        {can(PERMISSIONS.LOCATIONS_MANAGE) && (
          <Button onClick={handleOpenCreate}>
            <Plus className="mr-2 h-4 w-4" /> Add Location
          </Button>
        )}
      </div>

      <LocationsMap locations={locations} height="380px" />

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>City</TableHead>
              <TableHead>Coordinates</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-[100px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  <Loader2 className="mx-auto h-6 w-6 animate-spin text-muted-foreground" />
                </TableCell>
              </TableRow>
            ) : locations.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                  No locations found.
                </TableCell>
              </TableRow>
            ) : (
              locations.map((loc) => (
                <TableRow key={loc.id}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      {loc.name}
                      {loc.is_primary && <Star className="h-3 w-3 fill-amber-400 text-amber-400" />}
                    </div>
                  </TableCell>
                  <TableCell>{loc.city || '-'}</TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {loc.lat && loc.lng ? `${loc.lat.toFixed(4)}, ${loc.lng.toFixed(4)}` : '-'}
                  </TableCell>
                  <TableCell>
                    {loc.is_active ? (
                      <Badge variant="outline" className="bg-emerald-50 text-emerald-600 border-emerald-200">Active</Badge>
                    ) : (
                      <Badge variant="secondary">Inactive</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    {can(PERMISSIONS.LOCATIONS_MANAGE) && (
                      <div className="flex justify-end gap-2">
                        <Button variant="ghost" size="icon" onClick={() => handleOpenEdit(loc)}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button 
                          variant="ghost" 
                          size="icon" 
                          className="text-red-500 hover:text-red-600"
                          onClick={() => {
                            setSelectedLocation(loc)
                            setIsDeleteDialogOpen(true)
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{selectedLocation ? 'Edit Location' : 'Add Location'}</DialogTitle>
            <DialogDescription className="sr-only">Fill out the location details.</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 py-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2 col-span-2">
                <Label htmlFor="name">Name *</Label>
                <Input id="name" {...register('name')} />
                {errors.name && <p className="text-sm text-red-500">{errors.name.message}</p>}
              </div>

              <div className="space-y-2 col-span-2">
                <Label htmlFor="address">Address</Label>
                <Input id="address" {...register('address')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="city">City</Label>
                <Input id="city" {...register('city')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="state">State</Label>
                <Input id="state" {...register('state')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="postal_code">Postal Code</Label>
                <Input id="postal_code" {...register('postal_code')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="country">Country</Label>
                <Input id="country" {...register('country')} />
              </div>

              <div className="col-span-2 space-y-2">
                <div className="flex items-center justify-between">
                  <Label>Coordinates</Label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      if (!('geolocation' in navigator)) {
                        toast.error('Geolocation not supported by your browser')
                        return
                      }
                      setIsLocating(true)
                      navigator.geolocation.getCurrentPosition(
                        (pos) => {
                          setValue('lat', Number(pos.coords.latitude.toFixed(6)), { shouldValidate: true })
                          setValue('lng', Number(pos.coords.longitude.toFixed(6)), { shouldValidate: true })
                          toast.success('Location captured')
                          setIsLocating(false)
                        },
                        (err) => {
                          const msg = err.code === err.PERMISSION_DENIED
                            ? 'Location permission denied'
                            : err.code === err.POSITION_UNAVAILABLE
                            ? 'Location unavailable'
                            : err.code === err.TIMEOUT
                            ? 'Location request timed out'
                            : 'Failed to get location'
                          toast.error(msg)
                          setIsLocating(false)
                        },
                        { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 }
                      )
                    }}
                    disabled={isLocating}
                  >
                    {isLocating ? (
                      <Loader2 className="w-3 h-3 mr-2 animate-spin" />
                    ) : (
                      <Navigation className="w-3 h-3 mr-2" />
                    )}
                    Use My Location
                  </Button>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <Label htmlFor="lat" className="text-xs text-muted-foreground">Latitude</Label>
                    <Input id="lat" type="number" step="any" placeholder="e.g. 19.076" {...register('lat', { valueAsNumber: true, setValueAs: v => v === "" || isNaN(v) ? undefined : v })} />
                  </div>
                  <div>
                    <Label htmlFor="lng" className="text-xs text-muted-foreground">Longitude</Label>
                    <Input id="lng" type="number" step="any" placeholder="e.g. 72.877" {...register('lng', { valueAsNumber: true, setValueAs: v => v === "" || isNaN(v) ? undefined : v })} />
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">Click on the map to set coordinates</p>
                <MapPicker
                  lat={typeof currentLat === 'number' && !isNaN(currentLat) ? currentLat : undefined}
                  lng={typeof currentLng === 'number' && !isNaN(currentLng) ? currentLng : undefined}
                  onChange={(lat, lng) => {
                    setValue('lat', Number(lat.toFixed(6)), { shouldValidate: true })
                    setValue('lng', Number(lng.toFixed(6)), { shouldValidate: true })
                  }}
                />
              </div>

              <div className="flex items-center space-x-2 col-span-2 pt-2">
                <Checkbox 
                  id="is_primary" 
                  checked={isPrimary} 
                  onCheckedChange={(checked) => setValue('is_primary', checked === true)} 
                />
                <Label htmlFor="is_primary">Set as Primary Location</Label>
              </div>
            </div>

            <DialogFooter className="pt-4">
              <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {selectedLocation ? 'Update' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Location</DialogTitle>
            <DialogDescription className="sr-only">Confirm deletion of the selected location.</DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <p>Are you sure you want to delete <strong>{selectedLocation?.name}</strong>? This action cannot be undone.</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>Cancel</Button>
            <Button 
              variant="destructive" 
              onClick={() => {
                if (selectedLocation) deleteMutation.mutate(selectedLocation.id)
              }}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
