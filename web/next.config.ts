import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    const target = process.env.API_PROXY_TARGET;
    if (!target) {
      return [];
    }

    return [
      {
        source: "/v1/:path*",
        destination: `${target.replace(/\/$/, "")}/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
