'use client'

import { use } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Loader2, AlertTriangle, PackageCheck, Edit3, Clock } from 'lucide-react'

import { inventoryApi } from '@/lib/api/inventory'
import { productsApi } from '@/lib/api/products'
import { formatPrice } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

// Parse the audit action string written by inventory_service.Update —
// e.g. "inventory.updated:qty=42,threshold=10". Returns {qty, threshold}.
function parseInventoryAction(action: string): { qty?: number; threshold?: number } {
  const m = action.match(/qty=(\d+).*threshold=(\d+)/)
  if (!m) return {}
  return { qty: Number(m[1]), threshold: Number(m[2]) }
}

export default function InventoryDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()

  const invQ = useQuery({ queryKey: ['inventory', id], queryFn: () => inventoryApi.getById(id) })
  const timelineQ = useQuery({
    queryKey: ['inventory', id, 'timeline'],
    queryFn: () => inventoryApi.getTimeline(id),
  })
  // Pull all products so we can label this one. Cached across pages.
  const productsQ = useQuery({ queryKey: ['products'], queryFn: productsApi.list })

  const inv = invQ.data
  const timeline = timelineQ.data ?? []
  const product = productsQ.data?.find(p => p.id === inv?.product_id)

  if (invQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (!inv) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => router.push('/inventory')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> Back
        </Button>
        <p className="text-muted-foreground">Inventory not found.</p>
      </div>
    )
  }

  const isOutOfStock = inv.quantity === 0
  const isLowStock = !isOutOfStock && inv.quantity <= inv.low_stock_threshold

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => router.push('/inventory')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{product?.name ?? 'Unknown Product'}</h2>
          <div className="text-sm text-muted-foreground font-mono">{product?.sku ?? inv.product_id.split('-')[0]}</div>
        </div>
        <div className="ml-auto">
          {isOutOfStock ? (
            <Badge variant="destructive">Out of stock</Badge>
          ) : isLowStock ? (
            <Badge variant="outline" className="bg-amber-100 text-amber-800 border-amber-200">Low stock</Badge>
          ) : (
            <Badge variant="outline" className="bg-emerald-100 text-emerald-800 border-emerald-200">In stock</Badge>
          )}
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader><CardTitle>Stock</CardTitle></CardHeader>
          <CardContent className="grid grid-cols-3 gap-6">
            <div>
              <div className="text-xs text-muted-foreground">Quantity</div>
              <div className="text-3xl font-semibold">{inv.quantity}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Low-stock threshold</div>
              <div className="text-3xl font-semibold">{inv.low_stock_threshold}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Last updated</div>
              <div className="text-sm">{new Date(inv.updated_at).toLocaleString()}</div>
            </div>
          </CardContent>
        </Card>

        {product && (
          <Card>
            <CardHeader><CardTitle>Product</CardTitle></CardHeader>
            <CardContent className="space-y-2 text-sm">
              {product.description && <p className="text-muted-foreground">{product.description}</p>}
              <div className="flex justify-between">
                <span className="text-muted-foreground">Price</span>
                <span>{formatPrice(product.price)}</span>
              </div>
              <Button variant="outline" size="sm" className="mt-2 w-full"
                onClick={() => router.push(`/products`)}
              >
                <Edit3 className="mr-2 h-4 w-4" /> Open in products
              </Button>
            </CardContent>
          </Card>
        )}
      </div>

      <Card>
        <CardHeader><CardTitle>Stock-change history</CardTitle></CardHeader>
        <CardContent>
          {timelineQ.isLoading ? (
            <div className="py-6 flex justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>
          ) : timeline.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">No changes yet.</p>
          ) : (
            <ol className="relative border-l border-border ml-3 space-y-4">
              {timeline.map((evt) => {
                const parsed = parseInventoryAction(evt.action)
                return (
                  <li key={evt.id} className="ml-4">
                    <span className="absolute -left-3 flex h-6 w-6 items-center justify-center rounded-full bg-background border text-muted-foreground">
                      {parsed.qty === 0
                        ? <AlertTriangle className="h-3.5 w-3.5 text-red-600" />
                        : parsed.qty !== undefined && parsed.threshold !== undefined && parsed.qty <= parsed.threshold
                          ? <AlertTriangle className="h-3.5 w-3.5 text-amber-600" />
                          : <PackageCheck className="h-3.5 w-3.5 text-emerald-600" />}
                    </span>
                    <div className="text-sm font-medium">
                      {parsed.qty !== undefined
                        ? `Stock set to ${parsed.qty} (threshold ${parsed.threshold})`
                        : evt.action}
                    </div>
                    <div className="text-xs text-muted-foreground flex items-center gap-1">
                      <Clock className="h-3 w-3" />
                      {new Date(evt.created_at).toLocaleString()}
                    </div>
                  </li>
                )
              })}
            </ol>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
