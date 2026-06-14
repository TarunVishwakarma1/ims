'use client'

import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useRouter } from 'next/navigation'
import { Eye, EyeOff, Loader2, Package, CheckCircle2 } from 'lucide-react'
import Link from 'next/link'

import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { ApiError } from '@/types/api'
import { cn } from '@/lib/utils'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { HTTPError } from 'ky'

const registerSchema = z.object({
  org_name: z.string().min(2, { message: 'Organization name is required' }),
  org_slug: z.string().min(2, { message: 'Organization slug is required' }).regex(/^[a-z0-9-]+$/, 'Slug can only contain lowercase letters, numbers, and hyphens'),
  user_name: z.string().min(1, { message: 'Your name is required' }),
  email: z.string().email({ message: 'Invalid email address' }),
  password: z.string().min(8, { message: 'Password must be at least 8 characters' }),
  confirmPassword: z.string(),
}).refine(data => data.password === data.confirmPassword, {
  message: "Passwords don't match", 
  path: ['confirmPassword'],
})

type RegisterFormValues = z.infer<typeof registerSchema>

export default function RegisterPage() {
  const router = useRouter()
  const { login } = useAuthStore()
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  
  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      org_name: '',
      org_slug: '',
      user_name: '',
      email: '',
      password: '',
      confirmPassword: '',
    },
  })

  // Auto-generate slug from org name
  const orgName = watch('org_name');
  useEffect(() => {
    if (orgName) {
      const slug = orgName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
      setValue('org_slug', slug, { shouldValidate: true });
    }
  }, [orgName, setValue]);

  const onSubmit = async (data: RegisterFormValues) => {
    setError(null)
    try {
      const response = await authApi.register({
        org_name: data.org_name,
        org_slug: data.org_slug,
        user_name: data.user_name,
        email: data.email,
        password: data.password,
      })
      
      // Auto-login after registration
      login(response)
      router.push('/')
    } catch (err) {
      if (err instanceof HTTPError) {
        try {
          const errorData = await err.response.json() as ApiError
          setError(errorData.error || 'Registration failed.')
        } catch {
          setError('An unexpected error occurred.')
        }
      } else {
        setError(err instanceof Error ? err.message : 'Network error. Please try again later.')
      }
    }
  }

  return (
    <div className="flex min-h-screen bg-white dark:bg-zinc-950">
      <div className="hidden lg:flex lg:w-1/2 bg-gradient-to-br from-zinc-900 via-zinc-800 to-zinc-950 flex-col justify-between p-12 relative overflow-hidden order-2">
        {/* Subtle grid pattern background */}
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]"></div>
        
        <div className="relative flex items-center gap-2">
          <Package className="h-8 w-8 text-white" />
          <span className="text-2xl font-bold text-white">IMS</span>
        </div>
        
        <div className="relative space-y-6">
          <h1 className="text-4xl font-bold text-white leading-tight">
            Manage your inventory<br />with confidence
          </h1>
          <ul className="space-y-3 text-zinc-400">
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Real-time stock tracking</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Role-based access control</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Full audit logging</li>
            <li className="flex items-center gap-2"><CheckCircle2 className="h-4 w-4 text-emerald-400" /> Multi-tenant workspaces</li>
          </ul>
        </div>
        
        <p className="relative text-zinc-600 text-sm">© 2026 IMS. All rights reserved.</p>
      </div>

      <div className="w-full lg:w-1/2 flex items-center justify-center p-8 sm:p-12 order-1 overflow-y-auto">
        <div className="w-full max-w-md space-y-8 my-8">
          <div className="space-y-2 text-center lg:text-left">
            <h2 className="text-3xl font-bold tracking-tight">Create your workspace</h2>
            <p className="text-muted-foreground">
              Set up your organization and admin account
            </p>
          </div>

          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            
            <div className="space-y-4 rounded-xl border p-4 bg-zinc-50/50 dark:bg-zinc-900/50">
              <h3 className="font-semibold text-sm uppercase tracking-wider text-muted-foreground">Organization Details</h3>
              
              <div className="space-y-2">
                <Label htmlFor="org_name">Organization Name</Label>
                <Input
                  id="org_name"
                  type="text"
                  placeholder="Acme Corp"
                  {...register('org_name')}
                  className={errors.org_name ? 'border-red-500' : ''}
                  disabled={isSubmitting}
                />
                {errors.org_name && (
                  <p className="text-sm text-red-500">{errors.org_name.message}</p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="org_slug">Workspace URL Slug</Label>
                <div className="flex items-center gap-2">
                  <span className="text-sm text-muted-foreground">ims.app/</span>
                  <Input
                    id="org_slug"
                    type="text"
                    placeholder="acme-corp"
                    {...register('org_slug')}
                    className={cn('flex-1', errors.org_slug && 'border-red-500')}
                    disabled={isSubmitting}
                  />
                </div>
                {errors.org_slug && (
                  <p className="text-sm text-red-500">{errors.org_slug.message}</p>
                )}
              </div>
            </div>

            <div className="space-y-4 rounded-xl border p-4 bg-zinc-50/50 dark:bg-zinc-900/50">
              <h3 className="font-semibold text-sm uppercase tracking-wider text-muted-foreground">Admin Account</h3>
              
              <div className="space-y-2">
                <Label htmlFor="user_name">Full Name</Label>
                <Input
                  id="user_name"
                  type="text"
                  placeholder="John Doe"
                  {...register('user_name')}
                  className={errors.user_name ? 'border-red-500' : ''}
                  disabled={isSubmitting}
                />
                {errors.user_name && (
                  <p className="text-sm text-red-500">{errors.user_name.message}</p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="m@example.com"
                  {...register('email')}
                  className={errors.email ? 'border-red-500' : ''}
                  disabled={isSubmitting}
                />
                {errors.email && (
                  <p className="text-sm text-red-500">{errors.email.message}</p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPassword ? 'text' : 'password'}
                    {...register('password')}
                    className={errors.password ? 'border-red-500 pr-10' : 'pr-10'}
                    disabled={isSubmitting}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 focus:outline-none"
                    tabIndex={-1}
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    <span className="sr-only">{showPassword ? 'Hide password' : 'Show password'}</span>
                  </button>
                </div>
                {errors.password && (
                  <p className="text-sm text-red-500">{errors.password.message}</p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="confirmPassword">Confirm Password</Label>
                <Input
                  id="confirmPassword"
                  type={showPassword ? 'text' : 'password'}
                  {...register('confirmPassword')}
                  className={errors.confirmPassword ? 'border-red-500' : ''}
                  disabled={isSubmitting}
                />
                {errors.confirmPassword && (
                  <p className="text-sm text-red-500">{errors.confirmPassword.message}</p>
                )}
              </div>
            </div>

            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating workspace...
                </>
              ) : (
                'Create workspace'
              )}
            </Button>
          </form>

          <p className="text-center text-sm text-muted-foreground">
            Already have an account?{' '}
            <Link href="/login" className="font-medium text-primary hover:underline">
              Sign in →
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
