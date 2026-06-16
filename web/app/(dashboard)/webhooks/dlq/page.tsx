'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCcw, AlertTriangle } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { paymentsApi } from '@/lib/api/payments'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export default function DLQPage() {
  const qc = useQueryClient()
  const [replayingID, setReplayingID] = useState<string | null>(null)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['dlq'],
    queryFn: paymentsApi.listDLQ,
  })
  const events = data ?? []

  const replayMut = useMutation({
    mutationFn: (id: string) => paymentsApi.replayDLQ(id),
    onMutate: (id) => setReplayingID(id),
    onSettled: () => setReplayingID(null),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['dlq'] })
      qc.invalidateQueries({ queryKey: ['orders'] })
      qc.invalidateQueries({ queryKey: ['payments'] })
      toast.success('Event replayed successfully')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Replay failed')
      } else toast.error(err.message)
    },
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Webhook DLQ</h2>
          <p className="text-muted-foreground">
            Webhook events that exhausted retries. Investigate, fix the underlying issue, then replay.
          </p>
        </div>
        <Button variant="outline" onClick={() => refetch()} disabled={isLoading}>
          <RefreshCcw className={`mr-2 h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Provider</TableHead>
              <TableHead>Event type</TableHead>
              <TableHead>Event ID</TableHead>
              <TableHead>Error</TableHead>
              <TableHead>Failed at</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground inline" />
                </TableCell>
              </TableRow>
            ) : events.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                  DLQ is empty — no failed webhook events.
                </TableCell>
              </TableRow>
            ) : (
              events.map((evt) => (
                <TableRow key={evt.id}>
                  <TableCell>
                    <Badge variant="outline">{evt.provider}</Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{evt.event_type}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {evt.event_id.split('_').pop()?.slice(0, 12) || evt.event_id.slice(0, 12)}
                  </TableCell>
                  <TableCell className="max-w-xs">
                    {evt.error ? (
                      <div className="flex items-start gap-1 text-xs text-red-700">
                        <AlertTriangle className="h-3.5 w-3.5 mt-0.5 shrink-0" />
                        <span className="line-clamp-2">{evt.error}</span>
                      </div>
                    ) : (
                      <span className="text-muted-foreground text-xs">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-xs">
                    {evt.processed_at ? new Date(evt.processed_at).toLocaleString() : '—'}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={replayingID === evt.id || replayMut.isPending}
                      onClick={() => replayMut.mutate(evt.id)}
                    >
                      {replayingID === evt.id
                        ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        : <RefreshCcw className="mr-2 h-3.5 w-3.5" />}
                      Replay
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
