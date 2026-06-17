'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2, ChevronLeft, ChevronRight } from 'lucide-react'

import { auditApi } from '@/lib/api/audit'

import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export default function AuditPage() {
  const [entity, setEntity] = useState('')
  const [actionPrefix, setActionPrefix] = useState('')
  const [userID, setUserID] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(50)

  const filters = {
    entity: entity || undefined,
    action_prefix: actionPrefix || undefined,
    user_id: userID || undefined,
    from: from ? new Date(from + 'T00:00:00').toISOString() : undefined,
    to: to ? new Date(to + 'T23:59:59.999').toISOString() : undefined,
    page,
    per_page: perPage,
  }

  const { data, isLoading } = useQuery({
    queryKey: ['audit', filters],
    queryFn: () => auditApi.list(filters),
  })
  const items = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Audit log</h2>
        <p className="text-muted-foreground">
          Every state-changing action across the system. Filter by entity,
          action prefix, user, or date range for compliance reviews.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Select value={entity || 'all'} onValueChange={(v) => { setEntity(v === 'all' ? '' : v); setPage(1) }}>
          <SelectTrigger className="w-40"><SelectValue placeholder="Entity" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All entities</SelectItem>
            {['orders','inventory','products','categories','users','roles','locations'].map(e => (
              <SelectItem key={e} value={e}>{e}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          value={actionPrefix}
          onChange={(e) => { setActionPrefix(e.target.value); setPage(1) }}
          placeholder="Action prefix (e.g. order.)"
          className="w-56"
        />
        <Input
          value={userID}
          onChange={(e) => { setUserID(e.target.value); setPage(1) }}
          placeholder="User ID"
          className="w-72 font-mono text-xs"
        />
        <Input
          type="date"
          value={from}
          onChange={(e) => { setFrom(e.target.value); setPage(1) }}
          className="w-40"
          aria-label="From"
        />
        <span className="text-xs text-muted-foreground">→</span>
        <Input
          type="date"
          value={to}
          onChange={(e) => { setTo(e.target.value); setPage(1) }}
          className="w-40"
          aria-label="To"
        />
        {(entity || actionPrefix || userID || from || to) && (
          <Button variant="ghost" onClick={() => {
            setEntity(''); setActionPrefix(''); setUserID(''); setFrom(''); setTo(''); setPage(1)
          }}>Reset</Button>
        )}
        <span className="ml-auto text-sm text-muted-foreground">{total} entries</span>
      </div>

      <div className="rounded-md border bg-white dark:bg-zinc-950">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Timestamp</TableHead>
              <TableHead>Entity</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Entity ID</TableHead>
              <TableHead>User</TableHead>
              <TableHead>IP</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground inline" />
                </TableCell>
              </TableRow>
            ) : items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                  No entries match the filter.
                </TableCell>
              </TableRow>
            ) : (
              items.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="text-xs">{new Date(a.created_at).toLocaleString()}</TableCell>
                  <TableCell><span className="font-mono text-xs">{a.entity}</span></TableCell>
                  <TableCell className="font-mono text-xs">{a.action}</TableCell>
                  <TableCell className="font-mono text-xs">{a.entity_id.split('-')[0]}</TableCell>
                  <TableCell className="font-mono text-xs">{a.user_id ? a.user_id.split('-')[0] : '—'}</TableCell>
                  <TableCell className="font-mono text-xs">{a.ip_address || '—'}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Rows per page</span>
          <Select value={String(perPage)} onValueChange={(v) => { setPerPage(Number(v)); setPage(1) }}>
            <SelectTrigger className="w-20"><SelectValue /></SelectTrigger>
            <SelectContent>
              {[25, 50, 100, 200].map(n => <SelectItem key={n} value={String(n)}>{n}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Page {page} of {totalPages}</span>
          <Button variant="outline" size="icon" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <Button variant="outline" size="icon" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}
