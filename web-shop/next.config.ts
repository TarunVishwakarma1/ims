import type { NextConfig } from "next";

const backend = process.env.BACKEND_URL || "http://localhost:8080";
const backendUrl = new URL(backend);

// Allowed image hosts. Default: only the configured backend host —
// /_next/image with `**` wildcard is an SSRF amplifier. Extend in prod via
// SHOP_IMAGE_HOSTS env (comma-separated https hostnames).
const extraHosts = (process.env.SHOP_IMAGE_HOSTS || "")
  .split(",")
  .map((h) => h.trim())
  .filter((h) => h && h !== "**");

const remotePatterns = [
  {
    protocol: backendUrl.protocol.replace(":", "") as "http" | "https",
    hostname: backendUrl.hostname,
    port: backendUrl.port || undefined,
  },
  ...extraHosts.map((hostname) => ({ protocol: "https" as const, hostname })),
];

const config: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  images: {
    remotePatterns,
  },
  async rewrites() {
    return [
      { source: "/api/shop/:path*", destination: `${backend}/api/shop/:path*` },
      { source: "/uploads/:path*", destination: `${backend}/uploads/:path*` },
    ];
  },
};

export default config;
