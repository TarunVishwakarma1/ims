'use client'

import { use, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { HTTPError } from 'ky'
import { ArrowLeft, Loader2, Check, X, PackageCheck } from 'lucide-react'

import { returnsApi, type ReturnStatus } from '@/lib/api/returns'
import { formatPrice } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { PERMISSIONS } from '@/lib/rbac'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

function statusBadge(s: ReturnStatus) {
  const map: Record<ReturnStatus, string> = {
    requested: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    approved: 'bg-blue-100 text-blue-800 border-blue-200',
    rejected: 'bg-red-100 text-red-800 border-red-200',
    in_transit: 'bg-purple-100 text-purple-800 border-purple-200',
    received: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    refunded: 'bg-emerald-600 text-white border-emerald-600',
  }
  return <Badge variant="outline" className={map[s] || ''}>{s.replace('_', ' ')}</Badge>
}

export default function ReturnDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const qc = useQueryClient()
  const { can } = usePermission()
  const [rejectOpen, setRejectOpen] = useState(false)
  const [rejectNote, setRejectNote] = useState('')

  const { data: rr, isLoading } = useQuery({
    queryKey: ['return', id],
    queryFn: () => returnsApi.getById(id),
  })

  const handleErr = async (err: Error, fallback: string) => {
    if (err instanceof HTTPError) {
      const data = await err.response.json().catch(() => ({} as { error?: string }))
      toast.error(data.error || fallback)
    } else {
      toast.error(err.message || fallback)
    }
  }

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['return', id] })
    qc.invalidateQueries({ queryKey: ['returns'] })
    qc.invalidateQueries({ queryKey: ['orders'] })
  }

  const approveMut = useMutation({
    mutationFn: () => returnsApi.approve(id),
    onSuccess: () => { refresh(); toast.success('Return approved') },
    onError: (e: Error) => handleErr(e, 'Approve failed'),
  })
  const rejectMut = useMutation({
    mutationFn: () => returnsApi.reject(id, rejectNote),
    onSuccess: () => { refresh(); setRejectOpen(false); setRejectNote(''); toast.success('Return rejected') },
    onError: (e: Error) => handleErr(e, 'Reject failed'),
  })
  const receivedMut = useMutation({
    mutationFn: () => returnsApi.markReceived(id),
    onSuccess: () => { refresh(); toast.success('Return received — refund triggered') },
    onError: (e: Error) => handleErr(e, 'Mark received failed'),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (!rr) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => router.push('/returns')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> Back
        </Button>
        <p className="text-muted-foreground">Return not found.</p>
      </div>
    )
  }

  const items = rr.items ?? []
  const canManage = can(PERMISSIONS.ORDERS_MANAGE)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push('/returns')}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h2 className="text-2xl font-bold tracking-tight">
              Return <span className="font-mono text-base text-muted-foreground">#{rr.id.split('-')[0]}</span>
            </h2>
            <div className="text-sm text-muted-foreground">
              For order <button
                className="underline font-mono"
                onClick={() => router.push(`/orders/${rr.order_id}`)}
              >#{rr.order_id.split('-')[0]}</button>
              {' · '}
              {new Date(rr.created_at).toLocaleString()}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {statusBadge(rr.status)}
          {canManage && rr.status === 'requested' && (
            <>
              <Button variant="outline" onClick={() => approveMut.mutate()} disabled={approveMut.isPending}>
                {approveMut.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Check className="mr-2 h-4 w-4" />}
                Approve
              </Button>
              <Button variant="outline" className="text-red-600" onClick={() => setRejectOpen(true)}>
                <X className="mr-2 h-4 w-4" /> Reject
              </Button>
            </>
          )}
          {canManage && (rr.status === 'approved' || rr.status === 'in_transit') && (
            <Button variant="outline" onClick={() => receivedMut.mutate()} disabled={receivedMut.isPending}>
              {receivedMut.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <PackageCheck className="mr-2 h-4 w-4" />}
              Mark received
            </Button>
          )}
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Items being returned</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Product</TableHead>
                  <TableHead className="text-right">Qty</TableHead>
                  <TableHead className="text-right">Unit price</TableHead>
                  <TableHead className="text-right">Refund</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((it) => (
                  <TableRow key={it.id}>
                    <TableCell className="font-mono text-xs">{it.product_id.split('-')[0]}</TableCell>
                    <TableCell className="text-right">{it.quantity}</TableCell>
                    <TableCell className="text-right">{formatPrice(it.unit_price)}</TableCell>
                    <TableCell className="text-right font-medium">{formatPrice(it.quantity * it.unit_price)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex justify-between border-t mt-3 pt-3 font-semibold">
              <span>Total refund</span>
              <span>{formatPrice(rr.refund_amount)}</span>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Reason</CardTitle>
            </CardHeader>
            <CardContent className="text-sm whitespace-pre-wrap">{rr.reason}</CardContent>
          </Card>

          {rr.rejection_note && (
            <Card>
              <CardHeader>
                <CardTitle>Rejection note</CardTitle>
              </CardHeader>
              <CardContent className="text-sm whitespace-pre-wrap text-red-700">
                {rr.rejection_note}
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>Timeline</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Requested</span>
                <span>{new Date(rr.created_at).toLocaleString()}</span>
              </div>
              {rr.approved_at && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Approved</span>
                  <span>{new Date(rr.approved_at).toLocaleString()}</span>
                </div>
              )}
              {rr.received_at && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Received</span>
                  <span>{new Date(rr.received_at).toLocaleString()}</span>
                </div>
              )}
              {rr.refunded_at && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Refunded</span>
                  <span>{new Date(rr.refunded_at).toLocaleString()}</span>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Reject dialog */}
      <Dialog open={rejectOpen} onOpenChange={setRejectOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reject return?</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-2">
            <Label htmlFor="reject-note" className="text-xs">Note (required, shown to customer)</Label>
            <Input
              id="reject-note"
              value={rejectNote}
              onChange={(e) => setRejectNote(e.target.value)}
              placeholder="e.g. outside return window"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectOpen(false)}>Cancel</Button>
            <Button
              variant="destructive"
              disabled={!rejectNote || rejectMut.isPending}
              onClick={() => rejectMut.mutate()}
            >
              {rejectMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Reject return
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
