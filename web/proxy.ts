import type { NextRequest } from 'next/server'
import { NextResponse } from 'next/server'

// Protected routes — require auth
const PROTECTED = ['/dashboard', '/products', '/inventory', '/orders', '/users', '/categories']

// Auth routes — redirect to dashboard if already logged in
const AUTH_ROUTES = ['/login', '/register']

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl
  const token = request.cookies.get('ims_access_token')?.value
    ?? request.headers.get('Authorization')?.replace('Bearer ', '')

  // Handle root path
  if (pathname === '/') {
    if (token) {
      return NextResponse.redirect(new URL('/dashboard', request.url))
    }
    return NextResponse.next()
  }

  const isProtected = PROTECTED.some(route => pathname.startsWith(route))
  const isAuthRoute = AUTH_ROUTES.some(route => pathname.startsWith(route))

  if (isProtected && !token) {
    return NextResponse.redirect(new URL('/login', request.url))
  }

  if (isAuthRoute && token) {
    return NextResponse.redirect(new URL('/dashboard', request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
}
