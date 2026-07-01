import { describe, it, expect } from 'vitest'
import { mapNominatim } from './geocode'

describe('mapNominatim', () => {
  it('maps address fields', () => {
    const res = mapNominatim({
      address: {
        suburb: 'Koregaon Park',
        city: 'Pune',
        postcode: '411001',
      },
    })
    expect(res).toEqual({ area: 'Koregaon Park', city: 'Pune', pincode: '411001' })
  })

  it('falls back across area/city keys', () => {
    const res = mapNominatim({
      address: { neighbourhood: 'Baner', town: 'Pune', postcode: '411045' },
    })
    expect(res).toEqual({ area: 'Baner', city: 'Pune', pincode: '411045' })
  })

  it('tolerates missing fields', () => {
    expect(mapNominatim({})).toEqual({ area: '', city: '', pincode: '' })
  })
})
