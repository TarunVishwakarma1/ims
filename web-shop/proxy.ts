import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { SHOP_SESSION_COOKIE } from "@/lib/cookies";

// /checkout and /orders host their own LoginModal that drives the OTP flow
// in-context (P3e2). Guard only pages that have no in-page auth path so we
// don't end up showing the login UI twice (once full-page, once modal).
const PROTECTED_PREFIXES = ["/addresses", "/profile"];

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;

  // Translate shop_session cookie → Authorization Bearer for /api/shop/*
  // so the backend's RequireCustomer middleware sees the token.
  if (pathname.startsWith("/api/shop/")) {
    const token = req.cookies.get(SHOP_SESSION_COOKIE)?.value;
    if (!token) return NextResponse.next();
    const headers = new Headers(req.headers);
    headers.set("Authorization", `Bearer ${token}`);
    return NextResponse.next({ request: { headers } });
  }

  if (!PROTECTED_PREFIXES.some((p) => pathname === p || pathname.startsWith(p + "/"))) {
    return NextResponse.next();
  }
  if (req.cookies.has(SHOP_SESSION_COOKIE)) {
    return NextResponse.next();
  }
  const loginURL = req.nextUrl.clone();
  loginURL.pathname = "/login";
  loginURL.searchParams.set("next", pathname);
  return NextResponse.redirect(loginURL);
}

export const config = {
  matcher: [
    "/api/shop/:path*",
    "/orders/:path*",
    "/checkout/:path*",
    "/addresses/:path*",
    "/profile/:path*",
  ],
};
