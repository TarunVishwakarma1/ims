import type { NextConfig } from "next";

const backend = process.env.BACKEND_URL || "http://localhost:8080";

const config: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  images: {
    remotePatterns: [{ protocol: "https", hostname: "**" }],
  },
  async rewrites() {
    return [
      { source: "/api/shop/:path*", destination: `${backend}/api/shop/:path*` },
      { source: "/uploads/:path*", destination: `${backend}/uploads/:path*` },
    ];
  },
};

export default config;
