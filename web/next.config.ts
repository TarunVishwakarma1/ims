import type { NextConfig } from "next";

// Backend API origin that the browser needs to reach for connect-src.
// Set NEXT_PUBLIC_API_URL on the deployer; defaults to localhost for dev.
const apiOrigin = (() => {
  try {
    const url = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api";
    return new URL(url).origin;
  } catch {
    return "http://localhost:8080";
  }
})();

const isProd = process.env.NODE_ENV === "production";

// Build a Content-Security-Policy string. Next.js injects inline scripts
// for hydration data + next-themes injects an inline script to avoid a
// flash of unstyled content. Both require 'unsafe-inline' on script-src.
// Strict-CSP-with-nonces is the long-term fix (Next 15 supports it via
// middleware) but for now we accept 'unsafe-inline' to keep the app
// functional. React still defangs XSS at the framework level.
const csp = [
  "default-src 'self'",
  isProd
    ? "script-src 'self' 'unsafe-inline'"
    : "script-src 'self' 'unsafe-eval' 'unsafe-inline'",
  // Tailwind / shadcn pump styles inline — allow with 'unsafe-inline'.
  // Long-term: move to nonced inline-styles via Next's CSP nonce.
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  // EventSource (SSE) + fetch to the backend.
  `connect-src 'self' ${apiOrigin}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  // Block plugins entirely
  "object-src 'none'",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: csp },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    value:
      "geolocation=(), camera=(), microphone=(), payment=(self), usb=(), magnetometer=(), gyroscope=(), accelerometer=()",
  },
  // HSTS prod-only — accidental localhost HSTS pinning would force https
  // on dev sessions.
  ...(isProd
    ? [
        {
          key: "Strict-Transport-Security",
          value: "max-age=63072000; includeSubDomains; preload",
        },
      ]
    : []),
];

const nextConfig: NextConfig = {
  output: "standalone",
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
