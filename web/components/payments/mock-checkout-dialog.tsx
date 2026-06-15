'use client'

import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { CreditCard, Smartphone, Building2, Loader2, ShieldCheck, XCircle } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { paymentsApi } from '@/lib/api/payments'
import { formatPrice } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

interface MockCheckoutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  razorpayOrderID: string
  amount: number // paise
  onSuccess: () => void
  onFailure?: () => void
}

const METHODS = [
  { id: 'card', label: 'Card', icon: CreditCard },
  { id: 'upi', label: 'UPI', icon: Smartphone },
  { id: 'netbanking', label: 'Netbanking', icon: Building2 },
]

export function MockCheckoutDialog({
  open,
  onOpenChange,
  razorpayOrderID,
  amount,
  onSuccess,
  onFailure,
}: MockCheckoutDialogProps) {
  const [selectedMethod, setSelectedMethod] = useState('card')

  const captureMutation = useMutation({
    mutationFn: () => paymentsApi.mockCapture(razorpayOrderID),
    onSuccess: () => {
      toast.success('Payment successful')
      onOpenChange(false)
      onSuccess()
    },
    onError: async (err: Error) => {
      let msg = 'Payment failed'
      if (err instanceof HTTPError) {
        try {
          const data = (await err.response.json()) as { error?: string }
          msg = data.error || msg
        } catch {
          /* ignore */
        }
      }
      toast.error(msg)
    },
  })

  const failMutation = useMutation({
    mutationFn: () => paymentsApi.mockFail(razorpayOrderID, 'User cancelled the payment'),
    onSuccess: () => {
      toast.info('Payment cancelled')
      onOpenChange(false)
      onFailure?.()
    },
  })

  return (
    <Dialog open={open} onOpenChange={(next) => !captureMutation.isPending && !failMutation.isPending && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <div className="flex items-center justify-between">
            <DialogTitle className="flex items-center gap-2">
              <ShieldCheck className="h-5 w-5 text-emerald-500" />
              Razorpay Checkout
            </DialogTitle>
            <Badge variant="outline" className="bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-300">
              MOCK
            </Badge>
          </div>
          <DialogDescription>
            Dev-mode payment. No real money will be charged. Order: <code className="text-xs">{razorpayOrderID}</code>
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="rounded-lg border bg-muted/30 px-4 py-3">
            <div className="text-xs text-muted-foreground">Amount payable</div>
            <div className="text-2xl font-bold">{formatPrice(amount)}</div>
          </div>

          <div className="space-y-2">
            <div className="text-xs text-muted-foreground uppercase tracking-wide font-medium">
              Choose payment method
            </div>
            <div className="grid grid-cols-3 gap-2">
              {METHODS.map((m) => {
                const Icon = m.icon
                const isActive = selectedMethod === m.id
                return (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => setSelectedMethod(m.id)}
                    className={`flex flex-col items-center gap-1.5 rounded-lg border-2 p-3 text-xs font-medium transition-all ${
                      isActive
                        ? 'border-emerald-500 bg-emerald-50/50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
                        : 'border-zinc-200 hover:border-zinc-400 dark:border-zinc-800'
                    }`}
                  >
                    <Icon className="h-5 w-5" />
                    {m.label}
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        <DialogFooter className="flex-col sm:flex-row gap-2">
          <Button
            variant="outline"
            onClick={() => failMutation.mutate()}
            disabled={captureMutation.isPending || failMutation.isPending}
            className="flex-1"
          >
            {failMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            <XCircle className="mr-2 h-4 w-4" /> Simulate failure
          </Button>
          <Button
            onClick={() => captureMutation.mutate()}
            disabled={captureMutation.isPending || failMutation.isPending}
            className="flex-1 bg-emerald-500 hover:bg-emerald-400 text-white font-semibold"
          >
            {captureMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Pay {formatPrice(amount)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
