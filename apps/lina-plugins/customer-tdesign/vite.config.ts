import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
export default defineConfig({
  root: "frontend",
  base: "./",
  plugins: [react()],
  resolve: {
    alias: { "#": fileURLToPath(new URL("./frontend/src", import.meta.url)) },
  },
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    proxy: Object.fromEntries(
      ["/api", "/x", "/x-assets", "/__website"].map((path) => [
        path,
        {
          target: process.env.CUSTOMER_API_ORIGIN || "http://127.0.0.1:6032",
          changeOrigin: true,
        },
      ]),
    ),
  },
  test: { include: ["src/**/*.test.ts"] },
});
