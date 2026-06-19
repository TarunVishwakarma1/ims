'use client'

import Link from 'next/link'
import { useAuthStore } from '@/lib/stores/auth-store'
import {
  User as UserIcon,
  ShieldCheck,
  Mail,
  Building2,
  ChevronRight,
  KeyRound,
  type LucideIcon,
} from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'

interface SettingRow {
  href: string
  icon: LucideIcon
  title: string
  description: string
  badge?: { label: string; tone: 'ok' | 'warn' | 'muted' }
  disabled?: boolean
}

export default function SettingsPage() {
  const { user, organization } = useAuthStore()

  const initials = (user?.name ?? user?.email ?? '?')
    .split(' ')
    .map(s => s[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()

  const securityRows: SettingRow[] = [
    {
      href: '/settings/2fa',
      icon: ShieldCheck,
      title: 'Two-factor authentication',
      description: 'Add a TOTP code on top of your password.',
      badge: user?.totp_enabled
        ? { label: 'Enabled', tone: 'ok' }
        : { label: 'Disabled', tone: 'warn' },
    },
    {
      href: '/forgot-password',
      icon: KeyRound,
      title: 'Change password',
      description: 'Send yourself a password reset link.',
    },
  ]

  const accountRows: SettingRow[] = [
    {
      href: '#',
      icon: UserIcon,
      title: 'Profile',
      description: 'Name, profile picture, and timezone.',
      badge: { label: 'Coming soon', tone: 'muted' },
      disabled: true,
    },
    {
      href: '#',
      icon: Mail,
      title: 'Email preferences',
      description: 'Which order / inventory events you want notified about.',
      badge: { label: 'Coming soon', tone: 'muted' },
      disabled: true,
    },
  ]

  return (
    <div className="space-y-8 max-w-3xl">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
        <p className="text-muted-foreground">Manage your account, security, and preferences.</p>
      </div>

      {/* Identity card */}
      <Card>
        <CardContent className="flex items-center gap-4 p-6">
          <Avatar className="h-14 w-14">
            <AvatarFallback className="bg-gradient-to-br from-indigo-500 to-purple-600 text-base font-semibold text-white">
              {initials}
            </AvatarFallback>
          </Avatar>
          <div className="flex-1 min-w-0">
            <div className="text-lg font-semibold truncate">{user?.name ?? 'User'}</div>
            <div className="text-sm text-muted-foreground truncate">{user?.email}</div>
            <div className="mt-2 flex flex-wrap gap-2 text-xs">
              {user?.role && (
                <Badge variant="outline" className="capitalize">
                  {user.role}
                </Badge>
              )}
              {organization && (
                <Badge variant="outline" className="font-normal">
                  <Building2 className="h-3 w-3 mr-1" /> {organization.name}
                </Badge>
              )}
              {user?.email_verified
                ? <Badge variant="outline" className="bg-emerald-50 text-emerald-700 border-emerald-200">Email verified</Badge>
                : <Badge variant="outline" className="bg-amber-50 text-amber-800 border-amber-200">Email unverified</Badge>}
            </div>
          </div>
        </CardContent>
      </Card>

      <Section title="Security">
        {securityRows.map(r => <Row key={r.title} row={r} />)}
      </Section>

      <Section title="Account">
        {accountRows.map(r => <Row key={r.title} row={r} />)}
      </Section>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground px-1">
        {title}
      </h3>
      <Card>
        <div className="divide-y divide-border">{children}</div>
      </Card>
    </div>
  )
}

function Row({ row }: { row: SettingRow }) {
  const Icon = row.icon
  const tone = row.badge?.tone
  const badgeClass =
    tone === 'ok'
      ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
      : tone === 'warn'
        ? 'bg-amber-50 text-amber-800 border-amber-200'
        : ''

  const body = (
    <div className="flex items-center gap-4 px-6 py-4">
      <Icon className="h-5 w-5 text-muted-foreground shrink-0" />
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium">{row.title}</div>
        <div className="text-xs text-muted-foreground">{row.description}</div>
      </div>
      {row.badge && (
        <Badge variant="outline" className={badgeClass}>{row.badge.label}</Badge>
      )}
      {!row.disabled && <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />}
    </div>
  )

  if (row.disabled) {
    return <div className="opacity-60 cursor-not-allowed">{body}</div>
  }
  return (
    <Link href={row.href} className="block hover:bg-muted/50 transition-colors">
      {body}
    </Link>
  )
}
