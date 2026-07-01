'use client'

import { useState, type ChangeEvent } from 'react'
import dynamic from 'next/dynamic'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Store, Loader2, X } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { storefrontApi } from '@/lib/api/storefront'
import { canGoLive } from '@/lib/storefront-validation'
import { imageSrc } from '@/lib/image-src'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { ShopProfile, ShopProfileInput } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

// Leaflet touches window — never SSR it.
const LocationPicker = dynamic(
  () => import('@/components/storefront/location-picker').then((m) => m.LocationPicker),
  { ssr: false, loading: () => <div className="h-72 rounded-md border bg-muted" /> },
)

const EMPTY: ShopProfileInput = {
  slug: '', display_name: '', tagline: '', logo_url: '', area: '', city: '',
  pincodes: [], lat: null, lng: null, is_live: false,
}

function profileToInput(p: ShopProfile): ShopProfileInput {
  return {
    slug: p.slug, display_name: p.display_name, tagline: p.tagline,
    logo_url: p.logo_url, area: p.area, city: p.city,
    pincodes: p.pincodes, lat: p.lat, lng: p.lng, is_live: p.is_live,
  }
}

// ─── Inner form — only mounted once data is resolved ────────────────────────
// Receives the initial profile data as a prop so useState can be seeded
// directly (no useEffect setState cascade needed).
// The `key` on this component (= updated_at or "new") causes a clean remount
// when the server returns a fresh record after save.
function StorefrontForm({ initial, isNew }: { initial: ShopProfileInput; isNew: boolean }) {
  const qc = useQueryClient()
  const [form, setForm] = useState<ShopProfileInput>(initial)
  const [pincodeDraft, setPincodeDraft] = useState('')
  const [uploading, setUploading] = useState(false)

  async function onLogoFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = '' // let the same file be re-selected after an error
    if (!file) return
    setUploading(true)
    try {
      const { logo_url } = await storefrontApi.uploadLogo(file)
      setForm((f) => ({ ...f, logo_url }))
      toast.success('Logo uploaded')
    } catch {
      toast.error('Could not upload logo')
    } finally {
      setUploading(false)
    }
  }

  const save = useMutation({
    mutationFn: (input: ShopProfileInput) => storefrontApi.upsert(input),
    onSuccess: (p) => {
      qc.setQueryData(['storefront'], p)
      toast.success(p.is_live ? 'Storefront is live' : 'Storefront saved')
    },
    onError: async (e) => {
      if (e instanceof HTTPError) {
        const body = await e.response.json().catch(() => ({}))
        toast.error((body as { error?: string }).error ?? 'Could not save storefront')
      } else {
        toast.error('Could not save storefront')
      }
    },
  })

  const slugLocked = !isNew && initial.is_live
  const liveEligible = canGoLive(form)

  function set<K extends keyof ShopProfileInput>(k: K, v: ShopProfileInput[K]) {
    setForm((f) => ({ ...f, [k]: v }))
  }
  function addPincode() {
    const v = pincodeDraft.trim()
    if (v && !form.pincodes.includes(v)) set('pincodes', [...form.pincodes, v])
    setPincodeDraft('')
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <div className="flex items-center gap-2">
        <Store className="h-5 w-5 text-primary" />
        <h1 className="text-2xl font-semibold">My Storefront</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Shop details</CardTitle>
          <CardDescription>How your shop appears inside Kirana.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label>Display name</Label>
            <Input value={form.display_name}
              onChange={(e) => set('display_name', e.target.value)} />
          </div>
          <div>
            <Label>Storefront address (slug)</Label>
            <Input value={form.slug} disabled={slugLocked}
              onChange={(e) => set('slug', e.target.value)} />
            <p className="mt-1 text-xs text-muted-foreground">
              {slugLocked
                ? 'Locked — your shop is live at /s/' + form.slug
                : 'Your public URL will be /s/' + (form.slug || 'your-shop')}
            </p>
          </div>
          <div>
            <Label>Tagline</Label>
            <Input value={form.tagline} onChange={(e) => set('tagline', e.target.value)} />
          </div>
          <div>
            <Label>Logo</Label>
            <div className="flex items-center gap-3">
              {form.logo_url && (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={imageSrc(form.logo_url)} alt=""
                  className="h-12 w-12 shrink-0 rounded border object-cover" />
              )}
              <Input value={form.logo_url} placeholder="Paste an image URL, or upload →"
                onChange={(e) => set('logo_url', e.target.value)} />
              <label className="shrink-0">
                <input type="file" accept="image/png,image/jpeg,image/webp"
                  className="hidden" onChange={onLogoFile} disabled={uploading} />
                <span className="inline-flex h-9 cursor-pointer items-center rounded-md border px-3 text-sm hover:bg-muted">
                  {uploading ? 'Uploading…' : 'Upload'}
                </span>
              </label>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Location</CardTitle>
          <CardDescription>Drop the pin on your shop; we&apos;ll fill the address.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <LocationPicker
            lat={form.lat} lng={form.lng}
            onChange={(lat, lng, geo) =>
              setForm((f) => ({
                ...f, lat, lng,
                area: geo?.area || f.area,
                city: geo?.city || f.city,
                pincodes: geo?.pincode && !f.pincodes.includes(geo.pincode)
                  ? [...f.pincodes, geo.pincode] : f.pincodes,
              }))
            }
          />
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label>Area</Label>
              <Input value={form.area} onChange={(e) => set('area', e.target.value)} />
            </div>
            <div>
              <Label>City</Label>
              <Input value={form.city} onChange={(e) => set('city', e.target.value)} />
            </div>
          </div>
          <div>
            <Label>Serviceable pincodes</Label>
            <div className="flex gap-2">
              <Input value={pincodeDraft} onChange={(e) => setPincodeDraft(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addPincode() } }}
                placeholder="411001" />
              <Button type="button" variant="outline" onClick={addPincode}>Add</Button>
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              {form.pincodes.map((pc) => (
                <span key={pc} className="inline-flex items-center gap-1 rounded-full bg-muted px-3 py-1 text-sm">
                  {pc}
                  <button type="button" onClick={() => set('pincodes', form.pincodes.filter((x) => x !== pc))}>
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={form.is_live} disabled={!liveEligible && !form.is_live}
            onChange={(e) => set('is_live', e.target.checked)} />
          Live in Kirana directory
          {!liveEligible && !form.is_live && (
            <span className="text-xs text-muted-foreground">
              (need name, map location, and ≥1 pincode)
            </span>
          )}
        </label>
        <Button onClick={() => save.mutate(form)} disabled={save.isPending}>
          {save.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save'}
        </Button>
      </div>
    </div>
  )
}

// ─── Page shell — handles loading and passes resolved data to form ───────────
export default function StorefrontPage() {
  const { user } = useAuthStore()
  const canView = hasPermission(user?.role, PERMISSIONS.STOREFRONT_VIEW)

  const { data, isLoading } = useQuery({
    queryKey: ['storefront'],
    queryFn: storefrontApi.get,
    enabled: canView,
  })

  if (!canView) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <p className="text-sm text-muted-foreground">
          You don&apos;t have permission to manage the storefront.
        </p>
      </div>
    )
  }

  if (isLoading) {
    return <div className="p-8"><Loader2 className="animate-spin" /></div>
  }

  const initial = data ? profileToInput(data) : EMPTY
  // key = updated_at (or "new") so the form remounts with fresh server data
  // after a successful save (useQueryClient.setQueryData triggers this).
  const formKey = data?.updated_at ?? 'new'

  return <StorefrontForm key={formKey} initial={initial} isNew={!data} />
}
