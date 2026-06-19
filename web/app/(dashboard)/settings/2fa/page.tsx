'use client'

import { useEffect, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Loader2, Shield, Copy, CheckCircle2, AlertTriangle, ArrowLeft, Mail } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'
import Link from 'next/link'
import QRCode from 'qrcode'

import { totpApi } from '@/lib/api/totp'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/lib/stores/auth-store'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'

export default function TwoFactorPage() {
  const { user, setUser } = useAuthStore()

  // Refresh user state from the server on mount. localStorage-persisted
  // user might be stale (totp_enabled flipped on another tab / device).
  // The result rehydrates the store so view logic reflects reality.
  useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const fresh = await authApi.me()
      if (fresh) setUser(fresh)
      return fresh
    },
    refetchOnWindowFocus: true,
    staleTime: 30_000,
  })

  // Local UI state for the enrollment flow. Backed by server state via
  // user.totp_enabled — the canonical "is 2FA on" comes from the store
  // (mirrors /me). Step is derived, not stored, so navigating away and
  // back never loses the right view.
  const [enrollment, setEnrollment] = useState<{ uri: string; secret: string } | null>(null)
  const [qrDataUrl, setQrDataUrl] = useState<string>('')
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])

  const enabled = !!user?.totp_enabled
  const emailEnabled = !!user?.email_2fa_enabled

  const emailMut = useMutation({
    mutationFn: async () => emailEnabled ? totpApi.disableEmail() : totpApi.enableEmail(),
    onSuccess: () => {
      if (user) setUser({ ...user, email_2fa_enabled: !emailEnabled })
      toast.success(emailEnabled ? 'Email 2FA disabled' : 'Email 2FA enabled')
    },
    onError: () => toast.error('Could not update email 2FA'),
  })
  // Three view states: 'on' (enabled at server), 'enrolling' (mid-flow with
  // a generated QR), 'off' (not enabled, no pending enrollment in memory).
  const view: 'on' | 'enrolling' | 'off' =
    enabled ? 'on' : enrollment ? 'enrolling' : 'off'

  useEffect(() => {
    if (!enrollment) {
      setQrDataUrl('')
      return
    }
    QRCode.toDataURL(enrollment.uri, { width: 220, margin: 1 })
      .then(setQrDataUrl)
      .catch(() => toast.error('QR render failed'))
  }, [enrollment])

  const enrollMut = useMutation({
    mutationFn: () => totpApi.enroll(),
    onSuccess: (data) => {
      setEnrollment(data)
      setCode('')
    },
    onError: () => toast.error('Could not start enrollment'),
  })

  const confirmMut = useMutation({
    mutationFn: () => totpApi.confirm(code),
    onSuccess: (data) => {
      setBackupCodes(data.backup_codes)
      // Mirror enabled=true into the store so revisiting the page (or any
      // sidebar logic that checks user.totp_enabled) reflects reality.
      if (user) setUser({ ...user, totp_enabled: true })
      setEnrollment(null)
      setCode('')
      toast.success('Two-factor authentication enabled')
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({} as { error?: string }))
        toast.error(data.error || 'Invalid code')
      } else toast.error(err.message)
    },
  })

  const disableMut = useMutation({
    mutationFn: () => totpApi.disable(),
    onSuccess: () => {
      if (user) setUser({ ...user, totp_enabled: false })
      setEnrollment(null)
      setBackupCodes([])
      toast.success('2FA disabled')
    },
    onError: () => toast.error('Disable failed'),
  })

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <Link
          href="/settings"
          className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-2"
        >
          <ArrowLeft className="mr-1.5 h-3.5 w-3.5" /> Settings
        </Link>
        <h2 className="text-2xl font-bold tracking-tight">Two-factor authentication</h2>
        <p className="text-muted-foreground">
          Add a second login step beyond your password. Use any authenticator
          app (Authy, Google Authenticator, 1Password).
        </p>
      </div>

      {/* Methods overview — TOTP active, Email reserved for future. */}
      <div className="grid gap-3 sm:grid-cols-2">
        <Card className={view === 'on' ? 'border-emerald-300 dark:border-emerald-800' : ''}>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base flex items-center gap-2">
                <Shield className="h-4 w-4" /> Authenticator app
              </CardTitle>
              {view === 'on'
                ? <Badge className="bg-emerald-600 text-white">Enabled</Badge>
                : <Badge variant="outline">Disabled</Badge>}
            </div>
            <CardDescription className="text-xs">
              Time-based 6-digit codes via TOTP.
            </CardDescription>
          </CardHeader>
        </Card>
        <Card className={emailEnabled ? 'border-emerald-300 dark:border-emerald-800' : ''}>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base flex items-center gap-2">
                <Mail className="h-4 w-4" /> Email code
              </CardTitle>
              {emailEnabled
                ? <Badge className="bg-emerald-600 text-white">Enabled</Badge>
                : <Badge variant="outline">Disabled</Badge>}
            </div>
            <CardDescription className="text-xs">
              Single-use OTP to {user?.email} at every sign-in.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            <Button
              variant={emailEnabled ? 'destructive' : 'default'}
              size="sm"
              onClick={() => emailMut.mutate()}
              disabled={emailMut.isPending}
            >
              {emailMut.isPending && <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />}
              {emailEnabled ? 'Disable' : 'Enable'}
            </Button>
          </CardContent>
        </Card>
      </div>

      {view === 'off' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5" /> Set up authenticator app
            </CardTitle>
            <CardDescription>
              Every sign-in will ask for a 6-digit code in addition to your password.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => enrollMut.mutate()} disabled={enrollMut.isPending}>
              {enrollMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Enable
            </Button>
          </CardContent>
        </Card>
      )}

      {view === 'enrolling' && enrollment && (
        <Card>
          <CardHeader>
            <CardTitle>Scan the QR code</CardTitle>
            <CardDescription>
              The entry will show <span className="font-mono">IMS:{user?.email}</span>.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex justify-center bg-white p-4 rounded-md min-h-[252px] items-center">
              {qrDataUrl
                ? <img src={qrDataUrl} alt="TOTP QR code" width={220} height={220} />
                : <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />}
            </div>
            <div className="flex items-center gap-2 text-xs flex-wrap">
              <span className="text-muted-foreground">Manual secret:</span>
              <code className="font-mono bg-muted px-2 py-1 rounded break-all">{enrollment.secret}</code>
              <Button variant="ghost" size="icon" onClick={() => {
                navigator.clipboard.writeText(enrollment.secret); toast.success('Copied')
              }}>
                <Copy className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="space-y-2">
              <Label htmlFor="totp-code">Code from your authenticator</Label>
              <Input
                id="totp-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="123456"
              />
            </div>
            <div className="flex gap-2">
              <Button
                onClick={() => confirmMut.mutate()}
                disabled={code.length !== 6 || confirmMut.isPending}
              >
                {confirmMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Confirm
              </Button>
              <Button variant="ghost" onClick={() => { setEnrollment(null); setCode('') }}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {view === 'on' && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-emerald-700 dark:text-emerald-400">
                <CheckCircle2 className="h-5 w-5" /> Authenticator app active
              </CardTitle>
              <CardDescription>
                Every sign-in now requires a code from your authenticator app.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <Button variant="destructive" onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>
                {disableMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Disable 2FA
              </Button>
            </CardContent>
          </Card>

          {backupCodes.length > 0 && (
            <Card className="border-amber-200 bg-amber-50 dark:bg-amber-950/20">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-amber-800 dark:text-amber-300">
                  <AlertTriangle className="h-5 w-5" /> Save your backup codes
                </CardTitle>
                <CardDescription>
                  Each code is single-use. Store them somewhere safe — they will
                  NOT be shown again. If you lose them, contact an admin.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid grid-cols-2 gap-2 font-mono text-sm">
                  {backupCodes.map(c => (
                    <code key={c} className="bg-white dark:bg-zinc-900 px-2 py-1 rounded border">{c}</code>
                  ))}
                </div>
                <Button
                  variant="outline"
                  onClick={() => {
                    navigator.clipboard.writeText(backupCodes.join('\n'))
                    toast.success('Copied to clipboard')
                  }}
                >
                  <Copy className="mr-2 h-4 w-4" /> Copy all
                </Button>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
