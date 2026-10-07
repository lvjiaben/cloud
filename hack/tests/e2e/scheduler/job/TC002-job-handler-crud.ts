import type { APIRequestContext } from "@playwright/test";

import { test, expect } from "../../../fixtures/auth";
import { JobPage } from "../../../pages/JobPage";

import { createAdminApiContext, listJobs } from "../../../support/api/job";

test.describe("TC-2 源码注册任务可见且只读", () => {
  let api: APIRequestContext;

  test.beforeAll(async () => {
    api = await createAdminApiContext();
  });

  test.afterAll(async () => {
    await api.dispose();
  });

  test("TC-2a~e: 宿主内置任务应显示在列表中，公共新增入口不再支持 Handler，源码任务仅可查看详情", async ({
    adminPage,
  }) => {
    const expectedBuiltinNames = ["任务日志清理", "在线会话清理"];

    for (const name of expectedBuiltinNames) {
      const result = await listJobs(api, name);
      expect(
        result.list.some((item) => item.name === name && item.isBuiltin === 1),
      ).toBeTruthy();
    }

    const jobPage = new JobPage(adminPage);
    await jobPage.goto();

    if (process.platform === "win32") {
      await expect(adminPage.getByTestId("job-add")).toHaveCount(0);
    } else {
      await jobPage.openCreate();
      await expect(adminPage.getByTestId("job-form-tab-handler")).toHaveCount(
        0,
      );
      await expect(
        adminPage.getByLabel("任务处理器", { exact: true }),
      ).toHaveCount(0);
      await jobPage.closeDialog();
    }

    await jobPage.fillSearchKeyword("任务日志清理");
    await jobPage.clickSearch();
    await expect(await jobPage.hasJob("任务日志清理")).toBe(true);

    await jobPage.openEditSearchedJob();
    const builtinDetailCard = adminPage.getByTestId("job-builtin-detail-card");
    await expect(builtinDetailCard).toBeVisible();
    await expect(
      builtinDetailCard.getByText("宿主内置", { exact: true }),
    ).toBeVisible();
    await expect(
      builtinDetailCard.getByText("host:cleanup-job-logs", { exact: true }),
    ).toBeVisible();
    await expect(
      adminPage.getByRole("button", { name: /确\s*认/ }),
    ).toHaveCount(0);
  });
});
