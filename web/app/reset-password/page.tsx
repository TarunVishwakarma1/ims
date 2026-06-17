'use client'

import { Suspense, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { Eye, EyeOff, Loader2, Package, CheckCircle2, ArrowLeft } from 'lucide-react'
import { HTTPError } from 'ky'

import { authApi } from '@/lib/api/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

function ResetPasswordInner() {
  const router = useRouter()
  const params = useSearchParams()
  const token = params.get('token') ?? ''

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)

  const valid =
    token.length > 0 &&
    password.length >= 8 &&
    password === confirm

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!valid) return
    setLoading(true)
    try {
      await authApi.confirmPasswordReset(token, password)
      setDone(true)
      setTimeout(() => router.push('/login'), 2000)
    } catch (err) {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({})) as { error?: string }
        setError(data.error || 'Reset link is invalid or has expired.')
      } else {
        setError('Something went wrong. Try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen bg-white dark:bg-zinc-950">
      <div className="hidden lg:flex lg:w-1/2 bg-gradient-to-br from-zinc-900 via-zinc-800 to-zinc-950 flex-col justify-between p-12 relative overflow-hidden">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]"></div>
        <div className="relative flex items-center gap-2">
          <Package className="h-8 w-8 text-white" />
          <span className="text-2xl font-bold text-white">IMS</span>
        </div>
        <div className="relative space-y-3">
          <h1 className="text-4xl font-bold text-white leading-tight">Set a new password</h1>
          <p className="text-zinc-400">Pick something strong — at least 8 characters.</p>
        </div>
        <p className="relative text-zinc-600 text-sm">© 2026 IMS. All rights reserved.</p>
      </div>

      <div className="w-full lg:w-1/2 flex items-center justify-center p-8 sm:p-12">
        <div className="w-full max-w-md space-y-6">
          <Link href="/login" className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to sign in
          </Link>

          {!token ? (
            <Alert variant="destructive">
              <AlertDescription>
                This page expects a reset token in the URL. Open the link from your email.
              </AlertDescription>
            </Alert>
          ) : done ? (
            <Alert className="bg-emerald-50 text-emerald-900 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-200 dark:border-emerald-900">
              <CheckCircle2 className="h-4 w-4" />
              <AlertDescription>
                Password updated. Redirecting to sign in…
              </AlertDescription>
            </Alert>
          ) : (
            <>
              <div className="space-y-2">
                <h2 className="text-3xl font-bold tracking-tight">Choose a new password</h2>
                <p className="text-muted-foreground">
                  This link expires in 1 hour and can be used once.
                </p>
              </div>

              <form onSubmit={submit} className="space-y-4">
                {error && (
                  <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
                )}
                <div className="space-y-2">
                  <Label htmlFor="password">New password</Label>
                  <div className="relative">
                    <Input
                      id="password"
                      type={showPassword ? 'text' : 'password'}
                      autoComplete="new-password"
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      className="pr-10"
                      disabled={loading}
                      minLength={8}
                      required
                      autoFocus
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 focus:outline-none"
                      tabIndex={-1}
                    >
                      {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confirm">Confirm new password</Label>
                  <Input
                    id="confirm"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="new-password"
                    value={confirm}
                    onChange={e => setConfirm(e.target.value)}
                    disabled={loading}
                    required
                  />
                  {confirm.length > 0 && password !== confirm && (
                    <p className="text-xs text-red-500">Passwords don't match.</p>
                  )}
                </div>
                <Button type="submit" className="w-full" disabled={!valid || loading}>
                  {loading ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Updating…</> : 'Update password'}
                </Button>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

export default function ResetPasswordPage() {
  // useSearchParams must be wrapped in Suspense in Next 15 client components.
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
      <ResetPasswordInner />
    </Suspense>
  )
}
