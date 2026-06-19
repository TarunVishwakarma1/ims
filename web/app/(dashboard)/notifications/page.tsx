'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCcw, AlertTriangle, FileSearch } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { notificationsApi, type Notification, type NotificationStatus } from '@/lib/api/notifications'

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

function statusBadge(s: NotificationStatus) {
  const map: Record<NotificationStatus, string> = {
    pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    sent: 'bg-emerald-600 text-white border-emerald-600',
    failed: 'bg-orange-100 text-orange-800 border-orange-200',
    dlq: 'bg-red-100 text-red-800 border-red-200',
  }
  return <Badge variant="outline" className={map[s]}>{s}</Badge>
}

export default function NotificationsPage() {
  const qc = useQueryClient()
  const [tab, setTab] = useState<NotificationStatus>('dlq')
  const [viewing, setViewing] = useState<Notification | null>(null)

  const listQ = useQuery({
    queryKey: ['notifications', tab],
    queryFn: () => notificationsApi.list(tab),
  })
  const rows = listQ.data ?? []

  const resendMut = useMutation({
    mutationFn: (id: string) => notificationsApi.resend(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notifications'] })
      toast.success('Queued for resend')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Resend failed')
      } else toast.error(err.message)
    },
  })

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Notifications</h2>
        <p className="text-muted-foreground">
          Outbound email queue. Tabs split pending / sent / failed / DLQ; resend
          a DLQ row after fixing the underlying issue.
        </p>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as NotificationStatus)}>
        <TabsList>
          {(['dlq','failed','pending','sent'] as NotificationStatus[]).map(s => (
            <TabsTrigger key={s} value={s}>{s}</TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Recipient</TableHead>
              <TableHead>Subject</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Attempts</TableHead>
              <TableHead>Error</TableHead>
              <TableHead>Created</TableHead>
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
                  No notifications in this state.
                </TableCell>
              </TableRow>
            ) : (
              rows.map((n) => (
                <TableRow key={n.id}>
                  <TableCell className="font-mono text-xs">{n.recipient}</TableCell>
                  <TableCell className="max-w-sm truncate text-sm">{n.subject}</TableCell>
                  <TableCell>{statusBadge(n.status)}</TableCell>
                  <TableCell className="text-right text-sm">{n.attempts}</TableCell>
                  <TableCell className="max-w-xs">
                    {n.last_error ? (
                      <div className="flex items-start gap-1 text-xs text-red-700">
                        <AlertTriangle className="h-3.5 w-3.5 mt-0.5 shrink-0" />
                        <span className="line-clamp-2">{n.last_error}</span>
                      </div>
                    ) : <span className="text-muted-foreground text-xs">—</span>}
                  </TableCell>
                  <TableCell className="text-xs">
                    {new Date(n.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right space-x-1">
                    <Button variant="outline" size="sm" onClick={() => setViewing(n)}>
                      <FileSearch className="mr-2 h-3.5 w-3.5" /> View
                    </Button>
                    {(n.status === 'dlq' || n.status === 'failed') && (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={resendMut.isPending}
                        onClick={() => resendMut.mutate(n.id)}
                      >
                        <RefreshCcw className="mr-2 h-3.5 w-3.5" /> Resend
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={!!viewing} onOpenChange={(open) => { if (!open) setViewing(null) }}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Notification body</DialogTitle>
          </DialogHeader>
          {viewing && (
            <div className="space-y-3 text-sm">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="text-xs text-muted-foreground">To</div>
                  <div className="font-mono break-all">{viewing.recipient}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Subject</div>
                  <div>{viewing.subject}</div>
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-1">Body</div>
                <pre className="bg-muted rounded p-3 text-xs whitespace-pre-wrap overflow-auto max-h-96">
                  {viewing.body_text}
                </pre>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
