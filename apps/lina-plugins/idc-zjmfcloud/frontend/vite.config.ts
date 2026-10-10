import { resolve } from "node:path";
import { defineConfig } from "vite";
export default defineConfig({
  base: "./",
  build: {
    outDir: "dist",
    rollupOptions: {
      input: {
        admin: resolve(
          import.meta.dirname,
          "admin/server-management/index.html",
        ),
        customer: resolve(import.meta.dirname, "customer/servers/index.html"),
      },
    },
  },
});
