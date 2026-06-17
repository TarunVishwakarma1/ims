'use client'

import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCcw, FileSearch } from 'lucide-react'

import { paymentsApi, type DLQEventFull } from '@/lib/api/payments'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
} from '@/components/ui/dialog'

type StatusFilter = 'all' | 'processed' | 'received' | 'failed'

function statusBadge(s: string) {
  const map: Record<string, string> = {
    processed: 'bg-emerald-600 text-white border-emerald-600',
    received: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    failed: 'bg-red-100 text-red-800 border-red-200',
  }
  return <Badge variant="outline" className={map[s] || ''}>{s}</Badge>
}

export default function WebhookEventsPage() {
  const qc = useQueryClient()
  const [tab, setTab] = useState<StatusFilter>('processed')
  const [viewingID, setViewingID] = useState<string | null>(null)

  const listQ = useQuery({
    queryKey: ['webhook-events', tab],
    queryFn: () => paymentsApi.listEvents(tab === 'all' ? undefined : tab),
  })
  const rows = listQ.data ?? []

  const detailQ = useQuery({
    queryKey: ['dlq-event', viewingID],
    queryFn: () => paymentsApi.getDLQEvent(viewingID!),
    enabled: !!viewingID,
  })
  const detail: DLQEventFull | undefined = detailQ.data

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Webhook events</h2>
          <p className="text-muted-foreground">
            Recent inbound webhook events. Use the Processed tab to confirm a
            specific RazorPay event landed; DLQ tab is at /webhooks/dlq.
          </p>
        </div>
        <Button variant="outline" onClick={() => qc.invalidateQueries({ queryKey: ['webhook-events'] })}>
          <RefreshCcw className="mr-2 h-4 w-4" /> Refresh
        </Button>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as StatusFilter)}>
        <TabsList>
          {(['processed','received','failed','all'] as StatusFilter[]).map(s => (
            <TabsTrigger key={s} value={s}>{s}</TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Provider</TableHead>
              <TableHead>Event type</TableHead>
              <TableHead>Event ID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Received</TableHead>
              <TableHead>Processed</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {listQ.isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="h-24 text-center">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground inline" />
                </TableCell>
              </TableRow>
            ) : rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="h-24 text-center text-muted-foreground">
                  No events in this state.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((e) => (
                <TableRow key={e.id}>
                  <TableCell><Badge variant="outline">{e.provider}</Badge></TableCell>
                  <TableCell className="font-mono text-xs">{e.event_type}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {e.event_id.split('_').pop()?.slice(0, 12) || e.event_id.slice(0, 12)}
                  </TableCell>
                  <TableCell>{statusBadge(e.status)}</TableCell>
                  <TableCell className="text-xs">{new Date(e.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-xs">
                    {e.processed_at ? new Date(e.processed_at).toLocaleString() : '—'}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" size="sm" onClick={() => setViewingID(e.id)}>
                      <FileSearch className="mr-2 h-3.5 w-3.5" /> View
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={!!viewingID} onOpenChange={(open) => { if (!open) setViewingID(null) }}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Webhook event payload</DialogTitle>
          </DialogHeader>
          {detailQ.isLoading ? (
            <div className="py-6 flex justify-center"><Loader2 className="h-5 w-5 animate-spin text-muted-foreground" /></div>
          ) : detail ? (
            <div className="space-y-3 text-sm">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="text-xs text-muted-foreground">Event ID</div>
                  <div className="font-mono break-all">{detail.event_id}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Type</div>
                  <div className="font-mono">{detail.event_type}</div>
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-1">Decrypted payload</div>
                <pre className="bg-muted rounded p-3 text-xs font-mono overflow-auto max-h-96">
                  {JSON.stringify(detail.payload, null, 2)}
                </pre>
              </div>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  )
}
