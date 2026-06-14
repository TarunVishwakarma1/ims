'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, Loader2, Minus, Plus, ShoppingBag } from 'lucide-react'
import { toast } from 'sonner'
import { useRouter } from 'next/navigation'

import { marketplaceApi } from '@/lib/api/marketplace'
import { formatPrice } from '@/lib/utils'

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

export default function CartPage() {
  const queryClient = useQueryClient()
  const router = useRouter()

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
  })

  const removeItemMutation = useMutation({
    mutationFn: marketplaceApi.removeFromCart,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success('Item removed')
    },
  })

  const checkoutMutation = useMutation({
    mutationFn: () => marketplaceApi.checkout(),
    onSuccess: (orders) => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      toast.success(`Checkout successful! Created ${orders.length} orders.`)
      router.push('/orders')
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Checkout failed')
    }
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
                              disabled={(item.listing?.max_order_qty && item.quantity >= item.listing.max_order_qty) || updateQuantityMutation.isPending}
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
              <CardFooter className="flex justify-between border-t py-4">
                <span className="text-muted-foreground">Supplier Subtotal</span>
                <span className="font-semibold">{formatPrice(group.subtotal)}</span>
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
              <div className="flex justify-between font-semibold text-lg border-t pt-4">
                <span>Grand Total</span>
                <span>{formatPrice(grandTotal)}</span>
              </div>
            </CardContent>
            <CardFooter>
              <Button 
                className="w-full" 
                size="lg"
                disabled={checkoutMutation.isPending || items.length === 0}
                onClick={() => checkoutMutation.mutate()}
              >
                {checkoutMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Checkout
              </Button>
            </CardFooter>
          </Card>
        </div>
      </div>
    </div>
  )
}
