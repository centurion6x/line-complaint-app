import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // During local dev (npm run dev), proxy API calls to the Go backend
    // running separately on :8080, so the LIFF app can be served over
    // HTTPS via a tunnel (ngrok/cloudflared) without also tunnelling the
    // backend.
    proxy: {
      "/api": "http://localhost:8080",
      "/images": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
  },
});
