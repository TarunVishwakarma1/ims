'use client'

import Link from 'next/link'
import { CheckCircle2, Circle } from 'lucide-react'
import { onboardingSteps, type OnboardingState } from '@/lib/onboarding'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

export function OnboardingChecklist({ state }: { state: OnboardingState }) {
  const steps = onboardingSteps(state)
  const done = steps.filter((s) => s.done).length

  return (
    <Card>
      <CardHeader>
        <CardTitle>Get your shop live on Kirana</CardTitle>
        <CardDescription>{done} of {steps.length} done</CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        {steps.map((s) => (
          <Link key={s.key} href={s.href}
            className="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted">
            {s.done
              ? <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />
              : <Circle className="h-4 w-4 shrink-0 text-muted-foreground" />}
            <span className={s.done ? 'text-muted-foreground line-through' : ''}>{s.label}</span>
          </Link>
        ))}
      </CardContent>
    </Card>
  )
}
