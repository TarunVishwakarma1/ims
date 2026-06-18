'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, Loader2, Minus, Plus, ShoppingBag, Tag } from 'lucide-react'
import { toast } from 'sonner'
import { useRouter } from 'next/navigation'
import { HTTPError } from 'ky'

import { marketplaceApi } from '@/lib/api/marketplace'
import { paymentsApi } from '@/lib/api/payments'
import { couponsApi } from '@/lib/api/coupons'
import { formatPrice, cn } from '@/lib/utils'
import { MockCheckoutDialog } from '@/components/payments/mock-checkout-dialog'
import { openRealCheckout, useRazorpayPreload } from '@/components/payments/real-checkout'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useCartDrawer } from '@/lib/stores/cart-drawer-store'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

interface CartContentProps {
  variant?: 'drawer' | 'page'
}

export function CartContent({ variant = 'page' }: CartContentProps) {
  const queryClient = useQueryClient()
  const router = useRouter()
  const { user } = useAuthStore()
  const closeDrawer = useCartDrawer((s) => s.setOpen)

  const { data: payConfig } = useQuery({
    queryKey: ['payment-config'],
    queryFn: paymentsApi.getConfig,
    staleTime: 60 * 60 * 1000,
  })
  useRazorpayPreload(payConfig?.mock === false)

  const { data: cart, isLoading } = useQuery({
    queryKey: ['cart'],
    queryFn: marketplaceApi.getCart,
    retry: false,
  })

  const updateQuantityMutation = useMutation({
    mutationFn: ({ listingId, quantity }: { listingId: string; quantity: number }) =>
      marketplaceApi.updateCartItem(listingId, quantity),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['cart'] }),
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({}))
        toast.error(data.error || data.message || 'Failed to update quantity')
      } else toast.error(err.message || 'Failed to update quantity')
    },
  })

  const removeItemMutation = useMutation({
    mutationFn: marketplaceApi.removeFromCart,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success('Item removed')
    },
  })

  const [payQueue, setPayQueue] = useState<{ id: string; amount: number }[]>([])
  const [payIdx, setPayIdx] = useState(0)
  const [paying, setPaying] = useState(false)
  const [mockDialog, setMockDialog] = useState<{
    open: boolean
    razorpayOrderID: string
    paymentID: string
    amount: number
  }>({ open: false, razorpayOrderID: '', paymentID: '', amount: 0 })

  const payNext = async (queue: { id: string; amount: number }[], idx: number) => {
    if (idx >= queue.length) {
      setPaying(false)
      setPayQueue([])
      setPayIdx(0)
      toast.success(`All ${queue.length} order(s) processed.`)
      router.push('/orders')
      closeDrawer(false)
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
          if (idx === queue.length - 1) {
            closeDrawer(false)
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
          closeDrawer(false)
          router.push('/orders')
        },
      })
      return
    }

    setMockDialog({
      open: true,
      razorpayOrderID: paymentOrder.razorpay_order_id,
      paymentID: paymentOrder.payment.id,
      amount: paymentOrder.amount,
    })
  }

  const [coupons, setCoupons] = useState<Record<string, { code: string; amountOff: number; error?: string }>>({})

  const checkoutMutation = useMutation({
    mutationFn: async () => {
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
        const data = await err.response.json().catch(() => ({}))
        toast.error(data.error || data.message || 'Checkout failed')
      } else toast.error(err.message || 'Checkout failed')
    },
  })

  const items = cart?.items || []
  const groupedItems = items.reduce((acc, item) => {
    const orgId = item.listing?.org_id || 'unknown'
    const orgName = item.listing?.org_name || 'Unknown Supplier'
    if (!acc[orgId]) acc[orgId] = { orgName, items: [], subtotal: 0 }
    acc[orgId].items.push(item)
    acc[orgId].subtotal += (item.listing?.listing_price || 0) * item.quantity
    return acc
  }, {} as Record<string, { orgName: string; items: typeof items; subtotal: number }>)

  const grandSubtotal = Object.values(groupedItems).reduce((sum, g) => sum + g.subtotal, 0)
  const totalDiscount = Object.values(coupons).reduce((sum, c) => sum + (c.amountOff || 0), 0)
  const payable = Math.max(0, grandSubtotal - totalDiscount)

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center px-6 py-16 text-center space-y-3">
        <div className="rounded-full bg-zinc-100 dark:bg-zinc-800 p-5">
          <ShoppingBag className="h-10 w-10 text-zinc-400" />
        </div>
        <h3 className="text-base font-semibold">Your cart is empty</h3>
        <p className="text-xs text-muted-foreground max-w-xs">
          Browse the marketplace to find products from verified suppliers.
        </p>
        <Button
          size="sm"
          onClick={() => {
            closeDrawer(false)
            router.push('/marketplace')
          }}
          className="mt-2"
        >
          Browse marketplace
        </Button>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col h-full', variant === 'page' && 'max-w-3xl mx-auto')}>
      {/* Scrollable items area */}
      <div className="flex-1 overflow-y-auto">
        <div className={cn('space-y-4', variant === 'drawer' ? 'px-4 py-3' : 'p-0')}>
          {Object.entries(groupedItems).map(([orgId, group]) => (
            <div
              key={orgId}
              className="rounded-xl border bg-white dark:bg-zinc-950 overflow-hidden"
            >
              <div className="flex items-center justify-between px-4 py-2.5 bg-zinc-50/80 dark:bg-zinc-900/50 border-b">
                <div className="flex items-center gap-2 min-w-0">
                  <Badge variant="outline" className="font-normal shrink-0">Seller</Badge>
                  <span className="text-sm font-medium truncate">{group.orgName}</span>
                </div>
                <span className="text-xs text-muted-foreground shrink-0 ml-2">
                  {group.items.length} {group.items.length === 1 ? 'item' : 'items'}
                </span>
              </div>

              <div className="divide-y">
                {group.items.map((item) => (
                  <div key={item.id} className="flex gap-3 p-3">
                    {/* Product thumbnail placeholder */}
                    <div className="h-14 w-14 shrink-0 rounded-lg bg-gradient-to-br from-zinc-100 to-zinc-200 dark:from-zinc-800 dark:to-zinc-900 flex items-center justify-center">
                      <ShoppingBag className="h-5 w-5 text-zinc-400" />
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium leading-snug line-clamp-2">
                        {item.listing?.product_name}
                      </div>
                      <div className="mt-0.5 text-xs text-muted-foreground">
                        {formatPrice(item.listing?.listing_price || 0)} each
                      </div>

                      <div className="mt-2 flex items-center justify-between">
                        <div className="inline-flex items-center rounded-md border bg-white dark:bg-zinc-950">
                          <button
                            className="h-7 w-7 flex items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                            disabled={
                              item.quantity <= (item.listing?.min_order_qty || 1) ||
                              updateQuantityMutation.isPending
                            }
                            onClick={() =>
                              updateQuantityMutation.mutate({
                                listingId: item.listing_id,
                                quantity: item.quantity - 1,
                              })
                            }
                          >
                            <Minus className="h-3 w-3" />
                          </button>
                          <span className="w-7 text-center text-sm tabular-nums">{item.quantity}</span>
                          <button
                            className="h-7 w-7 flex items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                            disabled={
                              (item.listing?.max_order_qty && item.quantity >= item.listing.max_order_qty) ||
                              (item.listing?.stock_quantity !== undefined &&
                                item.quantity >= item.listing.stock_quantity) ||
                              updateQuantityMutation.isPending
                            }
                            onClick={() =>
                              updateQuantityMutation.mutate({
                                listingId: item.listing_id,
                                quantity: item.quantity + 1,
                              })
                            }
                          >
                            <Plus className="h-3 w-3" />
                          </button>
                        </div>
                        <div className="flex items-center gap-2">
                          <div className="text-sm font-semibold">
                            {formatPrice((item.listing?.listing_price || 0) * item.quantity)}
                          </div>
                          <button
                            className="h-7 w-7 flex items-center justify-center rounded-md text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-950/30 disabled:opacity-40"
                            disabled={removeItemMutation.isPending}
                            onClick={() => removeItemMutation.mutate(item.listing_id)}
                            aria-label="Remove item"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Per-supplier coupon + subtotal */}
              <div className="border-t bg-zinc-50/50 dark:bg-zinc-900/30 px-3 py-2.5 space-y-2">
                <CouponBox
                  supplierOrgId={orgId}
                  subtotal={group.subtotal}
                  applied={coupons[orgId]}
                  onApplied={(val) => setCoupons((s) => ({ ...s, [orgId]: val }))}
                  onCleared={() =>
                    setCoupons((s) => {
                      const next = { ...s }
                      delete next[orgId]
                      return next
                    })
                  }
                />
                <div className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">Subtotal</span>
                  <span className="font-semibold">{formatPrice(group.subtotal)}</span>
                </div>
                {coupons[orgId]?.amountOff > 0 && (
                  <div className="flex items-center justify-between text-xs text-emerald-700 dark:text-emerald-400">
                    <span>Coupon ({coupons[orgId].code})</span>
                    <span>-{formatPrice(coupons[orgId].amountOff)}</span>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Sticky checkout footer */}
      <div className="border-t bg-white dark:bg-zinc-950 px-4 py-3 space-y-3">
        <div className="space-y-1">
          <div className="flex justify-between text-xs">
            <span className="text-muted-foreground">Items</span>
            <span>{items.length}</span>
          </div>
          <div className="flex justify-between text-xs">
            <span className="text-muted-foreground">Subtotal</span>
            <span>{formatPrice(grandSubtotal)}</span>
          </div>
          {totalDiscount > 0 && (
            <div className="flex justify-between text-xs text-emerald-700 dark:text-emerald-400">
              <span>Coupon savings</span>
              <span>-{formatPrice(totalDiscount)}</span>
            </div>
          )}
          <Separator className="my-1.5" />
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">Payable</span>
            <span className="text-lg font-bold">{formatPrice(payable)}</span>
          </div>
          <p className="text-[10px] text-muted-foreground">
            Tax + delivery calculated at order creation.
          </p>
        </div>

        {payConfig && !payConfig.mock && !payConfig.live && (
          <div className="rounded-md border border-amber-300 bg-amber-50 dark:bg-amber-950/30 px-2.5 py-1.5 text-[10px] text-amber-900 dark:text-amber-300">
            Test sandbox — card <code>4111 1111 1111 1111</code>
          </div>
        )}
        {payConfig?.mock && (
          <div className="rounded-md border border-zinc-300 bg-zinc-50 dark:bg-zinc-900 px-2.5 py-1.5 text-[10px] text-muted-foreground">
            Mock mode — no real RazorPay call.
          </div>
        )}
        {paying && payQueue.length > 1 && (
          <div className="rounded-md border border-indigo-300 bg-indigo-50 dark:bg-indigo-950/30 px-2.5 py-1.5 text-[10px] text-indigo-900 dark:text-indigo-200">
            Paying order {payIdx + 1} of {payQueue.length}
          </div>
        )}

        <Button
          className="w-full"
          size="lg"
          disabled={checkoutMutation.isPending || paying || items.length === 0}
          onClick={() => checkoutMutation.mutate()}
        >
          {(checkoutMutation.isPending || paying) && (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          )}
          Checkout · {formatPrice(payable)}
        </Button>
      </div>

      <MockCheckoutDialog
        open={mockDialog.open}
        onOpenChange={(next) => setMockDialog((s) => ({ ...s, open: next }))}
        razorpayOrderID={mockDialog.razorpayOrderID}
        amount={mockDialog.amount}
        onSuccess={() => {
          setMockDialog((s) => ({ ...s, open: false }))
          if (payIdx === payQueue.length - 1) {
            const lastPaymentID = mockDialog.paymentID
            setPaying(false)
            setPayQueue([])
            closeDrawer(false)
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
          closeDrawer(false)
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
    <div className="space-y-1">
      <div className="flex items-center gap-1.5">
        <Tag className="h-3 w-3 text-muted-foreground" />
        <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium">
          Coupon code
        </span>
      </div>
      <div className="flex items-center gap-2">
        <Input
          placeholder="SAVE10"
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          disabled={isApplied || pending}
          className="h-7 text-xs flex-1"
        />
        {isApplied ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => {
              setCode('')
              onCleared()
            }}
          >
            Remove
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={apply}
            disabled={!code.trim() || pending}
          >
            {pending && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
            Apply
          </Button>
        )}
      </div>
      {applied?.error && !isApplied && (
        <p className="text-[10px] text-rose-600">{applied.error}</p>
      )}
    </div>
  )
}
