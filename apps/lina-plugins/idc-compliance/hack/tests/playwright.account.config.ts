import hostConfig from "../../../../../hack/tests/playwright.config";

// Customer account contract cases use browser routes and do not need the
// unrelated authenticated administrator global setup.
export default {
  ...hostConfig,
  globalSetup: undefined,
  testDir: "./e2e",
  tsconfig: "../../../../../hack/tests/tsconfig.json",
};
