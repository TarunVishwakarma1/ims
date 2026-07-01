import { describe, it, expect } from 'vitest'
import { onboardingSteps, onboardingComplete } from './onboarding'

describe('onboardingSteps', () => {
  it('reflects each flag in the matching step', () => {
    const steps = onboardingSteps({ hasStorefront: true, hasProducts: false, isLive: false })
    expect(steps.map((s) => [s.key, s.done])).toEqual([
      ['storefront', true],
      ['products', false],
      ['golive', false],
    ])
  })
})

describe('onboardingComplete', () => {
  it('false until every step is done', () => {
    expect(onboardingComplete({ hasStorefront: true, hasProducts: true, isLive: false })).toBe(false)
    expect(onboardingComplete({ hasStorefront: false, hasProducts: false, isLive: false })).toBe(false)
  })
  it('true when storefront + products + live', () => {
    expect(onboardingComplete({ hasStorefront: true, hasProducts: true, isLive: true })).toBe(true)
  })
})
