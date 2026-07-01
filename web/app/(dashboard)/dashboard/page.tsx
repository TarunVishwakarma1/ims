'use client'

import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { productsApi } from '@/lib/api/products'
import { categoriesApi } from '@/lib/api/categories'
import { ordersApi } from '@/lib/api/orders'
import { inventoryApi } from '@/lib/api/inventory'
import { storefrontApi } from '@/lib/api/storefront'
import { hasPermission, PERMISSIONS } from '@/lib/rbac'
import { useAuthStore } from '@/lib/stores/auth-store'
import { Package, Tags, ShoppingCart, AlertTriangle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function DashboardPage() {
  const { user } = useAuthStore()
  const canViewStorefront = hasPermission(user?.role, PERMISSIONS.STOREFRONT_VIEW)
  const { data: products } = useQuery({ queryKey: ['products'], queryFn: productsApi.list })
  const { data: categories } = useQuery({ queryKey: ['categories'], queryFn: categoriesApi.list })
  const { data: ordersResult } = useQuery({
    queryKey: ['orders', 'dashboard'],
    queryFn: () => ordersApi.list({ per_page: 1, page: 1 }),
  })
  const { data: inventory } = useQuery({ queryKey: ['inventory'], queryFn: inventoryApi.list })
  const { data: storefront } = useQuery({
    queryKey: ['storefront'],
    queryFn: storefrontApi.get,
    enabled: canViewStorefront,
  })

  const stats = [
    { title: 'Products', value: (products ?? []).length, icon: Package },
    { title: 'Categories', value: (categories ?? []).length, icon: Tags },
    { title: 'Orders', value: ordersResult?.total ?? 0, icon: ShoppingCart },
    { title: 'Low Stock', value: (inventory ?? []).filter(i => i.quantity <= i.low_stock_threshold).length, icon: AlertTriangle },
  ]

  return (
    <div className="space-y-6">
      {storefront === null && (
        <Link href="/storefront"
          className="block rounded-md border border-border bg-muted px-4 py-3 text-sm hover:bg-muted/70">
          Set up your storefront to start selling on Kirana →
        </Link>
      )}
      {storefront && !storefront.is_live && (
        <Link href="/storefront"
          className="block rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800 hover:bg-amber-100">
          Your storefront isn&apos;t live yet — finish setup to appear in Kirana →
        </Link>
      )}
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Overview</h2>
        <p className="text-muted-foreground">
          Your inventory status at a glance.
        </p>
      </div>
      
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat, i) => {
          const Icon = stat.icon
          return (
            <Card key={i}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">
                  {stat.title}
                </CardTitle>
                <Icon className={`h-4 w-4 ${stat.title === 'Low Stock' ? 'text-destructive' : 'text-muted-foreground'}`} />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stat.value}</div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </div>
  )
}
