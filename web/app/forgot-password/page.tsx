'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Loader2, Package, CheckCircle2, ArrowLeft } from 'lucide-react'

import { authApi } from '@/lib/api/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await authApi.requestPasswordReset(email)
      setSubmitted(true)
    } catch {
      setError('Something went wrong. Try again in a moment.')
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
          <h1 className="text-4xl font-bold text-white leading-tight">Reset your password</h1>
          <p className="text-zinc-400">We'll email you a secure link to set a new one.</p>
        </div>
        <p className="relative text-zinc-600 text-sm">© 2026 IMS. All rights reserved.</p>
      </div>

      <div className="w-full lg:w-1/2 flex items-center justify-center p-8 sm:p-12">
        <div className="w-full max-w-md space-y-6">
          <Link href="/login" className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="mr-2 h-4 w-4" /> Back to sign in
          </Link>

          <div className="space-y-2">
            <h2 className="text-3xl font-bold tracking-tight">Forgot your password?</h2>
            <p className="text-muted-foreground">
              Enter your email and we'll send a link to reset it.
            </p>
          </div>

          {submitted ? (
            <Alert className="bg-emerald-50 text-emerald-900 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-200 dark:border-emerald-900">
              <CheckCircle2 className="h-4 w-4" />
              <AlertDescription>
                If <strong>{email}</strong> is registered, we've sent a reset link.
                Check your inbox (and spam folder) in the next minute or two.
              </AlertDescription>
            </Alert>
          ) : (
            <form onSubmit={submit} className="space-y-4">
              {error && (
                <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
              )}
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  required
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  disabled={loading}
                  autoFocus
                />
              </div>
              <Button type="submit" className="w-full" disabled={loading || !email}>
                {loading ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Sending…</> : 'Send reset link'}
              </Button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
