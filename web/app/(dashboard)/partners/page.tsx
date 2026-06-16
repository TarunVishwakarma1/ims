'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { Loader2, ArrowDownLeft, ArrowUpRight } from 'lucide-react'

import { partnersApi, type PartnerRole } from '@/lib/api/partners'
import { formatPrice } from '@/lib/utils'

import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export default function PartnersPage() {
  const router = useRouter()
  const [role, setRole] = useState<PartnerRole>('supplier')

  const { data, isLoading } = useQuery({
    queryKey: ['partners', role],
    queryFn: () => partnersApi.list(role),
  })
  const partners = data ?? []

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Partners</h2>
        <p className="text-muted-foreground">
          Suppliers you&apos;ve bought from, and buyers who&apos;ve bought from you.
        </p>
      </div>

      <Tabs value={role} onValueChange={(v) => setRole(v as PartnerRole)}>
        <TabsList>
          <TabsTrigger value="supplier" className="gap-2">
            <ArrowDownLeft className="h-4 w-4" /> Suppliers
          </TabsTrigger>
          <TabsTrigger value="buyer" className="gap-2">
            <ArrowUpRight className="h-4 w-4" /> Buyers
          </TabsTrigger>
        </TabsList>

        <TabsContent value={role} className="mt-4">
          <div className="rounded-md border bg-white dark:bg-zinc-950">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Organization</TableHead>
                  <TableHead className="text-right">Orders</TableHead>
                  <TableHead className="text-right">Total value</TableHead>
                  <TableHead className="text-right">Active</TableHead>
                  <TableHead>Last order</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="h-24 text-center">
                      <Loader2 className="h-5 w-5 animate-spin text-muted-foreground inline" />
                    </TableCell>
                  </TableRow>
                ) : partners.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="h-24 text-center text-muted-foreground">
                      No {role}s yet.
                    </TableCell>
                  </TableRow>
                ) : (
                  partners.map((p) => (
                    <TableRow
                      key={p.org_id}
                      className="cursor-pointer hover:bg-muted/50"
                      onClick={() => router.push(`/partners/${p.org_id}`)}
                    >
                      <TableCell>
                        <div className="font-medium">{p.name}</div>
                        <div className="text-xs text-muted-foreground">{p.slug}</div>
                      </TableCell>
                      <TableCell className="text-right">{p.order_count}</TableCell>
                      <TableCell className="text-right font-medium">{formatPrice(p.total_value)}</TableCell>
                      <TableCell className="text-right">
                        {p.pending_count > 0
                          ? <Badge variant="outline" className="bg-amber-100 text-amber-800 border-amber-200">{p.pending_count}</Badge>
                          : <span className="text-muted-foreground text-xs">—</span>
                        }
                      </TableCell>
                      <TableCell>
                        {p.last_order_at ? new Date(p.last_order_at).toLocaleDateString() : '—'}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
