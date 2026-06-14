/* eslint-disable react-hooks/incompatible-library */
'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Plus, Edit, Trash2, Loader2, Store, Search } from 'lucide-react'
import { toast } from 'sonner'

import { marketplaceApi } from '@/lib/api/marketplace'
import { productsApi } from '@/lib/api/products'
import { locationsApi } from '@/lib/api/locations'
import { formatPrice } from '@/lib/utils'
import type { MarketplaceListing } from '@/types/api'

import { Button } from '@/components/ui/button'
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

const listingSchema = z.object({
  product_id: z.string().min(1, 'Product is required'),
  location_id: z.string().optional(),
  listing_price: z.number().min(0, 'Price must be positive'),
  min_order_qty: z.number().min(1, 'Minimum order must be at least 1'),
  max_order_qty: z.number().optional(),
})
type ListingFormValues = z.infer<typeof listingSchema>

export default function MarketplacePage() {
  const queryClient = useQueryClient()
  // State for Browse tab
  const [searchQuery, setSearchQuery] = useState('')
  const [minPrice, setMinPrice] = useState('')
  const [maxPrice, setMaxPrice] = useState('')

  // State for My Listings tab dialogs
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedListing, setSelectedListing] = useState<MarketplaceListing | null>(null)

  // Fetch Data
  const { data: searchResults, isLoading: isSearching } = useQuery({
    queryKey: ['marketplace', 'search', searchQuery, minPrice, maxPrice],
    queryFn: () => marketplaceApi.search({
      q: searchQuery || undefined,
      min_price: minPrice ? Number(minPrice) * 100 : undefined,
      max_price: maxPrice ? Number(maxPrice) * 100 : undefined,
    }),
  })

  const { data: rawListings, isLoading: isLoadingListings } = useQuery({
    queryKey: ['listings'],
    queryFn: marketplaceApi.listByOrg,
  })

  const { data: rawProducts } = useQuery({ queryKey: ['products'], queryFn: productsApi.list })
  const { data: rawLocations } = useQuery({ queryKey: ['locations'], queryFn: locationsApi.list })

  const listings = rawListings ?? []
  const products = rawProducts ?? []
  const locations = rawLocations ?? []

  // Mutations
  const createMutation = useMutation({
    mutationFn: marketplaceApi.createListing,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['listings'] })
      queryClient.invalidateQueries({ queryKey: ['marketplace'] })
      setIsDialogOpen(false)
      reset()
      toast.success('Listing created successfully')
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof marketplaceApi.updateListing>[1] }) => marketplaceApi.updateListing(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['listings'] })
      queryClient.invalidateQueries({ queryKey: ['marketplace'] })
      setIsDialogOpen(false)
      toast.success('Listing updated successfully')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: marketplaceApi.deleteListing,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['listings'] })
      queryClient.invalidateQueries({ queryKey: ['marketplace'] })
      setIsDeleteDialogOpen(false)
      toast.success('Listing deleted successfully')
    },
  })

  const addToCartMutation = useMutation({
    mutationFn: marketplaceApi.addToCart,
    onSuccess: () => {
      toast.success('Added to cart')
    },
  })

  // Form Setup
  const { register, handleSubmit, reset, setValue, watch, formState: { errors, isSubmitting } } = useForm<ListingFormValues>({
    resolver: zodResolver(listingSchema),
    defaultValues: { product_id: '', location_id: '', listing_price: 0, min_order_qty: 1, max_order_qty: undefined }
  })

  const handleOpenCreate = () => {
    setSelectedListing(null)
    reset({ product_id: '', location_id: '', listing_price: 0, min_order_qty: 1, max_order_qty: undefined })
    setIsDialogOpen(true)
  }

  const handleOpenEdit = (listing: MarketplaceListing) => {
    setSelectedListing(listing)
    reset({
      product_id: listing.product_id,
      location_id: listing.location_id || '',
      listing_price: listing.listing_price / 100, // convert paise to rupees
      min_order_qty: listing.min_order_qty,
      max_order_qty: listing.max_order_qty ?? undefined,
    })
    setIsDialogOpen(true)
  }

  const onSubmit = (data: ListingFormValues) => {
    const payload = {
      ...data,
      max_order_qty: data.max_order_qty ?? undefined,
      listing_price: Math.round(data.listing_price * 100), // convert rupees to paise
      location_id: data.location_id === 'none' || !data.location_id ? undefined : data.location_id,
    }

    if (selectedListing) {
      updateMutation.mutate({ id: selectedListing.id, data: payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Marketplace</h2>
          <p className="text-muted-foreground">Discover products or manage your own listings.</p>
        </div>
      </div>

      <Tabs defaultValue="browse" className="w-full">
        <TabsList className="grid w-full grid-cols-2 max-w-[400px]">
          <TabsTrigger value="browse">Browse Marketplace</TabsTrigger>
          <TabsTrigger value="listings">My Listings</TabsTrigger>
        </TabsList>
        
        {/* BROWSE TAB */}
        <TabsContent value="browse" className="pt-6 space-y-6">
          <div className="flex flex-col sm:flex-row gap-4 items-end bg-white dark:bg-zinc-950 p-4 rounded-md border">
            <div className="w-full sm:w-1/2 space-y-2">
              <Label htmlFor="search">Search</Label>
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input 
                  id="search"
                  placeholder="Search products, SKUs, or suppliers..." 
                  className="pl-8"
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                />
              </div>
            </div>
            <div className="w-full sm:w-1/4 space-y-2">
              <Label htmlFor="min">Min Price (₹)</Label>
              <Input id="min" type="number" value={minPrice} onChange={e => setMinPrice(e.target.value)} />
            </div>
            <div className="w-full sm:w-1/4 space-y-2">
              <Label htmlFor="max">Max Price (₹)</Label>
              <Input id="max" type="number" value={maxPrice} onChange={e => setMaxPrice(e.target.value)} />
            </div>
          </div>

          {isSearching ? (
            <div className="flex justify-center p-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : searchResults?.length === 0 ? (
            <div className="text-center p-12 border rounded-md bg-white dark:bg-zinc-950">
              <Store className="h-12 w-12 mx-auto text-muted-foreground mb-4 opacity-20" />
              <h3 className="text-lg font-medium">No results found</h3>
              <p className="text-muted-foreground">Try adjusting your search or filters.</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
              {searchResults?.map(item => (
                <Card key={item.id} className="flex flex-col">
                  <CardHeader className="pb-4">
                    <div className="flex justify-between items-start gap-2">
                      <div>
                        <CardTitle className="text-lg line-clamp-1">{item.product_name}</CardTitle>
                        <p className="text-sm text-muted-foreground mt-1 line-clamp-1">{item.org_name}</p>
                      </div>
                      <Badge variant="secondary" className="shrink-0">{formatPrice(item.listing_price)}</Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="flex-1 text-sm space-y-2">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">SKU</span>
                      <span className="font-medium">{item.product_sku}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Stock</span>
                      <span className="font-medium text-emerald-600">{item.stock_quantity ?? 0}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Min Order</span>
                      <span>{item.min_order_qty}</span>
                    </div>
                    {item.location_city && (
                      <div className="flex justify-between pt-2 border-t mt-2">
                        <span className="text-muted-foreground">Ships From</span>
                        <span>{item.location_city}</span>
                      </div>
                    )}
                  </CardContent>
                  <CardFooter className="pt-4 border-t">
                    <Button 
                      className="w-full" 
                      onClick={() => addToCartMutation.mutate({ listing_id: item.id, quantity: item.min_order_qty })}
                      disabled={addToCartMutation.isPending || (item.stock_quantity ?? 0) < item.min_order_qty}
                    >
                      {addToCartMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                      Add to Cart
                    </Button>
                  </CardFooter>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>

        {/* MY LISTINGS TAB */}
        <TabsContent value="listings" className="pt-6 space-y-6">
          <div className="flex justify-end">
            <Button onClick={handleOpenCreate}>
              <Plus className="mr-2 h-4 w-4" /> New Listing
            </Button>
          </div>

          <div className="rounded-md border bg-white dark:bg-zinc-950">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Product</TableHead>
                  <TableHead>Location</TableHead>
                  <TableHead>Price</TableHead>
                  <TableHead>Min/Max Qty</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="w-[100px]"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoadingListings ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center">
                      <Loader2 className="mx-auto h-6 w-6 animate-spin text-muted-foreground" />
                    </TableCell>
                  </TableRow>
                ) : listings.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                      You haven&apos;t created any marketplace listings yet.
                    </TableCell>
                  </TableRow>
                ) : (
                  listings.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell className="font-medium">
                        {l.product_name} <span className="text-muted-foreground font-normal text-xs ml-1">({l.product_sku})</span>
                      </TableCell>
                      <TableCell>{l.location_name || 'Global'}</TableCell>
                      <TableCell>{formatPrice(l.listing_price)}</TableCell>
                      <TableCell>{l.min_order_qty} / {l.max_order_qty || '∞'}</TableCell>
                      <TableCell>
                        {l.is_active ? (
                          <Badge variant="outline" className="bg-emerald-50 text-emerald-600 border-emerald-200">Active</Badge>
                        ) : (
                          <Badge variant="secondary">Inactive</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button variant="ghost" size="icon" onClick={() => handleOpenEdit(l)}>
                            <Edit className="h-4 w-4" />
                          </Button>
                          <Button 
                            variant="ghost" 
                            size="icon" 
                            className="text-red-500 hover:text-red-600"
                            onClick={() => {
                              setSelectedListing(l)
                              setIsDeleteDialogOpen(true)
                            }}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
      </Tabs>

      {/* CREATE/EDIT DIALOG */}
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{selectedListing ? 'Edit Listing' : 'Create Listing'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>Product *</Label>
              <Select 
                value={watch('product_id')} 
                onValueChange={v => setValue('product_id', v, { shouldValidate: true })}
                disabled={!!selectedListing} // Cannot change product of existing listing easily
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a product" />
                </SelectTrigger>
                <SelectContent>
                  {products.map(p => (
                    <SelectItem key={p.id} value={p.id}>{p.name} ({p.sku})</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.product_id && <p className="text-sm text-red-500">{errors.product_id.message}</p>}
            </div>

            <div className="space-y-2">
              <Label>Location (Optional)</Label>
              <Select 
                value={watch('location_id') || 'none'} 
                onValueChange={v => setValue('location_id', v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select dispatch location" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">Any / Global</SelectItem>
                  {locations.map(l => (
                    <SelectItem key={l.id} value={l.id}>{l.name} ({l.city})</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Listing Price (₹) *</Label>
                <Input type="number" step="0.01" {...register('listing_price', { valueAsNumber: true })} />
                {errors.listing_price && <p className="text-sm text-red-500">{errors.listing_price.message}</p>}
              </div>
              <div className="space-y-2">
                <Label>Min Order Qty *</Label>
                <Input type="number" {...register('min_order_qty', { valueAsNumber: true })} />
                {errors.min_order_qty && <p className="text-sm text-red-500">{errors.min_order_qty.message}</p>}
              </div>
              <div className="space-y-2">
                <Label>Max Order Qty (Optional)</Label>
                <Input type="number" {...register('max_order_qty', { valueAsNumber: true, setValueAs: v => v === "" || isNaN(v) ? undefined : v })} />
              </div>
            </div>

            <DialogFooter className="pt-4">
              <Button type="button" variant="outline" onClick={() => setIsDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {selectedListing ? 'Update' : 'Create'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* DELETE DIALOG */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Listing</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <p>Are you sure you want to remove this product from the marketplace?</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>Cancel</Button>
            <Button 
              variant="destructive" 
              onClick={() => {
                if (selectedListing) deleteMutation.mutate(selectedListing.id)
              }}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
