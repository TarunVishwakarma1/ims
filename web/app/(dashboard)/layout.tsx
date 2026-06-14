import { Sidebar } from '@/components/layout/sidebar'
import { Header } from '@/components/layout/header'
import { ErrorBoundary } from '@/components/error-boundary'
import { VerifyEmailBanner } from '@/components/auth/verify-email-banner'

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="flex h-screen overflow-hidden bg-gray-50/50 dark:bg-zinc-900/50">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0">
        <Header />
        <main className="flex-1 overflow-auto p-6">
          <VerifyEmailBanner />
          <ErrorBoundary>{children}</ErrorBoundary>
        </main>
      </div>
    </div>
  )
}
