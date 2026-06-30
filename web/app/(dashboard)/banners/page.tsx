'use client'

import { useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Image as ImageIcon, Plus, Loader2, Pencil, Upload, Megaphone, Archive } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { bannersApi } from '@/lib/api/banners'
import { cn } from '@/lib/utils'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { Banner, BannerInput } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'

const API_ORIGIN = (process.env.NEXT_PUBLIC_API_URL || '').replace(/\/api\/?$/, '')

// Backend serves uploads at /uploads/... — prefix relative paths with the API
// origin so the admin <img> can load them.
function imageSrc(url?: string): string {
  if (!url) return ''
  if (/^https?:\/\//.test(url)) return url
  return `${API_ORIGIN}${url.startsWith('/') ? '' : '/'}${url}`
}

interface FormState {
  title: string
  subtitle: string
  image_url: string
  cta_label: string
  cta_link: string
  category_slug: string
  event_key: string
  is_hero: boolean
  starts_at: string // datetime-local
  ends_at: string
}

function defaultWindow(): { start: string; end: string } {
  const now = new Date()
  const end = new Date(now.getTime() + 14 * 24 * 60 * 60 * 1000)
  const fmt = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  return { start: fmt(now), end: fmt(end) }
}

function blankForm(): FormState {
  const w = defaultWindow()
  return {
    title: '', subtitle: '', image_url: '', cta_label: '', cta_link: '',
    category_slug: '', event_key: '', is_hero: true, starts_at: w.start, ends_at: w.end,
  }
}

function bannerToForm(b: Banner): FormState {
  const toLocal = (iso: string) => {
    const d = new Date(iso)
    return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  }
  return {
    title: b.title,
    subtitle: b.subtitle ?? '',
    image_url: b.image_url ?? '',
    cta_label: b.cta_label ?? '',
    cta_link: b.cta_link ?? '',
    category_slug: b.category_slug ?? '',
    event_key: b.event_key ?? '',
    is_hero: b.is_hero,
    starts_at: toLocal(b.starts_at),
    ends_at: toLocal(b.ends_at),
  }
}

function statusBadge(status: Banner['status']) {
  const map: Record<Banner['status'], string> = {
    published: 'bg-emerald-100 text-emerald-800 border-emerald-200',
    draft: 'bg-zinc-100 text-zinc-700 border-zinc-200',
    archived: 'bg-amber-100 text-amber-800 border-amber-200',
  }
  return map[status]
}

export default function BannersPage() {
  const qc = useQueryClient()
  const { user } = useAuthStore()
  const canManage = hasPermission(user?.role, PERMISSIONS.BANNERS_MANAGE)

  const { data: banners = [], isLoading } = useQuery({
    queryKey: ['banners'],
    queryFn: () => bannersApi.list(),
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Banner | null>(null)
  const [form, setForm] = useState<FormState>(blankForm)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const invalidate = () => qc.invalidateQueries({ queryKey: ['banners'] })

  const onHttpError = async (err: Error, fallback: string) => {
    if (err instanceof HTTPError) {
      const data = await err.response.json().catch(() => ({} as { error?: string }))
      toast.error(data.error || fallback)
    } else toast.error(err.message)
  }

  const upsertMut = useMutation({
    mutationFn: (payload: BannerInput) =>
      editing ? bannersApi.update(editing.id, payload) : bannersApi.create(payload),
    onSuccess: () => {
      invalidate()
      setOpen(false)
      setEditing(null)
      setForm(blankForm())
      toast.success(editing ? 'Banner updated' : 'Banner created (draft)')
    },
    onError: (e: Error) => onHttpError(e, editing ? 'Update failed' : 'Create failed'),
  })

  const publishMut = useMutation({
    mutationFn: (id: string) => bannersApi.publish(id),
    onSuccess: () => { invalidate(); toast.success('Banner published') },
    onError: (e: Error) => onHttpError(e, 'Publish failed'),
  })

  const archiveMut = useMutation({
    mutationFn: (id: string) => bannersApi.archive(id),
    onSuccess: () => { invalidate(); toast.success('Banner archived') },
    onError: (e: Error) => onHttpError(e, 'Archive failed'),
  })

  const openCreate = () => { setEditing(null); setForm(blankForm()); setOpen(true) }
  const openEdit = (b: Banner) => { setEditing(b); setForm(bannerToForm(b)); setOpen(true) }

  const onPickFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      const { image_url } = await bannersApi.upload(file)
      setForm((f) => ({ ...f, image_url }))
      toast.success('Image uploaded')
    } catch (err) {
      await onHttpError(err as Error, 'Upload failed')
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const submit = () => {
    if (!form.title.trim()) { toast.error('Title is required'); return }
    if (!form.image_url) { toast.error('Upload an image'); return }
    if (!form.starts_at || !form.ends_at) { toast.error('Set a start and end'); return }
    if (new Date(form.ends_at) <= new Date(form.starts_at)) {
      toast.error('End must be after start'); return
    }
    upsertMut.mutate({
      title: form.title.trim(),
      subtitle: form.subtitle || undefined,
      image_url: form.image_url,
      cta_label: form.cta_label || undefined,
      cta_link: form.cta_link || undefined,
      category_slug: form.category_slug || undefined,
      event_key: form.event_key || undefined,
      is_hero: form.is_hero,
      starts_at: new Date(form.starts_at).toISOString(),
      ends_at: new Date(form.ends_at).toISOString(),
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <ImageIcon className="h-6 w-6" /> Banners
          </h2>
          <p className="text-muted-foreground">
            Festive and promotional banners for the shop home. Published banners show
            on the storefront only within their scheduled window.
          </p>
        </div>
        {canManage && (
          <Button onClick={openCreate}>
            <Plus className="mr-2 h-4 w-4" /> New banner
          </Button>
        )}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">All banners</CardTitle>
          <CardDescription>{banners.length} banners</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground text-sm">Loading…</div>
          ) : banners.length === 0 ? (
            <div className="py-12 text-center text-muted-foreground text-sm">No banners yet.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Banner</TableHead>
                  <TableHead>Slot</TableHead>
                  <TableHead>Window</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {banners.map((b) => (
                  <TableRow key={b.id} className={cn(b.status === 'archived' && 'opacity-60')}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={imageSrc(b.image_url)}
                          alt=""
                          className="h-10 w-20 rounded object-cover bg-muted shrink-0"
                        />
                        <div className="min-w-0">
                          <div className="font-medium truncate">{b.title}</div>
                          {b.subtitle && (
                            <div className="text-xs text-muted-foreground truncate">{b.subtitle}</div>
                          )}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-normal">
                        {b.is_hero ? 'Hero' : 'Carousel'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs whitespace-nowrap">
                      {new Date(b.starts_at).toLocaleDateString()} → {new Date(b.ends_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className={cn('font-normal capitalize', statusBadge(b.status))}>
                        {b.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {canManage && (
                        <div className="flex items-center gap-1 justify-end">
                          <Button variant="ghost" size="icon" title="Edit" onClick={() => openEdit(b)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          {b.status !== 'published' ? (
                            <Button
                              variant="ghost" size="icon" title="Publish"
                              disabled={publishMut.isPending}
                              onClick={() => publishMut.mutate(b.id)}
                            >
                              <Megaphone className="h-4 w-4 text-emerald-600" />
                            </Button>
                          ) : (
                            <Button
                              variant="ghost" size="icon" title="Archive"
                              disabled={archiveMut.isPending}
                              onClick={() => {
                                if (confirm(`Archive "${b.title}"? It will stop showing on the shop.`)) {
                                  archiveMut.mutate(b.id)
                                }
                              }}
                            >
                              <Archive className="h-4 w-4 text-amber-600" />
                            </Button>
                          )}
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={open} onOpenChange={(next) => { setOpen(next); if (!next) setEditing(null) }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editing ? 'Edit banner' : 'New banner'}</DialogTitle>
            <DialogDescription>
              New banners start as a draft. Publish when you’re ready — it shows on the
              shop only between the start and end dates.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            <div>
              <Label>Image</Label>
              <div className="mt-1 flex items-center gap-3">
                <div className="h-16 w-32 rounded border bg-muted overflow-hidden shrink-0 grid place-items-center">
                  {form.image_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={imageSrc(form.image_url)} alt="" className="h-full w-full object-cover" />
                  ) : (
                    <ImageIcon className="h-5 w-5 text-muted-foreground" />
                  )}
                </div>
                <div>
                  <input ref={fileRef} type="file" accept="image/png,image/jpeg,image/webp" className="hidden" onChange={onPickFile} />
                  <Button type="button" variant="outline" size="sm" disabled={uploading} onClick={() => fileRef.current?.click()}>
                    {uploading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Upload className="mr-2 h-4 w-4" />}
                    {form.image_url ? 'Replace image' : 'Upload image'}
                  </Button>
                  <p className="text-[10px] text-muted-foreground mt-1">PNG, JPG or WebP. Wide 3:1 works best.</p>
                </div>
              </div>
            </div>

            <div>
              <Label htmlFor="title">Title</Label>
              <Input id="title" placeholder="Diwali Dhamaka" value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })} />
            </div>
            <div>
              <Label htmlFor="subtitle">Subtitle</Label>
              <Input id="subtitle" placeholder="Up to 30% off sweets & gifting" value={form.subtitle}
                onChange={(e) => setForm({ ...form, subtitle: e.target.value })} />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="ctal">CTA label</Label>
                <Input id="ctal" placeholder="Shop the festival" value={form.cta_label}
                  onChange={(e) => setForm({ ...form, cta_label: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="ctalink">CTA link</Label>
                <Input id="ctalink" placeholder="/c/snacks" value={form.cta_link}
                  onChange={(e) => setForm({ ...form, cta_link: e.target.value })} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="start">Show from</Label>
                <Input id="start" type="datetime-local" value={form.starts_at}
                  onChange={(e) => setForm({ ...form, starts_at: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="end">Show until</Label>
                <Input id="end" type="datetime-local" value={form.ends_at}
                  onChange={(e) => setForm({ ...form, ends_at: e.target.value })} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="ek">Event key</Label>
                <Input id="ek" placeholder="diwali (optional)" value={form.event_key}
                  onChange={(e) => setForm({ ...form, event_key: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="cs">Category slug</Label>
                <Input id="cs" placeholder="snacks (optional)" value={form.category_slug}
                  onChange={(e) => setForm({ ...form, category_slug: e.target.value })} />
              </div>
            </div>

            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" className="h-4 w-4 rounded" checked={form.is_hero}
                onChange={(e) => setForm({ ...form, is_hero: e.target.checked })} />
              <span>Hero banner (large slot at top — otherwise shows in the carousel)</span>
            </label>
          </div>

          <DialogFooter>
            <Button variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
            <Button onClick={submit} disabled={upsertMut.isPending || uploading}>
              {upsertMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {editing ? 'Save changes' : 'Create draft'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
