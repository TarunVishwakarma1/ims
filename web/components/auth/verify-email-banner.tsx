'use client'

import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { MailWarning, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { HTTPError } from 'ky'

import { useAuthStore } from '@/lib/stores/auth-store'
import { authApi } from '@/lib/api/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export function VerifyEmailBanner() {
  const { user, setUser } = useAuthStore()
  const [showInput, setShowInput] = useState(false)
  const [otp, setOtp] = useState('')

  const resendMutation = useMutation({
    mutationFn: authApi.resendVerification,
    onSuccess: () => {
      toast.success('Verification code sent. Check the server log (dev mode).')
      setShowInput(true)
    },
    onError: () => toast.error('Could not send verification code'),
  })

  const verifyMutation = useMutation({
    mutationFn: authApi.verifyEmail,
    onSuccess: () => {
      toast.success('Email verified')
      if (user) setUser({ ...user, email_verified: true })
      setShowInput(false)
      setOtp('')
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
        <div className="flex items-center gap-2">
          <Input
            value={otp}
            onChange={e => setOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
            placeholder="6-digit code"
            className="w-32"
            inputMode="numeric"
            maxLength={6}
          />
          <Button
            size="sm"
            disabled={otp.length !== 6 || verifyMutation.isPending}
            onClick={() => verifyMutation.mutate(otp)}
          >
            {verifyMutation.isPending && <Loader2 className="mr-2 h-3 w-3 animate-spin" />}
            Verify
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setShowInput(false)}>
            Cancel
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
