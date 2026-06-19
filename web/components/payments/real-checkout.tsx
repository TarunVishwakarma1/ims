'use client'

import { useEffect, useRef } from 'react'
import { toast } from 'sonner'

import { useAuthStore } from '@/lib/stores/auth-store'

/**
 * Loads the RazorPay checkout.js script on demand and opens the widget.
 * Resolves with the payment_id on success, rejects on dismiss/failure.
 *
 * In production, RazorPay's webhook is what actually confirms the payment.
 * The browser handler here just gives the user immediate UI feedback —
 * trust always lives in the server-side webhook signature.
 */
declare global {
  interface Window {
    Razorpay: new (opts: Record<string, unknown>) => { open: () => void; close?: () => void }
  }
}

const RZP_SRC = 'https://checkout.razorpay.com/v1/checkout.js'

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve()
      return
    }
    const s = document.createElement('script')
    s.src = src
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Failed to load Razorpay'))
    document.body.appendChild(s)
  })
}

interface OpenRealCheckoutOpts {
  keyID: string
  razorpayOrderID: string
  amount: number // paise
  currency?: string
  prefill?: { name?: string; email?: string; contact?: string }
  notes?: Record<string, string>
  onSuccess: () => void
  onDismiss?: () => void
}

export async function openRealCheckout(opts: OpenRealCheckoutOpts) {
  try {
    await loadScript(RZP_SRC)
  } catch {
    toast.error('Could not load payment widget. Check your internet connection.')
    return
  }

  const rzp = new window.Razorpay({
    key: opts.keyID,
    amount: opts.amount,
    currency: opts.currency || 'INR',
    name: 'IMS',
    description: 'Order payment',
    order_id: opts.razorpayOrderID,
    prefill: opts.prefill || {},
    notes: opts.notes || {},
    theme: { color: '#10b981' },
    handler: () => {
      // Server-side webhook is the source of truth.
      // The handler firing means the browser saw a successful charge;
      // we still wait for the webhook before flipping the order to paid.
      opts.onSuccess()
    },
    modal: {
      ondismiss: () => {
        opts.onDismiss?.()
      },
    },
  })

  rzp.open()
}

/**
 * Hook that pre-loads checkout.js as soon as the user might need it.
 * Cuts time-to-open on first checkout from ~1s to instant.
 */
export function useRazorpayPreload(enabled = true) {
  const loaded = useRef(false)
  useEffect(() => {
    if (!enabled || loaded.current) return
    loaded.current = true
    loadScript(RZP_SRC).catch(() => {
      /* silent — will retry on actual checkout click */
    })
  }, [enabled])

  // Also surface auth state in case caller wants to skip when logged out
  const isAuth = useAuthStore((s) => s.isAuthenticated)
  return { isAuth }
}
