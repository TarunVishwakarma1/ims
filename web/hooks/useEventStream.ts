'use client'

import { useEffect, useRef } from 'react'

export type ServerEvent = {
  id: string
  type: string
  org_id?: string
  user_id?: string
  data?: unknown
  timestamp: string
}

type Handler = (evt: ServerEvent) => void

const API_URL = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, '') ?? ''

/**
 * useEventStream — subscribes to the backend SSE endpoint.
 *
 * Browser EventSource cannot set headers, so we authenticate via the
 * `ims_access_token` cookie (set in client.ts alongside localStorage).
 *
 * Auto-reconnects with EventSource's native exponential backoff. Re-fires the
 * handler with the new connection, no manual retry logic needed.
 */
export function useEventStream(topics: string[], onEvent: Handler, enabled = true) {
  const handlerRef = useRef(onEvent)
  handlerRef.current = onEvent

  useEffect(() => {
    if (!enabled || typeof window === 'undefined') return

    const params = new URLSearchParams()
    if (topics.length > 0) params.set('topics', topics.join(','))

    // withCredentials: true → browser sends ims_access_token cookie cross-origin
    const url = `${API_URL}/events${params.size > 0 ? '?' + params.toString() : ''}`
    const es = new EventSource(url, { withCredentials: true })

    const handleMessage = (e: MessageEvent) => {
      try {
        const evt = JSON.parse(e.data) as ServerEvent
        handlerRef.current(evt)
      } catch {
        // Ignore malformed payloads (heartbeats etc.)
      }
    }

    // We listen on every named event we care about. EventSource silently
    // ignores types we didn't register, so wildcard listeners aren't possible.
    // Solution: also listen on default `message` for fallback.
    es.addEventListener('message', handleMessage)
    es.addEventListener('ready', () => {})

    // Listen to each requested topic prefix explicitly. Backend sends
    // `event: <topic>` so we must subscribe to each topic name we know.
    const named = [
      'inventory.updated',
      'inventory.low_stock',
      'order.created',
      'order.status_changed',
      'product.created',
      'product.updated',
      'product.deleted',
      'listing.created',
      'listing.updated',
      'listing.deleted',
    ]
    for (const name of named) {
      es.addEventListener(name, handleMessage as EventListener)
    }

    es.onerror = () => {
      // EventSource auto-reconnects; we just observe.
      // eslint-disable-next-line no-console
      console.debug('SSE connection error, browser will retry')
    }

    return () => {
      es.close()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, topics.join(',')])
}
