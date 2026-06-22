export const SHOP_SESSION_COOKIE = "shop_session";
export const COOKIE_MAX_AGE_DAYS = 30;

export function shopSessionCookieOptions() {
  return {
    name: SHOP_SESSION_COOKIE,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: COOKIE_MAX_AGE_DAYS * 24 * 60 * 60,
  };
}
