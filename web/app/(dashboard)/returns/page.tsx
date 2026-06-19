'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { Loader2, ArrowDownLeft, ArrowUpRight } from 'lucide-react'

import { returnsApi, type ReturnStatus } from '@/lib/api/returns'
import { formatPrice } from '@/lib/utils'

import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function statusBadge(s: ReturnStatus) {
  const map: Record<ReturnStatus, string> = {
    requested: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    approved: 'bg-blue-100 text-blue-800 border-blue-200',
    rejected: 'bg-red-100 text-red-800 border-red-200',
    in_transit: 'bg-purple-100 text-purple-800 border-purple-200',
    received: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    refunded: 'bg-emerald-600 text-white border-emerald-600',
  }
  return <Badge variant="outline" className={map[s] || ''}>{s.replace('_', ' ')}</Badge>
}

export default function ReturnsPage() {
  const router = useRouter()
  const [tab, setTab] = useState<'incoming' | 'outgoing'>('incoming')
  const { data, isLoading } = useQuery({
    queryKey: ['returns'],
    queryFn: returnsApi.listAll,
  })
  const all = data ?? []
  const incoming = all.filter(r => r.direction === 'incoming')
  const outgoing = all.filter(r => r.direction === 'outgoing' || !r.direction)
  const rows = tab === 'incoming' ? incoming : outgoing

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Returns</h2>
        <p className="text-muted-foreground">
          RMA requests in both directions. Approve incoming returns from buyers,
          track your own outgoing returns to suppliers.
        </p>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as 'incoming' | 'outgoing')}>
        <TabsList>
          <TabsTrigger value="incoming" className="gap-2">
            <ArrowDownLeft className="h-4 w-4" /> Incoming ({incoming.length})
          </TabsTrigger>
          <TabsTrigger value="outgoing" className="gap-2">
            <ArrowUpRight className="h-4 w-4" /> Outgoing ({outgoing.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value={tab} className="mt-4">
          <div className="rounded-md border bg-white dark:bg-zinc-950">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Return ID</TableHead>
                  <TableHead>Order</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead className="text-right">Refund</TableHead>
                  <TableHead>Requested</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center">
                      <Loader2 className="h-5 w-5 animate-spin text-muted-foreground inline" />
                    </TableCell>
                  </TableRow>
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                      {tab === 'incoming'
                        ? 'No incoming returns. Buyers have not requested any RMAs on your products.'
                        : 'No outgoing returns. You haven’t initiated any RMAs.'}
                    </TableCell>
                  </TableRow>
                ) : (
                  rows.map((r) => (
                    <TableRow
                      key={r.id}
                      className="cursor-pointer hover:bg-muted/50"
                      onClick={() => router.push(`/returns/${r.id}`)}
                    >
                      <TableCell className="font-mono text-xs">{r.id.split('-')[0]}</TableCell>
                      <TableCell className="font-mono text-xs">{r.order_id.split('-')[0]}</TableCell>
                      <TableCell>{statusBadge(r.status)}</TableCell>
                      <TableCell className="max-w-sm truncate text-sm">{r.reason}</TableCell>
                      <TableCell className="text-right font-medium">{formatPrice(r.refund_amount)}</TableCell>
                      <TableCell className="text-xs">{new Date(r.created_at).toLocaleDateString()}</TableCell>
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
