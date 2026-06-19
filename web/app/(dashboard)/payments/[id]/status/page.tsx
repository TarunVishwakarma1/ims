'use client'

import { use, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { Loader2, CircleCheck, CircleX, Clock, ArrowRight } from 'lucide-react'

import { paymentsApi } from '@/lib/api/payments'
import { formatPrice } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

export default function PaymentStatusPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()

  // Poll every 2s until terminal status. Status flips from `created` to
  // `captured` / `failed` once the Razorpay webhook lands. 60s ceiling — if
  // the webhook is delayed beyond that the user is told to check later.
  const { data: payment } = useQuery({
    queryKey: ['payment-status', id],
    queryFn: () => paymentsApi.getById(id),
    refetchInterval: (q) => {
      const s = q.state.data?.status
      if (s === 'captured' || s === 'failed' || s === 'refunded') return false
      return 2000
    },
  })

  const status = payment?.status

  // Auto-redirect to receipt on capture, after a short celebration moment.
  useEffect(() => {
    if (status === 'captured') {
      const t = setTimeout(() => router.push(`/payments/${id}/receipt`), 1500)
      return () => clearTimeout(t)
    }
  }, [status, id, router])

  return (
    <div className="max-w-lg mx-auto py-8">
      <Card>
        <CardContent className="py-10 text-center space-y-4">
          {!payment && <Loading message="Loading…" />}

          {status === 'created' && <Loading message="Waiting for confirmation from Razorpay…" />}
          {status === 'authorized' && <Loading message="Payment authorized — capturing…" />}

          {status === 'captured' && (
            <div className="space-y-3">
              <div className="mx-auto h-14 w-14 rounded-full bg-emerald-100 dark:bg-emerald-950 flex items-center justify-center">
                <CircleCheck className="h-7 w-7 text-emerald-600 dark:text-emerald-400" />
              </div>
              <div>
                <h2 className="text-xl font-bold">Payment received</h2>
                <p className="text-sm text-muted-foreground">
                  {payment && formatPrice(payment.amount)} captured. Redirecting to your receipt…
                </p>
              </div>
            </div>
          )}

          {status === 'failed' && (
            <div className="space-y-3">
              <div className="mx-auto h-14 w-14 rounded-full bg-rose-100 dark:bg-rose-950 flex items-center justify-center">
                <CircleX className="h-7 w-7 text-rose-600 dark:text-rose-400" />
              </div>
              <div>
                <h2 className="text-xl font-bold">Payment failed</h2>
                <p className="text-sm text-muted-foreground">
                  {payment?.failure_reason || 'The payment could not be completed.'}
                </p>
              </div>
              <div className="flex gap-2 justify-center">
                {payment?.order_id && (
                  <Button onClick={() => router.push(`/orders/${payment.order_id}`)}>
                    Retry from order <ArrowRight className="ml-2 h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          )}

          {status === 'refunded' && (
            <div className="space-y-3">
              <div className="mx-auto h-14 w-14 rounded-full bg-violet-100 dark:bg-violet-950 flex items-center justify-center">
                <Clock className="h-7 w-7 text-violet-600 dark:text-violet-400" />
              </div>
              <h2 className="text-xl font-bold">Payment refunded</h2>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function Loading({ message }: { message: string }) {
  return (
    <div className="space-y-3">
      <div className="mx-auto h-14 w-14 rounded-full bg-indigo-100 dark:bg-indigo-950 flex items-center justify-center">
        <Loader2 className="h-7 w-7 text-indigo-600 dark:text-indigo-400 animate-spin" />
      </div>
      <div>
        <h2 className="text-xl font-bold">Processing payment</h2>
        <p className="text-sm text-muted-foreground">{message}</p>
        <p className="text-xs text-muted-foreground mt-2">
          You can safely leave this page — we&apos;ll email you a receipt.
        </p>
      </div>
    </div>
  )
}
