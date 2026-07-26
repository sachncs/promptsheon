import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
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
