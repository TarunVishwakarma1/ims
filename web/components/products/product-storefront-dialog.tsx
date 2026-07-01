'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { productStorefrontApi } from '@/lib/api/product-storefront'
import type { Product, ProductStorefront } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'

export function ProductStorefrontDialog({
  product, open, onOpenChange,
}: {
  product: Product | null
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Storefront{product ? ` — ${product.name}` : ''}</DialogTitle>
          <DialogDescription>Control how this product appears in your Kirana shop.</DialogDescription>
        </DialogHeader>
        {product && open && <Body product={product} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function Body({ product, onDone }: { product: Product; onDone: () => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ['product-storefront', product.id],
    queryFn: () => productStorefrontApi.get(product.id),
  })
  if (isLoading || !data) {
    return <div className="py-8 grid place-items-center"><Loader2 className="animate-spin" /></div>
  }
  return <Form key={product.id} product={product} initial={data} onDone={onDone} />
}

function Form({ product, initial, onDone }: { product: Product; initial: ProductStorefront; onDone: () => void }) {
  const qc = useQueryClient()
  const [visible, setVisible] = useState(initial.shop_visible)
  const [slug, setSlug] = useState(initial.shop_slug)
  const [priceInput, setPriceInput] = useState(
    initial.shop_price_paise != null ? String(initial.shop_price_paise / 100) : '',
  )
  const [description, setDescription] = useState(initial.shop_description)
  const [imagesText, setImagesText] = useState(initial.shop_image_urls.join('\n'))

  const save = useMutation({
    mutationFn: () =>
      productStorefrontApi.set(product.id, {
        shop_visible: visible,
        shop_slug: slug.trim(),
        shop_price_paise: priceInput.trim() === '' ? null : Math.round(Number(priceInput) * 100),
        shop_description: description,
        shop_image_urls: imagesText.split('\n').map((s) => s.trim()).filter(Boolean),
      }),
    onSuccess: (p) => {
      qc.setQueryData(['product-storefront', product.id], p)
      toast.success('Storefront updated')
      onDone()
    },
    onError: async (e) => {
      if (e instanceof HTTPError) {
        const body = await e.response.json().catch(() => ({}))
        toast.error((body as { error?: string }).error ?? 'Could not update storefront')
      } else {
        toast.error('Could not update storefront')
      }
    },
  })

  return (
    <div className="space-y-4">
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={visible} onChange={(e) => setVisible(e.target.checked)} />
        Show this product in my shop
      </label>
      <div>
        <Label>Storefront slug</Label>
        <Input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="parle-g-200g" />
        <p className="mt-1 text-xs text-muted-foreground">
          Required to be visible. Lowercase letters, numbers, hyphens. Public URL: /p/{slug || 'your-product'}
        </p>
      </div>
      <div>
        <Label>Shop price (₹)</Label>
        <Input type="number" min="0" step="0.01" value={priceInput}
          onChange={(e) => setPriceInput(e.target.value)}
          placeholder={`Base price ₹${(product.price / 100).toFixed(2)}`} />
        <p className="mt-1 text-xs text-muted-foreground">Leave blank to use the base price.</p>
      </div>
      <div>
        <Label>Shop description</Label>
        <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
      </div>
      <div>
        <Label>Image URLs (one per line)</Label>
        <Textarea value={imagesText} onChange={(e) => setImagesText(e.target.value)} rows={2}
          placeholder="https://…/photo.jpg" />
      </div>
      <DialogFooter>
        <Button onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  )
}
