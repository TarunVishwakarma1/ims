import { describe, it, expect } from 'vitest'
import { canGoLive } from '@/lib/storefront-validation'

describe('canGoLive', () => {
  it('false when required fields missing', () => {
    expect(canGoLive({ display_name: '', lat: null, lng: null, pincodes: [] })).toBe(false)
    expect(canGoLive({ display_name: 'S', lat: 18.5, lng: 73.8, pincodes: [] })).toBe(false)
    expect(canGoLive({ display_name: 'S', lat: null, lng: null, pincodes: ['411001'] })).toBe(false)
  })
  it('true when name + location + pincode present', () => {
    expect(canGoLive({ display_name: 'S', lat: 18.5, lng: 73.8, pincodes: ['411001'] })).toBe(true)
  })
})
