import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  transpilePackages: ['@xyflow/react'],
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/api/:path*',
      },
      {
        source: '/events/:path*',
        destination: 'http://localhost:8080/events/:path*',
      },
    ];
  },
};

export default nextConfig;