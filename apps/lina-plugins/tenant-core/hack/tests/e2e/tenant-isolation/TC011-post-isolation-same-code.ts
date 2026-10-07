import { test, expect } from "../../support/tenant-core";
import { scenarioTC0197 } from "../../support/tenant-core-scenarios";

test.describe("TC-7 角色标识跨租户隔离", () => {
  test.use({ multiTenantMode: "tenant-core-enabled" });

  test("TC-7a: role keys with the same prefix stay isolated across tenants", async ({
    multiTenantMode,
  }) => {
    expect(multiTenantMode).toBe("tenant-core-enabled");
    await scenarioTC0197();
  });
});
