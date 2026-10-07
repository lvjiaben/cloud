import { fileURLToPath } from "node:url";

// Formal pages use the host's real RequestClient. Resolve its existing test
// dependencies without adding them to the independent plugin runtime bundle.
const path = (value) => fileURLToPath(new URL(value, import.meta.url));

export default {
  root: path("."),
  resolve: {
    alias: {
      "@vben/request": path("../../../lina-vben/packages/effects/request/src/index.ts"),
      "axios-mock-adapter": path("../../../lina-vben/packages/effects/request/node_modules/axios-mock-adapter"),
      vitest: path("../../../lina-vben/node_modules/vitest/dist/index.js"),
    },
  },
  test: {
    environment: "happy-dom",
    include: ["tests/commerce-request.test.ts"],
    maxWorkers: 1,
  },
};
