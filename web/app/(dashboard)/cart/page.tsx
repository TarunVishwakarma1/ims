'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, Loader2, Minus, Plus, ShoppingBag } from 'lucide-react'
import { toast } from 'sonner'
import { useRouter } from 'next/navigation'
import { HTTPError } from 'ky'

import { marketplaceApi } from '@/lib/api/marketplace'
import { paymentsApi } from '@/lib/api/payments'
import { couponsApi } from '@/lib/api/coupons'
import { formatPrice } from '@/lib/utils'
import { MockCheckoutDialog } from '@/components/payments/mock-checkout-dialog'
import { openRealCheckout, useRazorpayPreload } from '@/components/payments/real-checkout'
import { useAuthStore } from '@/lib/stores/auth-store'

import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

export default function CartPage() {
  const queryClient = useQueryClient()
  const router = useRouter()
  const { user } = useAuthStore()

  // Fetch payment config once — decides mock vs real RazorPay path
  const { data: payConfig } = useQuery({
    queryKey: ['payment-config'],
    queryFn: paymentsApi.getConfig,
    staleTime: 60 * 60 * 1000, // 1h — config rarely changes
  })

  // Preload real RazorPay script when we'll need it
  useRazorpayPreload(payConfig?.mock === false)

  const { data: cart, isLoading } = useQuery({
    queryKey: ['cart'],
    queryFn: marketplaceApi.getCart,
    retry: false, // Don't retry if cart not found (404)
  })

  const updateQuantityMutation = useMutation({
    mutationFn: ({ listingId, quantity }: { listingId: string; quantity: number }) => 
      marketplaceApi.updateCartItem(listingId, quantity),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const errorData = await err.response.json().catch(() => ({}))
        toast.error(errorData.error || errorData.message || 'Failed to update quantity')
      } else {
        toast.error(err.message || 'Failed to update quantity')
      }
    }
  })

  const removeItemMutation = useMutation({
    mutationFn: marketplaceApi.removeFromCart,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success('Item removed')
    },
  })

  // Queue of orders waiting to be paid. Populated on Checkout; emptied one
  // order at a time as each Razorpay widget closes. Sequential because
  // Razorpay's widget is singleton-per-page and split-tender isn't a thing
  // here (one payment per supplier order).
  const [payQueue, setPayQueue] = useState<{ id: string; amount: number }[]>([])
  const [payIdx, setPayIdx] = useState(0)
  const [paying, setPaying] = useState(false)

  // Mock-mode dialog (held open between orders in the queue).
  const [mockDialog, setMockDialog] = useState<{
    open: boolean
    razorpayOrderID: string
    paymentID: string
    amount: number
  }>({ open: false, razorpayOrderID: '', paymentID: '', amount: 0 })

  // payNext starts (or continues) the payment loop. Picks the queue item at
  // the given index, creates a Razorpay order, and either opens the real
  // widget or the mock dialog. Real-mode handlers advance; mock-mode advances
  // when its dialog's onSuccess/onFailure fires.
  const payNext = async (queue: { id: string; amount: number }[], idx: number) => {
    if (idx >= queue.length) {
      setPaying(false)
      setPayQueue([])
      setPayIdx(0)
      toast.success(`All ${queue.length} order(s) processed.`)
      // Last payment is on /payments/[id]/status — but if we got here all
      // are real-mode and already redirected, so this only runs in mock.
      router.push('/orders')
      return
    }
    const target = queue[idx]
    let paymentOrder
    try {
      paymentOrder = await paymentsApi.createOrder({
        order_id: target.id,
        amount: target.amount,
      })
    } catch (err) {
      const msg = err instanceof HTTPError
        ? (await err.response.json().catch(() => ({} as { error?: string }))).error
        : (err instanceof Error ? err.message : 'Payment setup failed')
      toast.error(msg || 'Payment setup failed')
      setPaying(false)
      return
    }

    if (payConfig && !payConfig.mock && payConfig.key_id) {
      openRealCheckout({
        keyID: payConfig.key_id,
        razorpayOrderID: paymentOrder.razorpay_order_id,
        amount: paymentOrder.amount,
        prefill: { name: user?.name, email: user?.email },
        notes: { internal_order_id: target.id },
        onSuccess: () => {
          // Last order? Redirect to status page for it. Otherwise continue.
          if (idx === queue.length - 1) {
            router.push(`/payments/${paymentOrder.payment.id}/status`)
            return
          }
          toast.success(`Order ${idx + 1} of ${queue.length} submitted`)
          setPayIdx(idx + 1)
          payNext(queue, idx + 1)
        },
        onDismiss: () => {
          toast.info(`Checkout dismissed. Remaining order(s) stay unpaid.`)
          setPaying(false)
          setPayQueue([])
          router.push('/orders')
        },
      })
      return
    }

    // Mock mode — open the dev dialog, advance from its onSuccess/onFailure.
    setMockDialog({
      open: true,
      razorpayOrderID: paymentOrder.razorpay_order_id,
      paymentID: paymentOrder.payment.id,
      amount: paymentOrder.amount,
    })
  }

  // Per-supplier coupon state: { [supplierOrgId]: { code, amountOff, error } }.
  // The validate endpoint is called on Apply; checkout sends the codes only.
  const [coupons, setCoupons] = useState<Record<string, { code: string; amountOff: number; error?: string }>>({})

  const checkoutMutation = useMutation({
    mutationFn: async () => {
      // Send only applied (positive-discount) coupons.
      const couponsBySupplier: Record<string, string> = {}
      for (const [supId, c] of Object.entries(coupons)) {
        if (c.amountOff > 0) couponsBySupplier[supId] = c.code
      }
      const orders = await marketplaceApi.checkout({ couponsBySupplier })
      if (!orders || orders.length === 0) throw new Error('No orders created')
      return orders
    },
    onSuccess: (orders) => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      const queue = orders.map((o) => ({ id: o.id, amount: o.total_amount }))
      setPayQueue(queue)
      setPayIdx(0)
      setPaying(true)
      payNext(queue, 0)
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const errorData = await err.response.json().catch(() => ({}))
        toast.error(errorData.error || errorData.message || 'Checkout failed')
      } else {
        toast.error(err.message || 'Checkout failed')
      }
    },
  })

  const items = cart?.items || []

  // Group items by supplier org
  const groupedItems = items.reduce((acc, item) => {
    const orgId = item.listing?.org_id || 'unknown'
    const orgName = item.listing?.org_name || 'Unknown Supplier'
    
    if (!acc[orgId]) {
      acc[orgId] = { orgName, items: [], subtotal: 0 }
    }
    
    acc[orgId].items.push(item)
    acc[orgId].subtotal += (item.listing?.listing_price || 0) * item.quantity
    
    return acc
  }, {} as Record<string, { orgName: string; items: typeof items; subtotal: number }>)

  const grandTotal = Object.values(groupedItems).reduce((sum, group) => sum + group.subtotal, 0)

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center pt-24">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center pt-24 space-y-4">
        <ShoppingBag className="h-16 w-16 text-muted-foreground opacity-20" />
        <h2 className="text-2xl font-semibold tracking-tight">Your cart is empty</h2>
        <p className="text-muted-foreground">Browse the marketplace to add products.</p>
        <Button onClick={() => router.push('/marketplace')} className="mt-4">
          Go to Marketplace
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Shopping Cart</h2>
        <p className="text-muted-foreground">Review your items before checkout.</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-6">
          {Object.entries(groupedItems).map(([orgId, group]) => (
            <Card key={orgId}>
              <CardHeader className="bg-muted/50 pb-4">
                <CardTitle className="text-lg">Sold by: {group.orgName}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Product</TableHead>
                      <TableHead>Price</TableHead>
                      <TableHead>Qty</TableHead>
                      <TableHead className="text-right">Total</TableHead>
                      <TableHead></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {group.items.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell className="font-medium">
                          {item.listing?.product_name}
                        </TableCell>
                        <TableCell>{formatPrice(item.listing?.listing_price || 0)}</TableCell>
                        <TableCell>
                          <div className="flex items-center space-x-2">
                            <Button 
                              variant="outline" 
                              size="icon" 
                              className="h-6 w-6"
                              disabled={item.quantity <= (item.listing?.min_order_qty || 1) || updateQuantityMutation.isPending}
                              onClick={() => updateQuantityMutation.mutate({ listingId: item.listing_id, quantity: item.quantity - 1 })}
                            >
                              <Minus className="h-3 w-3" />
                            </Button>
                            <span className="w-4 text-center text-sm">{item.quantity}</span>
                            <Button 
                              variant="outline" 
                              size="icon" 
                              className="h-6 w-6"
                              disabled={(item.listing?.max_order_qty && item.quantity >= item.listing.max_order_qty) || (item.listing?.stock_quantity !== undefined && item.quantity >= item.listing.stock_quantity) || updateQuantityMutation.isPending}
                              onClick={() => updateQuantityMutation.mutate({ listingId: item.listing_id, quantity: item.quantity + 1 })}
                            >
                              <Plus className="h-3 w-3" />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell className="text-right font-medium">
                          {formatPrice((item.listing?.listing_price || 0) * item.quantity)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Button 
                            variant="ghost" 
                            size="icon" 
                            className="text-red-500 hover:text-red-600 h-8 w-8"
                            disabled={removeItemMutation.isPending}
                            onClick={() => removeItemMutation.mutate(item.listing_id)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
              <CardFooter className="flex-col items-stretch gap-3 border-t py-4">
                <CouponBox
                  supplierOrgId={orgId}
                  subtotal={group.subtotal}
                  applied={coupons[orgId]}
                  onApplied={(val) => setCoupons((s) => ({ ...s, [orgId]: val }))}
                  onCleared={() => setCoupons((s) => {
                    const next = { ...s }
                    delete next[orgId]
                    return next
                  })}
                />
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Supplier Subtotal</span>
                  <span className="font-semibold">{formatPrice(group.subtotal)}</span>
                </div>
                {coupons[orgId]?.amountOff > 0 && (
                  <div className="flex justify-between text-emerald-700 dark:text-emerald-400">
                    <span>Coupon ({coupons[orgId].code})</span>
                    <span>-{formatPrice(coupons[orgId].amountOff)}</span>
                  </div>
                )}
              </CardFooter>
            </Card>
          ))}
        </div>

        <div>
          <Card className="sticky top-6">
            <CardHeader>
              <CardTitle>Order Summary</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Items</span>
                <span>{items.length}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">Expires</span>
                <span>{cart?.expires_at ? new Date(cart.expires_at).toLocaleString() : 'N/A'}</span>
              </div>
              <div className="flex justify-between font-semibold text-lg border-t pt-4">
                <span>Grand Total</span>
                <span>{formatPrice(grandTotal)}</span>
              </div>
            </CardContent>
            <CardFooter className="flex-col gap-3 items-stretch">
              {/* Dev-only hints. Hidden in LIVE so customers see a clean UI. */}
              {payConfig && !payConfig.mock && !payConfig.live && (
                <div className="rounded-md border border-amber-300 bg-amber-50 dark:bg-amber-950/30 px-3 py-2 text-xs text-amber-900 dark:text-amber-300">
                  Test sandbox — use card <code>4111 1111 1111 1111</code>
                </div>
              )}
              {payConfig?.mock && (
                <div className="rounded-md border border-zinc-300 bg-zinc-50 dark:bg-zinc-900 px-3 py-2 text-xs text-muted-foreground">
                  Mock mode — no real RazorPay call.
                </div>
              )}
              {paying && payQueue.length > 1 && (
                <div className="rounded-md border bg-indigo-50 dark:bg-indigo-950/30 px-3 py-2 text-xs text-indigo-900 dark:text-indigo-200">
                  Paying order {payIdx + 1} of {payQueue.length}
                </div>
              )}
              <Button
                className="w-full"
                size="lg"
                disabled={checkoutMutation.isPending || paying || items.length === 0}
                onClick={() => checkoutMutation.mutate()}
              >
                {(checkoutMutation.isPending || paying) && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Checkout
              </Button>
            </CardFooter>
          </Card>
        </div>
      </div>

      <MockCheckoutDialog
        open={mockDialog.open}
        onOpenChange={(next) => setMockDialog((s) => ({ ...s, open: next }))}
        razorpayOrderID={mockDialog.razorpayOrderID}
        amount={mockDialog.amount}
        onSuccess={() => {
          setMockDialog((s) => ({ ...s, open: false }))
          // Last in queue? Route to its status page so we visualize webhook
          // completion. Otherwise advance the queue.
          if (payIdx === payQueue.length - 1) {
            const lastPaymentID = mockDialog.paymentID
            setPaying(false)
            setPayQueue([])
            router.push(`/payments/${lastPaymentID}/status`)
            return
          }
          setPayIdx((i) => i + 1)
          payNext(payQueue, payIdx + 1)
        }}
        onFailure={() => {
          setMockDialog((s) => ({ ...s, open: false }))
          toast.info('Orders remain pending. You can retry payment from your orders list.')
          setPaying(false)
          setPayQueue([])
          router.push('/orders')
        }}
      />
    </div>
  )
}

function CouponBox({
  supplierOrgId,
  subtotal,
  applied,
  onApplied,
  onCleared,
}: {
  supplierOrgId: string
  subtotal: number
  applied?: { code: string; amountOff: number; error?: string }
  onApplied: (val: { code: string; amountOff: number; error?: string }) => void
  onCleared: () => void
}) {
  const [code, setCode] = useState(applied?.code || '')
  const [pending, setPending] = useState(false)
  const isApplied = (applied?.amountOff ?? 0) > 0

  const apply = async () => {
    if (!code.trim()) return
    setPending(true)
    try {
      const res = await couponsApi.validate({
        supplier_org_id: supplierOrgId,
        code: code.trim(),
        subtotal,
      })
      onApplied({ code: code.trim(), amountOff: res.amount_off })
      toast.success(`Coupon applied — saved ${formatPrice(res.amount_off)}`)
    } catch (err) {
      const msg = err instanceof HTTPError
        ? (await err.response.json().catch(() => ({} as { error?: string }))).error
        : (err instanceof Error ? err.message : 'Invalid coupon')
      onApplied({ code: code.trim(), amountOff: 0, error: msg })
      toast.error(msg || 'Invalid coupon')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="flex items-center gap-2">
      <Input
        placeholder="Coupon code"
        value={code}
        onChange={(e) => setCode(e.target.value.toUpperCase())}
        disabled={isApplied || pending}
        className="max-w-[180px]"
      />
      {isApplied ? (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => { setCode(''); onCleared() }}
        >
          Remove
        </Button>
      ) : (
        <Button
          variant="outline"
          size="sm"
          onClick={apply}
          disabled={!code.trim() || pending}
        >
          {pending && <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />}
          Apply
        </Button>
      )}
      {applied?.error && !isApplied && (
        <span className="text-xs text-rose-600">{applied.error}</span>
      )}
    </div>
  )
}
