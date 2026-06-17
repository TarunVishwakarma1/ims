'use client'

import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Loader2, Shield, Copy, CheckCircle2, AlertTriangle } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'
import QRCode from 'qrcode'

import { totpApi } from '@/lib/api/totp'
import { useAuthStore } from '@/lib/stores/auth-store'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export default function TwoFactorPage() {
  const user = useAuthStore(s => s.user)
  const [step, setStep] = useState<'idle' | 'enrolling' | 'confirming' | 'done'>(
    user?.totp_enabled ? 'done' : 'idle'
  )
  const [enrollment, setEnrollment] = useState<{ uri: string; secret: string } | null>(null)
  const [qrDataUrl, setQrDataUrl] = useState<string>('')
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])

  // Render the otpauth URI to a data: URL locally. Avoids the api.qrserver.com
  // external dependency (which CSP img-src 'self' data: would also block).
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
      setStep('confirming')
    },
    onError: () => toast.error('Enroll failed'),
  })

  const confirmMut = useMutation({
    mutationFn: () => totpApi.confirm(code),
    onSuccess: (data) => {
      setBackupCodes(data.backup_codes)
      setStep('done')
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
      setStep('idle')
      setEnrollment(null)
      setBackupCodes([])
      toast.success('2FA disabled')
    },
    onError: () => toast.error('Disable failed'),
  })

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Two-factor authentication</h2>
        <p className="text-muted-foreground">
          Adds a TOTP code on top of your password. Use any authenticator app
          (Authy, Google Authenticator, 1Password, …).
        </p>
      </div>

      {step === 'idle' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2"><Shield className="h-5 w-5" /> Status: disabled</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Once enabled, every sign-in will ask for a code in addition to your password.
            </p>
            <Button onClick={() => enrollMut.mutate()} disabled={enrollMut.isPending}>
              {enrollMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Enroll
            </Button>
          </CardContent>
        </Card>
      )}

      {step === 'confirming' && enrollment && (
        <Card>
          <CardHeader>
            <CardTitle>Confirm enrollment</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm">
              Scan the QR code below with your authenticator app, then enter the
              6-digit code it shows to confirm.
            </p>
            <div className="flex justify-center bg-white p-4 rounded-md min-h-[252px] items-center">
              {qrDataUrl
                ? <img src={qrDataUrl} alt="TOTP QR code" width={220} height={220} />
                : <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />}
            </div>
            <div className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">Manual secret:</span>
              <code className="font-mono bg-muted px-2 py-1 rounded">{enrollment.secret}</code>
              <Button variant="ghost" size="icon" onClick={() => {
                navigator.clipboard.writeText(enrollment.secret); toast.success('Copied')
              }}>
                <Copy className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="space-y-2">
              <Label htmlFor="totp-code">Code from your app</Label>
              <Input
                id="totp-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="123456"
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={() => confirmMut.mutate()} disabled={!code || confirmMut.isPending}>
                {confirmMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Confirm
              </Button>
              <Button variant="ghost" onClick={() => { setStep('idle'); setEnrollment(null); setCode('') }}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'done' && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-emerald-700">
                <CheckCircle2 className="h-5 w-5" /> Status: enabled
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-muted-foreground">
                Every sign-in now asks for a TOTP code. Use a backup code if you
                lose access to your authenticator.
              </p>
              <Button variant="destructive" onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>
                {disableMut.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Disable 2FA
              </Button>
            </CardContent>
          </Card>

          {backupCodes.length > 0 && (
            <Card className="border-amber-200 bg-amber-50 dark:bg-amber-950/20">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-amber-800">
                  <AlertTriangle className="h-5 w-5" /> Save your backup codes
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm">
                  Each code is single-use. Store them somewhere safe — they will
                  NOT be shown again. If you lose them, contact an admin to reset
                  your 2FA.
                </p>
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
