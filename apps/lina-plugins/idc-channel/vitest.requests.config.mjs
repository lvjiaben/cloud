import { fileURLToPath } from "node:url";

// Test the formal page against the same request and authentication interceptors
// supplied by Vben, using only the workspace's already installed dependencies.
const path = (value) => fileURLToPath(new URL(value, import.meta.url));

export default {
  root: path("."),
  resolve: {
    alias: {
      "@vben/request": path("../../lina-vben/packages/effects/request/src/index.ts"),
      "axios-mock-adapter": path("../../lina-vben/packages/effects/request/node_modules/axios-mock-adapter"),
      vitest: path("../../lina-vben/node_modules/vitest/dist/index.js"),
    },
  },
  test: {
    environment: "happy-dom",
    include: ["frontend/pages/channel-request.test.ts"],
    maxWorkers: 1,
  },
};
