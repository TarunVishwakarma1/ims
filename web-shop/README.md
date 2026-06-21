# web-shop

B2C storefront. Next.js 16 + React 19 + Tailwind 4. Pinned to versions in `web/`.

## Dev

```bash
cd web-shop
cp .env.example .env.local   # edit BACKEND_URL if backend not on localhost:8080
bun install
bun run dev                  # http://localhost:3000
```

Backend must be running (`docker compose up postgres valkey && cd backend && SHOP_ENABLED=true go run ./cmd/server`).

## Build

```bash
bun run build
bun run start
```

## Docker

```bash
docker build -t web-shop .
docker run --rm -p 3000:3000 -e BACKEND_URL=http://host.docker.internal:8080 web-shop
```

## Routing notes

- `/api/shop/*` and `/uploads/*` rewritten to backend by `next.config.ts`.
- `/api/auth/*` owned by Next route handlers (sets `shop_session` httpOnly cookie).
- Protected routes (`/orders`, `/checkout`, `/addresses`, `/profile`) redirect to `/login?next=…` via `proxy.ts`.
- Cart drawer uses the `@cart` parallel route slot. Plan 3e fills it.

## Sub-plans

- 3a (this) — scaffold.
- 3b — OTP login flow.
- 3c — home (banners + categories + feed).
- 3d — catalog + search + product detail.
- 3e — cart + checkout (COD + Razorpay).
- 3f — addresses + orders + cancel.

## Conventions

- `web/AGENTS.md` rule applies: Next 16 differs from training-data Next. Read `node_modules/next/dist/docs/` before non-trivial Next-specific work.
- Components live under `components/`. shadcn primitives go under `components/ui/`.
- HTTP: browser via `lib/api.ts:api` (ky), server via `lib/api.ts:serverFetch` (cookie-aware).
- Theme tokens live in `app/globals.css` `@theme` block.
