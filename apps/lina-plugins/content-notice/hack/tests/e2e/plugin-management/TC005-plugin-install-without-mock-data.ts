import type { APIResponse } from "@host-tests/support/playwright";

import { test, expect } from "@host-tests/fixtures/auth";
import { createAdminApiContext, findPlugin } from "@host-tests/fixtures/plugin";
import { queryPgScalar } from "@host-tests/support/postgres";

const pluginID = "content-notice";

async function expectDenied(response: APIResponse) {
  const payload = (await response.json()) as { code?: number };
  expect(response.ok() && payload.code === 0).toBe(false);
}

test.describe("TC-5 内置通知公告数据留存", () => {
  test("TC-5a: 停用和卸载请求失败且不会删除公告表", async () => {
    const adminApi = await createAdminApiContext();
    try {
      const before = queryPgScalar(
        "SELECT COUNT(*) FROM plugin_content_notice;",
      );
      await expectDenied(await adminApi.put(`plugins/${pluginID}/disable`));
      await expectDenied(
        await adminApi.delete(`plugins/${pluginID}`, {
          data: { purgeStorageData: 1 },
        }),
      );
      expect(queryPgScalar("SELECT COUNT(*) FROM plugin_content_notice;")).toBe(
        before,
      );
      expect(await findPlugin(adminApi, pluginID)).toMatchObject({
        enabled: 1,
        installed: 1,
      });
    } finally {
      await adminApi.dispose();
    }
  });
});
