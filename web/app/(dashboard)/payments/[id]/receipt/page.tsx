'use client'

import { use } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { ArrowLeft, Printer, CircleCheck, Package, RotateCcw, FileDown, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { paymentsApi } from '@/lib/api/payments'
import { ordersApi } from '@/lib/api/orders'
import { formatPrice } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

export default function ReceiptPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [downloadingPdf, setDownloadingPdf] = useState(false)
  const downloadPdf = async () => {
    setDownloadingPdf(true)
    try {
      await paymentsApi.downloadReceiptPdf(id)
    } catch {
      toast.error('Could not download PDF')
    } finally {
      setDownloadingPdf(false)
    }
  }
  const { data, isLoading } = useQuery({
    queryKey: ['receipt', id],
    queryFn: () => paymentsApi.getReceipt(id),
  })
  // Pull current payment to surface refunded amount on the receipt.
  const { data: payment } = useQuery({
    queryKey: ['payment', id],
    queryFn: () => paymentsApi.getById(id),
  })
  const { data: refunds = [] } = useQuery({
    queryKey: ['refunds', id],
    queryFn: () => paymentsApi.listRefunds(id),
    enabled: !!data,
  })
  // Pull the linked order for tax/discount line breakdown on the receipt.
  const { data: order } = useQuery({
    queryKey: ['order', data?.order_id],
    queryFn: () => ordersApi.getById(data!.order_id),
    enabled: !!data?.order_id,
  })

  if (isLoading || !data) {
    return (
      <div className="max-w-2xl mx-auto py-12 text-center text-muted-foreground">
        Loading receipt…
      </div>
    )
  }

  const captured = new Date(data.captured_at)
  const linesSubtotal = data.items.reduce((sum, it) => {
    const n = parseFloat((it.Total || '0').replace(/[^\d.-]/g, ''))
    return sum + (Number.isFinite(n) ? n : 0)
  }, 0)
  const total = data.amount / 100

  return (
    <div className="max-w-2xl mx-auto space-y-4 print:space-y-2">
      <div className="flex justify-between items-center print:hidden">
        <Link href={`/payments/${data.payment_id}`} className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="mr-1.5 h-3.5 w-3.5" /> Back to payment
        </Link>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={downloadPdf} disabled={downloadingPdf}>
            {downloadingPdf ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <FileDown className="mr-2 h-4 w-4" />
            )}
            Download PDF
          </Button>
          <Button variant="outline" size="sm" onClick={() => window.print()}>
            <Printer className="mr-2 h-4 w-4" /> Print
          </Button>
        </div>
      </div>

      <Card className="overflow-hidden print:border-0 print:shadow-none">
        <div className="bg-gradient-to-br from-indigo-600 via-purple-600 to-pink-600 px-8 py-8 text-white">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Package className="h-6 w-6" />
              <span className="text-lg font-bold tracking-tight">IMS</span>
            </div>
            {data.invoice_number && (
              <div className="text-right">
                <div className="text-[10px] uppercase tracking-widest opacity-75">Tax invoice</div>
                <div className="text-sm font-mono font-semibold">{data.invoice_number}</div>
              </div>
            )}
          </div>
          <div className="text-xs uppercase tracking-widest opacity-80 mb-1">Payment Receipt</div>
          <div className="text-3xl font-bold">{formatPrice(data.amount)}</div>
          <div className="mt-2 inline-flex items-center gap-1.5 bg-white/20 backdrop-blur-sm px-2.5 py-1 rounded-full text-xs">
            <CircleCheck className="h-3 w-3" /> Paid via {data.method || 'Razorpay'}
          </div>
        </div>

        <CardContent className="px-8 py-6 space-y-6">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <div className="text-muted-foreground text-xs">Receipt for</div>
              <div className="font-medium">{data.buyer_name}</div>
              <div className="text-xs text-muted-foreground">{data.buyer_email}</div>
            </div>
            <div className="text-right">
              <div className="text-muted-foreground text-xs">Captured</div>
              <div className="font-medium">{captured.toLocaleDateString()}</div>
              <div className="text-xs text-muted-foreground">{captured.toLocaleTimeString()}</div>
            </div>
          </div>

          <div className="space-y-1.5 text-sm">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Order ID</span>
              <span className="font-mono text-xs">{data.order_id}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Payment ID</span>
              <span className="font-mono text-xs">{data.payment_id}</span>
            </div>
            {data.razorpay_payment_id && (
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">Razorpay payment</span>
                <span className="font-mono text-xs">{data.razorpay_payment_id}</span>
              </div>
            )}
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Organization</span>
              <span>{data.org_name}</span>
            </div>
          </div>

          {data.items.length > 0 && (
            <div>
              <div className="text-xs uppercase tracking-wide text-muted-foreground mb-2">Items</div>
              <div className="border rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-zinc-50 dark:bg-zinc-900/50">
                    <tr>
                      <th className="text-left px-3 py-2 font-medium text-xs uppercase tracking-wide text-muted-foreground">Item</th>
                      <th className="text-right px-3 py-2 font-medium text-xs uppercase tracking-wide text-muted-foreground w-16">Qty</th>
                      <th className="text-right px-3 py-2 font-medium text-xs uppercase tracking-wide text-muted-foreground w-24">Amount</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((it, idx) => (
                      <tr key={idx} className="border-t">
                        <td className="px-3 py-2">{it.Name}</td>
                        <td className="px-3 py-2 text-right">{it.Qty}</td>
                        <td className="px-3 py-2 text-right font-medium">{it.Total}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          <div className="border-t pt-4 space-y-1">
            <div className="flex justify-between text-sm text-muted-foreground">
              <span>Subtotal</span>
              <span>{order ? formatPrice(order.subtotal ?? 0) : `₹${linesSubtotal.toFixed(2)}`}</span>
            </div>
            {order && (order.discount ?? 0) > 0 && (
              <div className="flex justify-between text-sm text-emerald-700 dark:text-emerald-400">
                <span>Discount</span>
                <span>-{formatPrice(order.discount || 0)}</span>
              </div>
            )}
            {order && (order.tax_amount ?? 0) > 0 && (
              order.is_inter_state ? (
                <div className="flex justify-between text-sm text-muted-foreground">
                  <span>IGST</span>
                  <span>{formatPrice(order.tax_igst || 0)}</span>
                </div>
              ) : (
                <>
                  <div className="flex justify-between text-sm text-muted-foreground">
                    <span>CGST</span>
                    <span>{formatPrice(order.tax_cgst || 0)}</span>
                  </div>
                  <div className="flex justify-between text-sm text-muted-foreground">
                    <span>SGST</span>
                    <span>{formatPrice(order.tax_sgst || 0)}</span>
                  </div>
                </>
              )
            )}
            {order && (order.delivery_fee ?? 0) > 0 && (
              <div className="flex justify-between text-sm text-muted-foreground">
                <span>Delivery</span>
                <span>{formatPrice(order.delivery_fee || 0)}</span>
              </div>
            )}
            <div className="flex justify-between text-lg font-bold mt-2 pt-2 border-t">
              <span>Total paid</span>
              <span>{formatPrice(data.amount)}</span>
            </div>
            {payment && payment.amount_refunded > 0 && (
              <>
                <div className="flex justify-between text-sm text-violet-700 dark:text-violet-300 mt-2">
                  <span className="inline-flex items-center gap-1"><RotateCcw className="h-3 w-3" /> Refunded</span>
                  <span>-{formatPrice(payment.amount_refunded)}</span>
                </div>
                <div className="flex justify-between text-sm font-semibold mt-1">
                  <span>Net paid</span>
                  <span>{formatPrice(payment.amount - payment.amount_refunded)}</span>
                </div>
              </>
            )}
          </div>

          {refunds.length > 0 && (
            <div className="border-t pt-4">
              <div className="text-xs uppercase tracking-wide text-muted-foreground mb-2">Refunds issued</div>
              <div className="space-y-1.5">
                {refunds.map((rf) => (
                  <div key={rf.id} className="flex justify-between text-sm">
                    <div>
                      <div className="font-mono text-xs">{rf.razorpay_refund_id || rf.id.slice(0, 8) + '…'}</div>
                      {rf.reason && <div className="text-xs text-muted-foreground">{rf.reason}</div>}
                    </div>
                    <div className="text-right">
                      <div className="font-medium">-{formatPrice(rf.amount)}</div>
                      <div className="text-xs text-muted-foreground">{new Date(rf.created_at).toLocaleDateString()}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <p className="text-xs text-muted-foreground border-t pt-4">
            Keep this receipt for your records. For refunds or queries, contact your administrator with the payment ID above.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
