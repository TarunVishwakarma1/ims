import Link from 'next/link'
import {
  Package,
  BarChart3,
  Shield,
  Users,
  ClipboardList,
  Zap,
  ArrowRight,
  CheckCircle2,
  Lock,
  ChevronRight,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const features = [
  {
    icon: BarChart3,
    title: 'Real-time Inventory',
    description: 'Track stock levels across all products instantly. Know exactly what you have, what is running low, and what needs restocking.',
    accent: 'text-emerald-400',
    bg: 'bg-emerald-500/10',
    glow: 'group-hover:border-emerald-800',
  },
  {
    icon: Shield,
    title: 'Role-based Access',
    description: 'Granular permission system with dynamic roles. Admins, managers, staff — each gets exactly the access they need, nothing more.',
    accent: 'text-violet-400',
    bg: 'bg-violet-500/10',
    glow: 'group-hover:border-violet-800',
  },
  {
    icon: Users,
    title: 'Multi-user Teams',
    description: 'Invite your whole team. Create custom roles on the fly without deploying new code. Scale from 2 to 2000 users seamlessly.',
    accent: 'text-blue-400',
    bg: 'bg-blue-500/10',
    glow: 'group-hover:border-blue-800',
  },
  {
    icon: ClipboardList,
    title: 'Order Management',
    description: 'Create, track and manage orders end-to-end. Automatic stock decrement on confirmation with full transaction safety.',
    accent: 'text-orange-400',
    bg: 'bg-orange-500/10',
    glow: 'group-hover:border-orange-800',
  },
  {
    icon: Lock,
    title: 'Full Audit Log',
    description: 'Every action is recorded. Who did what, when, and from where. Complete traceability for compliance and debugging.',
    accent: 'text-rose-400',
    bg: 'bg-rose-500/10',
    glow: 'group-hover:border-rose-800',
  },
  {
    icon: Zap,
    title: 'Blazing Fast API',
    description: 'Built on Go with pgx and connection pooling. Sub-millisecond DB queries. 6.8MB Docker image. Handles thousands of requests per second.',
    accent: 'text-yellow-400',
    bg: 'bg-yellow-500/10',
    glow: 'group-hover:border-yellow-800',
  },
]

const steps = [
  {
    number: '01',
    title: 'Create your account',
    description: 'Register in seconds. Set up your admin profile and invite your team members with exactly the roles they need.',
  },
  {
    number: '02',
    title: 'Add your products',
    description: 'Create categories, add products with SKUs and pricing. Inventory records are created automatically on product creation.',
  },
  {
    number: '03',
    title: 'Manage with confidence',
    description: 'Track stock, process orders, monitor low stock alerts, and keep your entire team in sync — all from one dashboard.',
  },
]

const stats = [
  { value: '< 7ms', label: 'Avg API response' },
  { value: '6.8MB', label: 'Docker image size' },
  { value: '100%', label: 'Type-safe stack' },
  { value: '∞', label: 'Custom roles' },
]

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-zinc-950 text-white overflow-x-hidden">

      {/* ── Navbar ── */}
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-md">
        <div className="mx-auto max-w-7xl px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500">
              <Package className="h-5 w-5 text-white" />
            </div>
            <span className="text-lg font-bold tracking-tight">IMS</span>
          </div>

          <div className="hidden md:flex items-center gap-8 text-sm text-zinc-400">
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <a href="#how-it-works" className="hover:text-white transition-colors">How it works</a>
            <a href="#roles" className="hover:text-white transition-colors">Roles</a>
          </div>

          <div className="flex items-center gap-3">
            <Link href="/login">
              <Button variant="ghost" size="sm" className="text-zinc-300 hover:text-white hover:bg-zinc-800">
                Sign in
              </Button>
            </Link>
            <Link href="/register">
              <Button size="sm" className="bg-emerald-500 hover:bg-emerald-400 text-white font-semibold shadow-lg shadow-emerald-500/20">
                Get started
              </Button>
            </Link>
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="relative pt-36 pb-28 px-6 overflow-hidden">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#ffffff07_1px,transparent_1px),linear-gradient(to_bottom,#ffffff07_1px,transparent_1px)] bg-[size:40px_40px]" />
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[900px] h-[600px] bg-emerald-500/8 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute top-24 right-1/4 w-[500px] h-[400px] bg-violet-500/6 rounded-full blur-3xl pointer-events-none" />

        <div className="relative mx-auto max-w-5xl text-center">
          <Badge className="mb-6 bg-emerald-500/10 text-emerald-400 border-emerald-500/20 hover:bg-emerald-500/10 px-4 py-1.5 text-sm font-medium">
            Open source · Self-hostable · Production ready
          </Badge>

          <h1 className="text-5xl sm:text-6xl lg:text-7xl font-bold tracking-tight leading-[1.1] mb-6">
            Inventory management
            <br />
            <span className="bg-gradient-to-r from-emerald-400 via-teal-400 to-violet-400 bg-clip-text text-transparent">
              built for teams
            </span>
          </h1>

          <p className="text-xl text-zinc-400 max-w-2xl mx-auto leading-relaxed mb-10">
            A secure, fast, and fully role-based inventory system. Manage products, orders, stock levels, and your entire team — all in one place, self-hosted on your own infrastructure.
          </p>

          <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
            <Link href="/register">
              <Button size="lg" className="bg-emerald-500 hover:bg-emerald-400 text-white px-8 gap-2 text-base font-semibold shadow-xl shadow-emerald-500/20 transition-all hover:scale-105">
                Start for free <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
            <Link href="/login">
              <Button size="lg" variant="outline" className="border-zinc-700 text-zinc-300 hover:bg-zinc-800 hover:text-white hover:border-zinc-600 px-8 text-base transition-all">
                Sign in to dashboard
              </Button>
            </Link>
          </div>

          <div className="mt-8 flex flex-wrap items-center justify-center gap-6 text-sm text-zinc-500">
            <span className="flex items-center gap-1.5"><CheckCircle2 className="h-4 w-4 text-emerald-500" /> No credit card required</span>
            <span className="flex items-center gap-1.5"><CheckCircle2 className="h-4 w-4 text-emerald-500" /> Self-hosted on your infra</span>
            <span className="flex items-center gap-1.5"><CheckCircle2 className="h-4 w-4 text-emerald-500" /> MIT + Commons Clause license</span>
          </div>
        </div>
      </section>

      {/* ── Stats Bar ── */}
      <section className="border-y border-zinc-800 bg-zinc-900/40">
        <div className="mx-auto max-w-7xl px-6 py-12">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-3xl sm:text-4xl font-bold text-white mb-2">{stat.value}</div>
                <div className="text-sm text-zinc-500">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Features ── */}
      <section id="features" className="py-28 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="text-center mb-16">
            <Badge className="mb-4 bg-zinc-800 text-zinc-400 border-zinc-700 hover:bg-zinc-800">Features</Badge>
            <h2 className="text-4xl sm:text-5xl font-bold mb-5">Everything your team needs</h2>
            <p className="text-zinc-400 text-lg max-w-xl mx-auto">
              Built with a Go backend and Next.js frontend. Secure, performant, and designed to scale with your business from day one.
            </p>
          </div>

          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-5">
            {features.map((feature) => {
              const Icon = feature.icon
              return (
                <div
                  key={feature.title}
                  className={`group rounded-2xl border border-zinc-800 bg-zinc-900/60 p-6 ${feature.glow} transition-all duration-300 hover:bg-zinc-900`}
                >
                  <div className={`mb-4 inline-flex h-11 w-11 items-center justify-center rounded-xl ${feature.bg} ${feature.accent}`}>
                    <Icon className="h-5 w-5" />
                  </div>
                  <h3 className="text-lg font-semibold mb-2 text-white">{feature.title}</h3>
                  <p className="text-zinc-400 text-sm leading-relaxed">{feature.description}</p>
                </div>
              )
            })}
          </div>
        </div>
      </section>

      {/* ── How it works ── */}
      <section id="how-it-works" className="py-28 px-6 bg-zinc-900/25 border-y border-zinc-800/50">
        <div className="mx-auto max-w-7xl">
          <div className="text-center mb-16">
            <Badge className="mb-4 bg-zinc-800 text-zinc-400 border-zinc-700 hover:bg-zinc-800">How it works</Badge>
            <h2 className="text-4xl sm:text-5xl font-bold mb-5">Up and running in minutes</h2>
            <p className="text-zinc-400 text-lg max-w-xl mx-auto">
              Docker-based deployment. One command to start the entire stack — Postgres, backend, and web — all wired up and ready.
            </p>
          </div>

          <div className="grid md:grid-cols-3 gap-10">
            {steps.map((step) => (
              <div key={step.number} className="text-center relative">
                <div className="inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-zinc-800/80 border border-zinc-700 text-2xl font-bold text-emerald-400 mb-6 mx-auto">
                  {step.number}
                </div>
                <h3 className="text-xl font-semibold mb-3">{step.title}</h3>
                <p className="text-zinc-400 leading-relaxed">{step.description}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Dynamic Roles ── */}
      <section id="roles" className="py-28 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="grid lg:grid-cols-2 gap-20 items-center">
            <div>
              <Badge className="mb-5 bg-violet-500/10 text-violet-400 border-violet-500/20 hover:bg-violet-500/10">Dynamic RBAC</Badge>
              <h2 className="text-4xl sm:text-5xl font-bold mb-6 leading-tight">
                Roles that adapt to
                <br />
                <span className="text-violet-400">your business</span>
              </h2>
              <p className="text-zinc-400 text-lg leading-relaxed mb-8">
                Don&apos;t rebuild your app every time your org chart changes. Add new roles, assign permissions, and reload — all from the admin dashboard. Zero downtime, zero code changes.
              </p>
              <ul className="space-y-4">
                {[
                  'Create unlimited custom roles from the UI',
                  'Visual permission matrix — toggle per resource',
                  'System roles protected from accidental deletion',
                  'FK-enforced role integrity in PostgreSQL',
                  'In-memory permission cache — no DB hit per request',
                ].map((item) => (
                  <li key={item} className="flex items-start gap-3 text-zinc-300 text-sm">
                    <CheckCircle2 className="h-5 w-5 text-violet-400 flex-shrink-0 mt-0.5" />
                    {item}
                  </li>
                ))}
              </ul>
            </div>

            <div className="relative">
              <div className="rounded-2xl border border-zinc-800 bg-zinc-900 overflow-hidden shadow-2xl shadow-zinc-950">
                <div className="border-b border-zinc-800 px-5 py-4 flex items-center justify-between bg-zinc-900/80">
                  <span className="font-semibold text-sm text-zinc-200">Roles & Permissions</span>
                  <Badge className="bg-zinc-800 text-zinc-400 text-xs border-zinc-700">Live preview</Badge>
                </div>
                <div className="p-5">
                  <div className="mb-3 flex items-center gap-3">
                    <div className="w-28" />
                    {['view', 'create', 'edit', 'delete'].map(p => (
                      <div key={p} className="w-6 text-center text-[10px] text-zinc-500 uppercase tracking-wide">{p[0]}</div>
                    ))}
                  </div>
                  <div className="space-y-3">
                    {[
                      { role: 'admin', perms: [true, true, true, true], color: 'text-purple-400' },
                      { role: 'manager', perms: [true, true, true, false], color: 'text-blue-400' },
                      { role: 'staff', perms: [true, false, false, false], color: 'text-emerald-400' },
                      { role: 'accountant ✨', perms: [true, true, false, false], color: 'text-yellow-400' },
                    ].map((row) => (
                      <div key={row.role} className="flex items-center gap-3">
                        <span className={`text-sm font-medium w-28 truncate ${row.color}`}>{row.role}</span>
                        {row.perms.map((has, i) => (
                          <div
                            key={i}
                            className={`h-6 w-6 rounded flex items-center justify-center text-xs font-bold ${
                              has ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-800 text-zinc-600'
                            }`}
                          >
                            {has ? '✓' : '×'}
                          </div>
                        ))}
                      </div>
                    ))}
                  </div>
                  <div className="mt-5 pt-4 border-t border-zinc-800 flex items-center gap-2">
                    <div className="h-8 flex-1 rounded-lg bg-zinc-800/70 border border-zinc-700 px-3 flex items-center">
                      <span className="text-xs text-zinc-600">New role name...</span>
                    </div>
                    <div className="h-8 px-3 rounded-lg bg-emerald-500/15 border border-emerald-700/30 flex items-center text-xs text-emerald-400 font-medium whitespace-nowrap">
                      + Add Role
                    </div>
                  </div>
                </div>
              </div>

              <div className="absolute -top-3 -right-3 bg-emerald-500 text-white text-xs font-bold px-3 py-1.5 rounded-full shadow-lg shadow-emerald-500/40">
                Live · No redeploy needed
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Tech Stack ── */}
      <section className="py-16 px-6 border-y border-zinc-800 bg-zinc-900/25">
        <div className="mx-auto max-w-7xl">
          <p className="text-center text-zinc-600 text-xs mb-8 uppercase tracking-widest font-semibold">Powered by</p>
          <div className="flex flex-wrap items-center justify-center gap-x-10 gap-y-5">
            {['Go 1.26', 'Next.js 16', 'PostgreSQL 16', 'Redis / Valkey', 'Docker', 'pgx v5', 'TanStack Query', 'Tailwind CSS v4'].map((tech) => (
              <span key={tech} className="text-zinc-500 font-medium text-sm hover:text-zinc-300 transition-colors cursor-default">
                {tech}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* ── Final CTA ── */}
      <section className="py-36 px-6 relative overflow-hidden">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#ffffff05_1px,transparent_1px),linear-gradient(to_bottom,#ffffff05_1px,transparent_1px)] bg-[size:40px_40px]" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[700px] h-[500px] bg-emerald-500/7 rounded-full blur-3xl pointer-events-none" />

        <div className="relative mx-auto max-w-3xl text-center">
          <h2 className="text-5xl sm:text-6xl font-bold mb-6 leading-tight">
            Ready to take control
            <br />
            <span className="bg-gradient-to-r from-emerald-400 to-teal-400 bg-clip-text text-transparent">
              of your inventory?
            </span>
          </h2>
          <p className="text-zinc-400 text-xl mb-10 leading-relaxed max-w-xl mx-auto">
            Self-host in minutes with Docker. Full control over your data. No vendor lock-in. No monthly fees.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link href="/register">
              <Button size="lg" className="bg-emerald-500 hover:bg-emerald-400 text-white px-10 gap-2 text-base font-semibold shadow-2xl shadow-emerald-500/20 transition-all hover:scale-105">
                Get started free <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
            <Link href="/login">
              <Button size="lg" variant="outline" className="border-zinc-700 text-zinc-300 hover:bg-zinc-800 hover:text-white hover:border-zinc-600 px-10 text-base transition-all">
                Sign in <ChevronRight className="h-4 w-4 ml-1" />
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="border-t border-zinc-800 py-10 px-6">
        <div className="mx-auto max-w-7xl flex flex-col md:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-500">
              <Package className="h-4 w-4 text-white" />
            </div>
            <span className="font-bold text-sm">IMS</span>
          </div>

          <div className="flex items-center gap-8 text-sm text-zinc-500">
            <a href="#features" className="hover:text-white transition-colors">Features</a>
            <Link href="/login" className="hover:text-white transition-colors">Sign in</Link>
            <Link href="/register" className="hover:text-white transition-colors">Register</Link>
            <a
              href="https://github.com/TarunVishwakarma1/ims"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white transition-colors"
            >
              GitHub
            </a>
          </div>

          <p className="text-sm text-zinc-600">
            © 2026 Tarun Vishwakarma. MIT + Commons Clause.
          </p>
        </div>
      </footer>

    </div>
  )
}
