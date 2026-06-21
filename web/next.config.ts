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
// Razorpay checkout pulls a script from checkout.razorpay.com, renders the
// payment modal inside an iframe served from api.razorpay.com, beacons
// telemetry to lumberjack.razorpay.com, and pulls method icons from cdn.
// All of those need explicit CSP allowances or the widget never opens.
const razorpayScript = "https://checkout.razorpay.com https://cdn.razorpay.com";
const razorpayFrame = "https://api.razorpay.com https://checkout.razorpay.com";
const razorpayConnect = "https://api.razorpay.com https://lumberjack.razorpay.com https://lumberjack-cx.razorpay.com";
const razorpayImg = "https://*.razorpay.com https://*.rzp.io";

const csp = [
  "default-src 'self'",
  isProd
    ? `script-src 'self' 'unsafe-inline' ${razorpayScript}`
    : `script-src 'self' 'unsafe-eval' 'unsafe-inline' ${razorpayScript}`,
  // Tailwind / shadcn pump styles inline — allow with 'unsafe-inline'.
  // Long-term: move to nonced inline-styles via Next's CSP nonce.
  "style-src 'self' 'unsafe-inline'",
  `img-src 'self' data: blob: https://*.tile.openstreetmap.org https://unpkg.com ${razorpayImg}`,
  "font-src 'self' data:",
  // EventSource (SSE) + fetch to the backend + Razorpay telemetry.
  `connect-src 'self' ${apiOrigin} ${razorpayConnect}`,
  // Razorpay renders the payment UI inside an iframe served from
  // api.razorpay.com / checkout.razorpay.com — without frame-src they're
  // blocked silently and the modal never appears.
  `frame-src 'self' ${razorpayFrame}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  // Razorpay also POSTs to its own domain for the final auth step.
  `form-action 'self' ${razorpayFrame}`,
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
      "geolocation=(self), camera=(), microphone=(), payment=(self), usb=(), magnetometer=(), gyroscope=(), accelerometer=()",
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
