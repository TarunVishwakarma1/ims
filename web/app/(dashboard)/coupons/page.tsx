'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Ticket, Plus, Trash2, Loader2, Pencil, Power } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { couponsApi } from '@/lib/api/coupons'
import { formatPrice, cn } from '@/lib/utils'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { Coupon } from '@/types/api'

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
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'

interface FormState {
  code: string
  discount_type: 'percent' | 'fixed'
  discount_value_input: string
  min_subtotal_input: string
  max_uses_input: string
  description: string
  expires_at: string
  is_active: boolean
}

const blankForm: FormState = {
  code: '', discount_type: 'percent', discount_value_input: '',
  min_subtotal_input: '', max_uses_input: '', description: '',
  expires_at: '', is_active: true,
}

// Convert a wire coupon back into editable form values. Inverse of the
// submit() encoder.
function couponToForm(c: Coupon): FormState {
  return {
    code: c.code,
    discount_type: c.discount_type,
    discount_value_input:
      c.discount_type === 'fixed' ? (c.discount_value / 100).toString() : c.discount_value.toString(),
    min_subtotal_input: c.min_subtotal ? (c.min_subtotal / 100).toString() : '',
    max_uses_input: c.max_uses ? c.max_uses.toString() : '',
    description: c.description ?? '',
    // datetime-local wants "YYYY-MM-DDTHH:mm" in local time, no zone.
    expires_at: c.expires_at
      ? new Date(c.expires_at).toISOString().slice(0, 16)
      : '',
    is_active: c.is_active,
  }
}

export default function CouponsPage() {
  const qc = useQueryClient()
  const { user } = useAuthStore()
  const canManage = hasPermission(user?.role, PERMISSIONS.COUPONS_MANAGE)

  const { data: coupons = [], isLoading } = useQuery({
    queryKey: ['coupons'],
    queryFn: couponsApi.list,
  })

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Coupon | null>(null)
  const [form, setForm] = useState<FormState>(blankForm)

  const upsertMut = useMutation({
    mutationFn: async (payload: Partial<Coupon>) => {
      if (editing) return couponsApi.update(editing.id, payload)
      return couponsApi.create(payload)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['coupons'] })
      setOpen(false)
      setEditing(null)
      setForm(blankForm)
      toast.success(editing ? 'Coupon updated' : 'Coupon created')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || (editing ? 'Update failed' : 'Create failed'))
      } else toast.error(err.message)
    },
  })

  const deleteMut = useMutation({
    mutationFn: (id: string) => couponsApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['coupons'] })
      toast.success('Coupon deleted')
    },
  })

  // Toggle active sends a full coupon payload with is_active flipped — Update
  // is a patch-style endpoint but expects the existing fields preserved.
  const toggleActiveMut = useMutation({
    mutationFn: (c: Coupon) =>
      couponsApi.update(c.id, { ...c, is_active: !c.is_active }),
    onSuccess: (_, c) => {
      qc.invalidateQueries({ queryKey: ['coupons'] })
      toast.success(c.is_active ? 'Coupon deactivated' : 'Coupon activated')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Toggle failed')
      } else toast.error(err.message)
    },
  })

  const openCreate = () => {
    setEditing(null)
    setForm(blankForm)
    setOpen(true)
  }

  const openEdit = (c: Coupon) => {
    setEditing(c)
    setForm(couponToForm(c))
    setOpen(true)
  }

  const submit = () => {
    const dv = parseFloat(form.discount_value_input)
    if (!Number.isFinite(dv) || dv <= 0) {
      toast.error('Discount value must be positive')
      return
    }
    const discount_value = form.discount_type === 'fixed' ? Math.round(dv * 100) : Math.round(dv)
    const min_subtotal = form.min_subtotal_input
      ? Math.round(parseFloat(form.min_subtotal_input) * 100)
      : 0
    const max_uses = form.max_uses_input ? parseInt(form.max_uses_input, 10) : null
    upsertMut.mutate({
      code: form.code.trim().toUpperCase(),
      discount_type: form.discount_type,
      discount_value,
      min_subtotal,
      max_uses,
      description: form.description || null,
      expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : null,
      is_active: form.is_active,
    } as Partial<Coupon>)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Ticket className="h-6 w-6" /> Coupons
          </h2>
          <p className="text-muted-foreground">
            Discount codes buyers can apply to orders sold by this organization.
          </p>
        </div>
        {canManage && (
          <Button onClick={openCreate}>
            <Plus className="mr-2 h-4 w-4" /> New coupon
          </Button>
        )}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Codes</CardTitle>
          <CardDescription>{coupons.length} coupons</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="py-12 text-center text-muted-foreground text-sm">Loading…</div>
          ) : coupons.length === 0 ? (
            <div className="py-12 text-center text-muted-foreground text-sm">No coupons yet.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Discount</TableHead>
                  <TableHead>Min subtotal</TableHead>
                  <TableHead>Uses</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {coupons.map((c) => (
                  <TableRow key={c.id} className={cn(!c.is_active && 'opacity-60')}>
                    <TableCell className="font-mono">{c.code}</TableCell>
                    <TableCell>
                      {c.discount_type === 'percent'
                        ? `${c.discount_value}%`
                        : formatPrice(c.discount_value)}
                    </TableCell>
                    <TableCell>{c.min_subtotal ? formatPrice(c.min_subtotal) : '—'}</TableCell>
                    <TableCell>{c.usage_count}{c.max_uses ? ` / ${c.max_uses}` : ''}</TableCell>
                    <TableCell className="text-xs">
                      {c.expires_at ? new Date(c.expires_at).toLocaleDateString() : '—'}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className={cn(
                        'font-normal',
                        c.is_active
                          ? 'bg-emerald-100 text-emerald-800 border-emerald-200'
                          : 'bg-zinc-100 text-zinc-700 border-zinc-200',
                      )}>
                        {c.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {canManage && (
                        <div className="flex items-center gap-1 justify-end">
                          <Button
                            variant="ghost"
                            size="icon"
                            title="Edit"
                            onClick={() => openEdit(c)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            title={c.is_active ? 'Deactivate' : 'Activate'}
                            disabled={toggleActiveMut.isPending}
                            onClick={() => toggleActiveMut.mutate(c)}
                          >
                            <Power className={cn('h-4 w-4', c.is_active ? 'text-emerald-600' : 'text-zinc-400')} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            title="Delete"
                            onClick={() => {
                              if (confirm(`Delete coupon ${c.code}? This cannot be undone.`)) {
                                deleteMut.mutate(c.id)
                              }
                            }}
                          >
                            <Trash2 className="h-4 w-4 text-rose-600" />
                          </Button>
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? `Edit coupon ${editing.code}` : 'New coupon'}</DialogTitle>
            <DialogDescription>
              Codes apply only to orders sold by this org. Case-insensitive.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div>
              <Label htmlFor="code">Code</Label>
              <Input
                id="code"
                placeholder="SAVE10"
                value={form.code}
                disabled={!!editing}
                onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase() })}
              />
              {editing && (
                <p className="text-[10px] text-muted-foreground mt-1">
                  Code cannot change after creation (buyers may have it saved).
                </p>
              )}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Type</Label>
                <Select
                  value={form.discount_type}
                  onValueChange={(v) => setForm({ ...form, discount_type: v as never })}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="percent">Percent</SelectItem>
                    <SelectItem value="fixed">Fixed (₹)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label htmlFor="dv">
                  Value {form.discount_type === 'percent' ? '(%)' : '(₹)'}
                </Label>
                <Input
                  id="dv"
                  type="number"
                  step="0.01"
                  value={form.discount_value_input}
                  onChange={(e) => setForm({ ...form, discount_value_input: e.target.value })}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="ms">Min subtotal (₹)</Label>
                <Input
                  id="ms"
                  type="number"
                  step="0.01"
                  placeholder="0"
                  value={form.min_subtotal_input}
                  onChange={(e) => setForm({ ...form, min_subtotal_input: e.target.value })}
                />
              </div>
              <div>
                <Label htmlFor="mu">Max uses</Label>
                <Input
                  id="mu"
                  type="number"
                  min="1"
                  placeholder="Unlimited"
                  value={form.max_uses_input}
                  onChange={(e) => setForm({ ...form, max_uses_input: e.target.value })}
                />
              </div>
            </div>
            <div>
              <Label htmlFor="exp">Expires at</Label>
              <Input
                id="exp"
                type="datetime-local"
                value={form.expires_at}
                onChange={(e) => setForm({ ...form, expires_at: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="desc">Description</Label>
              <Input
                id="desc"
                placeholder="Holiday sale"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="h-4 w-4 rounded"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
              />
              <span>Active (buyers can use this code)</span>
            </label>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
            <Button onClick={submit} disabled={upsertMut.isPending || !form.code.trim()}>
              {upsertMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {editing ? 'Save changes' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
