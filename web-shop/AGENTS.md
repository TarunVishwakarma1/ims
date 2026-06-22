<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This project uses Next.js 16. APIs, conventions, and file structure differ from training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices (e.g. `middleware.ts` → `proxy.ts`).
<!-- END:nextjs-agent-rules -->

## web-shop conventions

- Browser HTTP: `lib/api.ts:api` (ky, same-origin via `/api/shop/*` rewrite).
- Server HTTP: `lib/api.ts:serverFetch` (reads cookie, forwards Bearer to BACKEND_URL).
- httpOnly cookie name: `shop_session` (`lib/cookies.ts:SHOP_SESSION_COOKIE`).
- Protected route gate: `proxy.ts` (NOT `middleware.ts` — Next 16 idiom).
- Theme tokens: `app/globals.css` `@theme` block (Tailwind v4 CSS-first).
- Parallel cart slot: `app/@cart/default.tsx` (returns null; Plan 3e replaces).
