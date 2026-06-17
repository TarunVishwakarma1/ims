'use client'

import { useCallback, useEffect, useState } from 'react'

/**
 * Saved filter presets stored per-page in localStorage. Keyed by a stable
 * `pageKey` so /orders presets don't collide with /audit etc. Each preset
 * is a snapshot of the page's filter URL params.
 *
 * localStorage is intentional v1 — single device, no sync. DB-backed v2
 * can mirror this hook's surface so callers don't change.
 */
export interface FilterPreset {
  id: string
  name: string
  params: Record<string, string>
}

const storageKey = (pageKey: string) => `ims:presets:${pageKey}`

export function useFilterPresets(pageKey: string) {
  const [presets, setPresets] = useState<FilterPreset[]>([])

  // Load once on mount. Guard against SSR (window undefined).
  useEffect(() => {
    if (typeof window === 'undefined') return
    try {
      const raw = window.localStorage.getItem(storageKey(pageKey))
      if (raw) setPresets(JSON.parse(raw))
    } catch {
      // Corrupted JSON — drop the bucket so the user isn't stuck.
      window.localStorage.removeItem(storageKey(pageKey))
    }
  }, [pageKey])

  const persist = useCallback((next: FilterPreset[]) => {
    setPresets(next)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(storageKey(pageKey), JSON.stringify(next))
    }
  }, [pageKey])

  const save = useCallback((name: string, params: Record<string, string>) => {
    const id = crypto.randomUUID()
    persist([...presets, { id, name, params }])
  }, [presets, persist])

  const remove = useCallback((id: string) => {
    persist(presets.filter(p => p.id !== id))
  }, [presets, persist])

  return { presets, save, remove }
}
