export const SHOP_SESSION_COOKIE = "shop_session";
export const COOKIE_MAX_AGE_DAYS = 30;

// cookieSecure decides whether the session cookie carries the Secure flag.
//
// A Secure cookie is never sent over plain http, so it MUST be off when the
// app is served on http://localhost (the Docker dev stack runs `next start`,
// which forces NODE_ENV=production — so keying Secure off NODE_ENV wrongly
// enabled it for local http and the browser silently dropped the cookie).
//
// Control it explicitly via COOKIE_SECURE:
//   COOKIE_SECURE=true   → Secure on (real https deployments)
//   COOKIE_SECURE=false  → Secure off (local http)
//   unset                → off (safe default for local dev)
function cookieSecure(): boolean {
  return process.env.COOKIE_SECURE === "true";
}

export function shopSessionCookieOptions() {
  return {
    name: SHOP_SESSION_COOKIE,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: cookieSecure(),
    path: "/",
    maxAge: COOKIE_MAX_AGE_DAYS * 24 * 60 * 60,
  };
}
