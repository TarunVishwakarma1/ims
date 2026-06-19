'use client'

import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { MailWarning, Loader2, RefreshCcw } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { useAuthStore } from '@/lib/stores/auth-store'
import { authApi } from '@/lib/api/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// Dashboard layout re-mounts this banner on every route change. Persist
// "input is open" + the in-progress OTP in localStorage so navigating
// between tabs doesn't nuke either.
const OPEN_KEY = 'ims:verify-email-input-open'
const OTP_KEY  = 'ims:verify-email-input-otp'

export function VerifyEmailBanner() {
  const { user, setUser } = useAuthStore()
  const [showInput, setShowInput] = useState(false)
  const [otp, setOtp] = useState('')
  const [hydrated, setHydrated] = useState(false)

  // Hydrate from localStorage after mount — avoids SSR/CSR mismatch.
  // Default to input-visible: the signup endpoint already auto-sends the
  // first OTP, so the user should be ready to paste it without clicking
  // "Send code" first (which would queue a duplicate email). The Resend
  // button stays available inside the input row for explicit retries.
  useEffect(() => {
    if (typeof window === 'undefined') return
    const stored = window.localStorage.getItem(OPEN_KEY)
    setShowInput(stored === null ? true : stored === '1')
    setOtp(window.localStorage.getItem(OTP_KEY) ?? '')
    setHydrated(true)
  }, [])

  useEffect(() => {
    if (!hydrated || typeof window === 'undefined') return
    if (showInput) window.localStorage.setItem(OPEN_KEY, '1')
    else           window.localStorage.removeItem(OPEN_KEY)
  }, [showInput, hydrated])

  useEffect(() => {
    if (!hydrated || typeof window === 'undefined') return
    if (otp) window.localStorage.setItem(OTP_KEY, otp)
    else     window.localStorage.removeItem(OTP_KEY)
  }, [otp, hydrated])

  const clearAll = () => {
    setShowInput(false)
    setOtp('')
  }

  const resendMutation = useMutation({
    mutationFn: authApi.resendVerification,
    onSuccess: () => {
      toast.success('Verification code sent — check your inbox')
      setShowInput(true)
    },
    onError: () => toast.error('Could not send verification code'),
  })

  const verifyMutation = useMutation({
    mutationFn: authApi.verifyEmail,
    onSuccess: () => {
      toast.success('Email verified')
      if (user) setUser({ ...user, email_verified: true })
      clearAll()
    },
    onError: async (err: Error) => {
      if (err instanceof HTTPError) {
        try {
          const data = (await err.response.json()) as { error?: string }
          toast.error(data.error || 'Verification failed')
        } catch {
          toast.error('Verification failed')
        }
      } else {
        toast.error(err.message || 'Verification failed')
      }
    },
  })

  if (!user || user.email_verified) return null

  return (
    <div className="rounded-md border border-amber-500/30 bg-amber-50/60 dark:bg-amber-950/20 p-3 flex flex-wrap items-center justify-between gap-3 mb-6">
      <div className="flex items-center gap-2 text-sm text-amber-900 dark:text-amber-200">
        <MailWarning className="h-4 w-4" />
        <span>Verify your email <strong>{user.email}</strong> to keep your account secure.</span>
      </div>
      {showInput ? (
        <div className="flex items-center gap-2 flex-wrap">
          <Input
            value={otp}
            onChange={e => setOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
            placeholder="6-digit code"
            className="w-32"
            inputMode="numeric"
            maxLength={6}
            autoComplete="one-time-code"
          />
          <Button
            size="sm"
            disabled={otp.length !== 6 || verifyMutation.isPending}
            onClick={() => verifyMutation.mutate(otp)}
          >
            {verifyMutation.isPending && <Loader2 className="mr-2 h-3 w-3 animate-spin" />}
            Verify
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={resendMutation.isPending}
            onClick={() => { setOtp(''); resendMutation.mutate() }}
            title="Send a new code"
          >
            {resendMutation.isPending
              ? <Loader2 className="mr-2 h-3 w-3 animate-spin" />
              : <RefreshCcw className="mr-2 h-3 w-3" />}
            Resend
          </Button>
          <Button size="sm" variant="ghost" onClick={clearAll}>
            Close
          </Button>
        </div>
      ) : (
        <Button
          size="sm"
          variant="outline"
          onClick={() => resendMutation.mutate()}
          disabled={resendMutation.isPending}
        >
          {resendMutation.isPending && <Loader2 className="mr-2 h-3 w-3 animate-spin" />}
          Send code
        </Button>
      )}
    </div>
  )
}
