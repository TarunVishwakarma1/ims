'use client'

import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useRouter } from 'next/navigation'
import { Eye, EyeOff, Loader2, Package, CheckCircle2 } from 'lucide-react'
import Link from 'next/link'

import { api } from '@/lib/api/client'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { LoginResponse, ApiError } from '@/types/api'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { HTTPError } from 'ky'

const loginSchema = z.object({
  email: z.string().email({ message: 'Invalid email address' }),
  password: z.string().min(1, { message: 'Password is required' }),
})

type LoginFormValues = z.infer<typeof loginSchema>

export default function LoginPage() {
  const router = useRouter()
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [registered, setRegistered] = useState(false)

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search)
      if (params.get('registered') === 'true') {
        setTimeout(() => setRegistered(true), 0)
        // Clean up URL without reloading
        window.history.replaceState({}, document.title, '/login')
      }
    }
  }, [])
  
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  // 2FA second-step state. Populated when /login returns require_totp.
  const [pendingToken, setPendingToken] = useState<string | null>(null)
  const [twoFAMethod, setTwoFAMethod] = useState<'totp' | 'email'>('totp')
  const [totpCode, setTotpCode] = useState('')
  const [verifying, setVerifying] = useState(false)
  const [resending, setResending] = useState(false)

  const onSubmit = async (data: LoginFormValues) => {
    setError(null)
    try {
      const response = await api.post('auth/login', { json: data }).json<LoginResponse>()
      if (response.require_totp && response.pending_token) {
        setPendingToken(response.pending_token)
        setTwoFAMethod(response.two_fa_method === 'email' ? 'email' : 'totp')
        return
      }
      useAuthStore.getState().login(response)
      router.push('/dashboard')
    } catch (err) {
      if (err instanceof HTTPError) {
        try {
          const errorData = await err.response.json() as ApiError
          setError(errorData.error || 'Login failed. Please check your credentials.')
        } catch {
          setError('An unexpected error occurred.')
        }
      } else {
        setError(err instanceof Error ? err.message : 'Network error. Please try again later.')
      }
    }
  }

  const verify2FA = async () => {
    if (!pendingToken) return
    setError(null)
    setVerifying(true)
    try {
      const response = await api.post('auth/login/verify-2fa', {
        json: { pending_token: pendingToken, code: totpCode },
      }).json<LoginResponse>()
      useAuthStore.getState().login(response)
      router.push('/dashboard')
    } catch (err) {
      if (err instanceof HTTPError) {
        const data = await err.response.json().catch(() => ({})) as ApiError
        setError(data.error || 'Invalid code')
      } else {
        setError(err instanceof Error ? err.message : 'Verification failed')
      }
    } finally {
      setVerifying(false)
    }
  }

  return (
    <div className="flex min-h-screen bg-white dark:bg-zinc-950">
      <div className="hidden lg:flex lg:w-1/2 bg-gradient-to-br from-zinc-900 via-zinc-800 to-zinc-950 flex-col justify-between p-12 relative overflow-hidden">
        {/* Subtle grid pattern background */}
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]"></div>
        
        <div className="relative flex items-center gap-2">
          <Package className="h-8 w-8 text-white" />
          <span className="text-2xl font-bold text-white">IMS</span>
        </div>
        
        <div className="relative space-y-6">
          <h1 className="text-4xl font-bold text-white leading-tight">
            Manage your inventory<br />with confidence
          </h1>
          <ul className="space-y-3 text-zinc-400">
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Real-time stock tracking</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Role-based access control</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Full audit logging</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Multi-user team support</li>
          </ul>
        </div>
        
        <p className="relative text-zinc-600 text-sm">© 2026 IMS. All rights reserved.</p>
      </div>

      <div className="w-full lg:w-1/2 flex items-center justify-center p-8 sm:p-12">
        <div className="w-full max-w-md space-y-8">
          <div className="space-y-2 text-center lg:text-left">
            <h2 className="text-3xl font-bold tracking-tight">Sign in</h2>
            <p className="text-muted-foreground">
              Enter your email and password to access your account
            </p>
          </div>

          {pendingToken ? (
            <div className="space-y-4">
              <div>
                <h2 className="text-2xl font-bold tracking-tight mb-1">
                  {twoFAMethod === 'email' ? 'Email verification' : 'Two-factor verification'}
                </h2>
                <p className="text-muted-foreground text-sm">
                  {twoFAMethod === 'email'
                    ? 'We sent a 6-digit code to your email. Enter it below to finish signing in.'
                    : 'Enter the 6-digit code from your authenticator app (or a backup code).'}
                </p>
              </div>
              {error && (
                <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
              )}
              <div className="space-y-2">
                <Label htmlFor="totp-code">Code</Label>
                <Input
                  id="totp-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  placeholder={twoFAMethod === 'email' ? '6-digit code' : '123456 or backup code'}
                  value={totpCode}
                  onChange={(e) => setTotpCode(e.target.value)}
                  disabled={verifying}
                  autoFocus
                />
              </div>
              <Button
                className="w-full"
                disabled={!totpCode || verifying}
                onClick={verify2FA}
              >
                {verifying ? (<><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Verifying…</>) : 'Verify'}
              </Button>
              {twoFAMethod === 'email' && (
                <Button
                  variant="outline"
                  className="w-full"
                  disabled={resending}
                  onClick={async () => {
                    if (!pendingToken) return
                    setResending(true)
                    try {
                      await api.post('auth/login/resend-2fa', { json: { pending_token: pendingToken } }).json()
                      setError(null)
                    } catch {
                      setError('Could not resend the code. Try again in a moment.')
                    } finally {
                      setResending(false)
                    }
                  }}
                >
                  {resending ? (<><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Sending…</>) : 'Resend code'}
                </Button>
              )}
              <Button
                variant="ghost"
                className="w-full"
                onClick={() => { setPendingToken(null); setTotpCode(''); setError(null) }}
              >
                Back
              </Button>
            </div>
          ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            {registered && (
              <Alert className="bg-emerald-50 text-emerald-900 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-200 dark:border-emerald-900">
                <AlertDescription>Account created successfully! Please sign in.</AlertDescription>
              </Alert>
            )}

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                placeholder="m@example.com"
                {...register('email')}
                className={errors.email ? 'border-red-500' : ''}
                disabled={isSubmitting}
              />
              {errors.email && (
                <p className="text-sm text-red-500">{errors.email.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="password">Password</Label>
                <Link href="/forgot-password" className="text-xs text-muted-foreground hover:text-foreground hover:underline">
                  Forgot password?
                </Link>
              </div>
              <div className="relative">
                <Input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  {...register('password')}
                  className={errors.password ? 'border-red-500 pr-10' : 'pr-10'}
                  disabled={isSubmitting}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 focus:outline-none"
                  tabIndex={-1}
                >
                  {showPassword ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                  <span className="sr-only">
                    {showPassword ? 'Hide password' : 'Show password'}
                  </span>
                </button>
              </div>
              {errors.password && (
                <p className="text-sm text-red-500">{errors.password.message}</p>
              )}
            </div>

            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Signing in...
                </>
              ) : (
                'Sign in'
              )}
            </Button>
          </form>
          )}

          <p className="text-center text-sm text-muted-foreground">
            Don&apos;t have an account?{' '}
            <Link href="/register" className="font-medium text-primary hover:underline">
              Register
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
