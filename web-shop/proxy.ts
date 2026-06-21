import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { SHOP_SESSION_COOKIE } from "@/lib/cookies";

const PROTECTED_PREFIXES = ["/orders", "/checkout", "/addresses", "/profile"];

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;
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
  matcher: ["/orders/:path*", "/checkout/:path*", "/addresses/:path*", "/profile/:path*"],
};
