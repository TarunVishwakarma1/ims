'use client'

import { useQuery } from '@tanstack/react-query'
import { productsApi } from '@/lib/api/products'
import { categoriesApi } from '@/lib/api/categories'
import { ordersApi } from '@/lib/api/orders'
import { inventoryApi } from '@/lib/api/inventory'
import { Package, Tags, ShoppingCart, AlertTriangle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function DashboardPage() {
  const { data: products } = useQuery({ queryKey: ['products'], queryFn: productsApi.list })
  const { data: categories } = useQuery({ queryKey: ['categories'], queryFn: categoriesApi.list })
  const { data: orders } = useQuery({ queryKey: ['orders'], queryFn: ordersApi.list })
  const { data: inventory } = useQuery({ queryKey: ['inventory'], queryFn: inventoryApi.list })

  const stats = [
    { title: 'Products', value: (products ?? []).length, icon: Package },
    { title: 'Categories', value: (categories ?? []).length, icon: Tags },
    { title: 'Orders', value: (orders ?? []).length, icon: ShoppingCart },
    { title: 'Low Stock', value: (inventory ?? []).filter(i => i.quantity <= i.low_stock_threshold).length, icon: AlertTriangle },
  ]

  return (
    <div className="space-y-6">
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
