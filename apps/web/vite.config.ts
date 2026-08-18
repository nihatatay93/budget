import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: resolve(import.meta.dirname, "../server/internal/webui/dist"),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/healthz": "http://localhost:8080",
      "/readyz": "http://localhost:8080",
      "/v1": "http://localhost:8080",
    },
  },
});
