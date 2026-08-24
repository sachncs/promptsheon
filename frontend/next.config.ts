import type { NextConfig } from 'next';
import path from 'node:path';

// PROMPTSHEON_PORT (default 8080) is the port the Fastify backend
// listens on. Letting the rewrite follow it means the test harness
// can run the backend on any free port (e.g. 8081) without
// changing the frontend.
const BACKEND_PORT = process.env['PROMPTSHEON_PORT'] ?? '8080';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  transpilePackages: ['@xyflow/react'],
  turbopack: {
    root: path.join(__dirname, '..'),
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `http://localhost:${BACKEND_PORT}/api/:path*`,
      },
      {
        source: '/events/:path*',
        destination: `http://localhost:${BACKEND_PORT}/events/:path*`,
      },
    ];
  },
};

export default nextConfig;