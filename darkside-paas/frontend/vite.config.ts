import path from "node:path"
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import { tanstackRouter } from "@tanstack/router-plugin/vite"
import tailwindcss from "@tailwindcss/vite"

// The order matters: tanstack router must come before react so its codegen
// runs first. See https://tanstack.com/router/latest/docs/installation/with-vite
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // In dev mode, the frontend runs on :5173 and proxies API calls to the
      // Go binary on :8080. In production the Go binary serves the built SPA
      // directly so this proxy is irrelevant.
      "/darkside.v1.HealthService": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/webhooks": "http://localhost:8080",
    },
  },
})
