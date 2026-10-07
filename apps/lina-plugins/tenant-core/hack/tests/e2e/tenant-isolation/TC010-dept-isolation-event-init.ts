import { test, expect } from "../../support/tenant-core";
import { scenarioTC0196 } from "../../support/tenant-core-scenarios";

test.describe("TC-6 租户角色隔离与无事件队列", () => {
  test.use({ multiTenantMode: "tenant-core-enabled" });

  test("TC-6a: tenant role rows use tenant scope without event outbox", async ({
    multiTenantMode,
  }) => {
    expect(multiTenantMode).toBe("tenant-core-enabled");
    await scenarioTC0196();
  });
});
