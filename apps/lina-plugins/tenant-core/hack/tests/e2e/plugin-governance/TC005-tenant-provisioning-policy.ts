import { test, expect } from "../../support/tenant-core";
import { scenarioTC0213 } from "../../support/tenant-core-scenarios";

test.describe("TC-1 tenant provisioning policy", () => {
  test.use({ multiTenantMode: "tenant-core-enabled" });

  test("TC-1a: new tenant receives platform-managed tenant-scoped plugin enablement", async ({
    multiTenantMode,
  }) => {
    expect(multiTenantMode).toBe("tenant-core-enabled");
    await scenarioTC0213();
  });
});
