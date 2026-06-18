'use client'

import { use, useEffect, useState } from 'react'
import Link from 'next/link'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, Receipt, RotateCcw, CircleCheck, CircleX, Clock,
  Copy, Loader2, ExternalLink, AlertTriangle,
} from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { paymentsApi } from '@/lib/api/payments'
import { formatPrice, cn } from '@/lib/utils'
import { useAuthStore } from '@/lib/stores/auth-store'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import type { Payment, PaymentStatus } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'

const STATUS_META: Record<PaymentStatus, { label: string; cls: string; icon: typeof CircleCheck }> = {
  created:            { label: 'Pending',             cls: 'bg-amber-100 text-amber-800 border-amber-200',     icon: Clock },
  authorized:         { label: 'Authorized',          cls: 'bg-sky-100 text-sky-800 border-sky-200',           icon: Clock },
  captured:           { label: 'Captured',            cls: 'bg-emerald-100 text-emerald-800 border-emerald-200', icon: CircleCheck },
  failed:             { label: 'Failed',              cls: 'bg-rose-100 text-rose-800 border-rose-200',         icon: CircleX },
  partially_refunded: { label: 'Partially refunded',  cls: 'bg-amber-100 text-amber-800 border-amber-200',     icon: RotateCcw },
  refunded:           { label: 'Refunded',            cls: 'bg-violet-100 text-violet-800 border-violet-200',   icon: RotateCcw },
}

export default function PaymentDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { user } = useAuthStore()
  const queryClient = useQueryClient()
  const canRefund = hasPermission(user?.role, PERMISSIONS.PAYMENTS_REFUND)

  // `expectingChange` is set after a refund mutation succeeds. It enables
  // aggressive polling (2s) for ~30s so the user sees the status flip as
  // soon as the webhook lands. After that we fall back to normal stale-time.
  const [expectingChange, setExpectingChange] = useState(false)

  const { data: payment, isLoading } = useQuery({
    queryKey: ['payment', id],
    queryFn: () => paymentsApi.getById(id),
    refetchInterval: expectingChange ? 2000 : false,
  })

  // Stop polling once amount_refunded > 0 or status hit terminal — webhook landed.
  useEffect(() => {
    if (!expectingChange || !payment) return
    if (payment.status === 'refunded' || payment.status === 'partially_refunded') {
      setExpectingChange(false)
      queryClient.invalidateQueries({ queryKey: ['refunds', id] })
      queryClient.invalidateQueries({ queryKey: ['payments'] })
    }
  }, [payment, expectingChange, id, queryClient])

  // Safety stop — if webhook never lands, give up after 30s.
  useEffect(() => {
    if (!expectingChange) return
    const t = setTimeout(() => setExpectingChange(false), 30_000)
    return () => clearTimeout(t)
  }, [expectingChange])

  const { data: refunds = [] } = useQuery({
    queryKey: ['refunds', id],
    queryFn: () => paymentsApi.listRefunds(id),
    enabled: !!payment,
  })

  const [refundOpen, setRefundOpen] = useState(false)
  const [refundAmount, setRefundAmount] = useState('')
  const [refundReason, setRefundReason] = useState('')
  const [refundFormError, setRefundFormError] = useState<string | null>(null)

  // Remaining refundable in rupees — derived live so the dialog reflects any
  // partial refunds that already landed.
  const remainingPaise = payment ? Math.max(0, payment.amount - payment.amount_refunded) : 0
  const remainingRupees = remainingPaise / 100

  const refundMut = useMutation({
    mutationFn: () => {
      const raw = refundAmount.trim()
      const paise = raw ? Math.round(parseFloat(raw) * 100) : 0
      return paymentsApi.refund(id, paise, refundReason)
    },
    onSuccess: () => {
      toast.success('Refund initiated — waiting for confirmation…')
      setRefundOpen(false)
      setRefundAmount('')
      setRefundReason('')
      setRefundFormError(null)
      setExpectingChange(true)
      // Optimistic-style refetch — webhook may already be in for mock mode.
      queryClient.invalidateQueries({ queryKey: ['payment', id] })
      queryClient.invalidateQueries({ queryKey: ['refunds', id] })
      queryClient.invalidateQueries({ queryKey: ['payments'] })
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        const msg = data.error || 'Refund failed'
        setRefundFormError(msg)
        toast.error(msg)
      } else {
        setRefundFormError(err.message)
        toast.error(err.message)
      }
    },
  })

  // Client-side validation for the refund dialog. Runs on every change so
  // the submit button is gated and the error inline.
  const validateRefundAmount = (raw: string): string | null => {
    if (!raw.trim()) return null // empty = full refund of remaining
    const n = parseFloat(raw)
    if (!Number.isFinite(n) || n <= 0) return 'Enter a positive amount.'
    const paise = Math.round(n * 100)
    if (paise > remainingPaise) {
      return `Refund cannot exceed remaining refundable (${formatPrice(remainingPaise)}).`
    }
    return null
  }

  const onRefundAmountChange = (v: string) => {
    setRefundAmount(v)
    setRefundFormError(validateRefundAmount(v))
  }

  if (isLoading || !payment) {
    return (
      <div className="space-y-6 max-w-3xl">
        <BackLink />
        <Card><CardContent className="py-16 text-center text-muted-foreground">Loading payment…</CardContent></Card>
      </div>
    )
  }

  const meta = STATUS_META[payment.status] ?? STATUS_META.created
  const Icon = meta.icon
  const isRefundable = (payment.status === 'captured' || payment.status === 'partially_refunded') && remainingPaise > 0

  return (
    <div className="space-y-6 max-w-3xl">
      <BackLink />

      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Payment</h2>
          <p className="text-muted-foreground font-mono text-sm">{payment.id}</p>
        </div>
        <Badge variant="outline" className={cn('gap-1 font-normal', meta.cls)}>
          <Icon className="h-3 w-3" /> {meta.label}
        </Badge>
      </div>

      {expectingChange && (
        <Alert className="border-indigo-200 bg-indigo-50 dark:bg-indigo-950/30">
          <Loader2 className="h-4 w-4 animate-spin" />
          <AlertDescription>
            Refund submitted — waiting for Razorpay confirmation. The status will update automatically.
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Summary</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <Row label="Amount" value={formatPrice(payment.amount)} bold />
          {payment.amount_refunded > 0 && (
            <>
              <Row label="Refunded" value={formatPrice(payment.amount_refunded)} className="text-violet-600 dark:text-violet-400" />
              <Row label="Remaining refundable" value={formatPrice(remainingPaise)} />
            </>
          )}
          <Row label="Currency" value={payment.currency} />
          <Row label="Status" value={meta.label} />
          <Row label="Method" value={payment.method ?? '—'} />
          {payment.failure_reason && (
            <Row label="Failure reason" value={payment.failure_reason} className="text-rose-600" />
          )}
          <Row label="Mode" value={payment.is_mock ? 'Mock (test)' : 'Live'} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">References</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {payment.order_id && (
            <Row
              label="Order"
              value={
                <Link href={`/orders/${payment.order_id}`} className="text-indigo-600 dark:text-indigo-400 hover:underline inline-flex items-center gap-1">
                  {payment.order_id.slice(0, 8)}… <ExternalLink className="h-3 w-3" />
                </Link>
              }
            />
          )}
          <CopyRow label="Razorpay order" value={payment.razorpay_order_id} />
          {payment.razorpay_payment_id && (
            <CopyRow label="Razorpay payment" value={payment.razorpay_payment_id} />
          )}
        </CardContent>
      </Card>

      {refunds.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <RotateCcw className="h-4 w-4" /> Refund history
            </CardTitle>
            <CardDescription>{refunds.length} refund{refunds.length === 1 ? '' : 's'} on this payment</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            <table className="w-full text-sm">
              <thead className="bg-zinc-50 dark:bg-zinc-900/50 text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="text-left px-4 py-2">Refund ID</th>
                  <th className="text-left px-4 py-2">Reason</th>
                  <th className="text-right px-4 py-2">Amount</th>
                  <th className="text-left px-4 py-2">Status</th>
                  <th className="text-left px-4 py-2">When</th>
                </tr>
              </thead>
              <tbody>
                {refunds.map((rf) => (
                  <tr key={rf.id} className="border-t">
                    <td className="px-4 py-2 font-mono text-xs">{rf.razorpay_refund_id || rf.id.slice(0, 8) + '…'}</td>
                    <td className="px-4 py-2 text-muted-foreground">{rf.reason || '—'}</td>
                    <td className="px-4 py-2 text-right font-medium">{formatPrice(rf.amount)}</td>
                    <td className="px-4 py-2">
                      <Badge variant="outline" className={cn(
                        'font-normal',
                        rf.status === 'processed' && 'bg-emerald-100 text-emerald-800 border-emerald-200',
                        rf.status === 'failed' && 'bg-rose-100 text-rose-800 border-rose-200',
                        rf.status === 'pending' && 'bg-amber-100 text-amber-800 border-amber-200',
                      )}>
                        {rf.status}
                      </Badge>
                    </td>
                    <td className="px-4 py-2 text-xs text-muted-foreground">{new Date(rf.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Timeline</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <Row label="Created" value={new Date(payment.created_at).toLocaleString()} />
          <Row label="Last updated" value={new Date(payment.updated_at).toLocaleString()} />
        </CardContent>
      </Card>

      <div className="flex flex-wrap gap-2">
        {(payment.status === 'captured' || payment.status === 'partially_refunded' || payment.status === 'refunded') && (
          <Button variant="outline" asChild>
            <Link href={`/payments/${payment.id}/receipt`}>
              <Receipt className="mr-2 h-4 w-4" /> View receipt
            </Link>
          </Button>
        )}
        {canRefund && isRefundable && (
          <Button variant="destructive" onClick={() => { setRefundFormError(null); setRefundOpen(true) }}>
            <RotateCcw className="mr-2 h-4 w-4" /> Issue refund
          </Button>
        )}
      </div>

      <RefundDialog
        open={refundOpen}
        onClose={() => setRefundOpen(false)}
        payment={payment}
        amount={refundAmount}
        setAmount={onRefundAmountChange}
        reason={refundReason}
        setReason={setRefundReason}
        onSubmit={() => {
          const e = validateRefundAmount(refundAmount)
          if (e) {
            setRefundFormError(e)
            return
          }
          refundMut.mutate()
        }}
        pending={refundMut.isPending}
        formError={refundFormError}
        remainingPaise={remainingPaise}
        remainingRupees={remainingRupees}
      />
    </div>
  )
}

function BackLink() {
  return (
    <Link href="/payments" className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground">
      <ArrowLeft className="mr-1.5 h-3.5 w-3.5" /> Payments
    </Link>
  )
}

function Row({ label, value, bold, className }: { label: string; value: React.ReactNode; bold?: boolean; className?: string }) {
  return (
    <div className="flex justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn(bold && 'font-semibold', className)}>{value}</span>
    </div>
  )
}

function CopyRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4 items-center">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono text-xs flex items-center gap-1">
        {value}
        <Button
          variant="ghost" size="icon" className="h-6 w-6"
          onClick={() => { navigator.clipboard.writeText(value); toast.success('Copied') }}
        >
          <Copy className="h-3 w-3" />
        </Button>
      </span>
    </div>
  )
}

function RefundDialog(props: {
  open: boolean
  onClose: () => void
  payment: Payment
  amount: string
  setAmount: (v: string) => void
  reason: string
  setReason: (v: string) => void
  onSubmit: () => void
  pending: boolean
  formError: string | null
  remainingPaise: number
  remainingRupees: number
}) {
  return (
    <Dialog open={props.open} onOpenChange={(v) => !v && props.onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Refund payment</DialogTitle>
          <DialogDescription>
            Leave amount blank for a full refund of the remaining{' '}
            <strong>{formatPrice(props.remainingPaise)}</strong>.
            {props.payment.amount_refunded > 0 && (
              <span className="block mt-1">
                Already refunded: {formatPrice(props.payment.amount_refunded)} of {formatPrice(props.payment.amount)}.
              </span>
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <Label htmlFor="refund-amt">Amount (₹)</Label>
            <Input
              id="refund-amt" type="number" step="0.01" min="0"
              max={props.remainingRupees}
              placeholder={`Full remaining (${props.remainingRupees.toFixed(2)})`}
              value={props.amount}
              onChange={(e) => props.setAmount(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Max refundable: ₹{props.remainingRupees.toFixed(2)}
            </p>
          </div>
          <div>
            <Label htmlFor="refund-reason">Reason</Label>
            <Input
              id="refund-reason"
              placeholder="Customer requested cancellation"
              value={props.reason}
              onChange={(e) => props.setReason(e.target.value)}
            />
          </div>
          {props.formError && (
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>{props.formError}</AlertDescription>
            </Alert>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={props.onClose}>Cancel</Button>
          <Button
            variant="destructive"
            onClick={props.onSubmit}
            disabled={props.pending || !!props.formError}
          >
            {props.pending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Issue refund
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
