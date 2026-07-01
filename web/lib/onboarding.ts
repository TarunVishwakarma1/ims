// Seller onboarding checklist: the three steps a new shop completes before it
// sells on Kirana. Pure logic, kept out of the component so it's unit-testable.

export interface OnboardingState {
  hasStorefront: boolean
  hasProducts: boolean
  isLive: boolean
}

export interface OnboardingStep {
  key: 'storefront' | 'products' | 'golive'
  label: string
  href: string
  done: boolean
}

export function onboardingSteps(s: OnboardingState): OnboardingStep[] {
  return [
    { key: 'storefront', label: 'Set up your storefront', href: '/storefront', done: s.hasStorefront },
    { key: 'products', label: 'Add your first product', href: '/products', done: s.hasProducts },
    { key: 'golive', label: 'Go live in Kirana', href: '/storefront', done: s.isLive },
  ]
}

export const onboardingComplete = (s: OnboardingState): boolean =>
  onboardingSteps(s).every((step) => step.done)
