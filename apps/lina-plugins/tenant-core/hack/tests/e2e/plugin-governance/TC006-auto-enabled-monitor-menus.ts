import { test, expect } from "../../support/tenant-core";
import { scenarioTC0237 } from "../../support/tenant-core-scenarios";

test.describe("TC-2 auto-enabled tenant plugin menus", () => {
  test.use({ multiTenantMode: "tenant-core-enabled" });

  test("TC-2a: tenant routes include the auto-enabled tenant plugin", async ({
    multiTenantMode,
  }) => {
    expect(multiTenantMode).toBe("tenant-core-enabled");
    await scenarioTC0237();
  });
});
