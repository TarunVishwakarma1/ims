FROM oven/bun:alpine AS deps
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

FROM oven/bun:alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Next bakes rewrite destinations into routes-manifest at build time, so
# BACKEND_URL must be available during `bun run build`. Compose passes the
# Docker service URL via build args; host dev falls back to localhost:8080.
ARG BACKEND_URL=http://localhost:8080
ENV BACKEND_URL=$BACKEND_URL
# next/image remote host allowlist is baked into the standalone server at
# build time, so SHOP_IMAGE_HOSTS must be present during `bun run build`
# (.env is excluded by .dockerignore). Compose passes it as a build arg.
ARG SHOP_IMAGE_HOSTS=""
ENV SHOP_IMAGE_HOSTS=$SHOP_IMAGE_HOSTS
ENV NEXT_TELEMETRY_DISABLED=1
RUN bun run build

FROM node:26-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3001
ENV HOSTNAME=0.0.0.0

RUN addgroup --system --gid 1001 nodejs \
    && adduser --system --uid 1001 nextjs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs
EXPOSE 3001
CMD ["node", "server.js"]
