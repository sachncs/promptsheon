import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  // X-9: base defaults to "/" so assets resolve at the root of
  // the daemon's reverse-proxied mount. Operators deploying
  // under a sub-path (e.g. /promptsheon/) can override with
  // PROMPTSHEON_BASE or by editing this file.
  base: process.env.PROMPTSHEON_BASE || "/",
  plugins: [tailwindcss()],
  server: {
    port: 4173,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true, secure: false },
      "/health": { target: "http://localhost:8080", changeOrigin: true, secure: false },
      "/ready": { target: "http://localhost:8080", changeOrigin: true, secure: false }
    }
  },
  preview: { port: 4173 }
});
