import { Sidebar } from '@/components/layout/sidebar'
import { Header } from '@/components/layout/header'
import { ErrorBoundary } from '@/components/error-boundary'
import { VerifyEmailBanner } from '@/components/auth/verify-email-banner'
import { CartDrawer } from '@/components/cart/cart-drawer'

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="relative flex h-screen overflow-hidden bg-gradient-to-br from-zinc-50 via-white to-zinc-100 dark:from-zinc-950 dark:via-zinc-900 dark:to-zinc-950">
      {/* Decorative gradient blobs behind the floating sidebar — give the
          blurred glass surface something interesting to refract. */}
      <div className="pointer-events-none absolute -top-32 -left-24 h-96 w-96 rounded-full bg-indigo-300/40 blur-3xl dark:bg-indigo-600/20" />
      <div className="pointer-events-none absolute top-1/3 -left-10 h-72 w-72 rounded-full bg-pink-300/30 blur-3xl dark:bg-purple-600/20" />

      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0 pr-3 py-3">
        <Header />
        <main className="flex-1 overflow-auto p-6">
          <VerifyEmailBanner />
          <ErrorBoundary>{children}</ErrorBoundary>
        </main>
      </div>
      <CartDrawer />
    </div>
  )
}
