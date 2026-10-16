import { expect, test } from "../../../../../../../hack/tests/fixtures/auth";

import { ProviderCapacityPage } from "../../pages/ProviderCapacityPage";
import { installProviderCapacityMock } from "../../support/provider-capacity-mock";

test.describe("TC-1 供应商资源更新", () => {
  test("TC-1a: 中文页面只展示有界安全投影和本页指标", async ({ adminPage }) => {
    await installProviderCapacityMock(adminPage);
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();

    await expect(
      capacity.page.getByText("供应商资源更新", { exact: true }).first(),
    ).toBeVisible();
    await expect(capacity.operationRow("魔方云生产渠道")).toContainText(
      "魔方云生产渠道",
    );
    await expect(capacity.operationRow("魔方云生产渠道")).toContainText("428");
    await expect(
      capacity.pageRoot.getByText("本页观测项").locator(".."),
    ).toContainText("428");
    await expect(capacity.pageRoot).not.toContainText(
      /credential|endpoint|fingerprint|remoteCursor|leaseToken/iu,
    );
    await capacity.screenshot("provider-capacity-operations-zh-cn");
  });

  test("TC-1b: 使用官方页码和真实总数进行前后翻页", async ({ adminPage }) => {
    const state = await installProviderCapacityMock(adminPage);
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();

    await expect(capacity.previousButton).toHaveClass(/is--disabled/);
    await capacity.nextPage();
    expect(state.listRequests.at(-1)?.searchParams.get("page")).toBe("2");
    expect(state.listRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
    await expect(capacity.operationRow("新加坡备用渠道")).toContainText(
      "供应商渠道暂时不可用",
    );

    await capacity.previousPage();
    expect(state.listRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
    expect(state.listRequests.at(-1)?.searchParams.get("page")).toBe("1");
    await expect(capacity.operationRow("魔方云生产渠道")).toBeVisible();
  });

  test("TC-1c: 筛选回第一页且原样提交受支持状态", async ({ adminPage }) => {
    const state = await installProviderCapacityMock(adminPage);
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();
    await capacity.nextPage();

    await capacity.selectFilter("provider-capacity-search-status", "失败");
    const request = state.listRequests.at(-1);
    expect(request?.searchParams.get("status")).toBe("failed");
    expect(request?.searchParams.get("cursor")).toBeNull();
    await expect(capacity.pageIndicator).toContainText("1");
  });

  test("TC-1d: 容量弹窗只能选择渠道和资源类型并提交最小请求", async ({
    adminPage,
  }) => {
    const state = await installProviderCapacityMock(adminPage);
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();
    await capacity.openSync("channel_mofang_prod");

    await expect(capacity.syncModal).toContainText(
      "更新供应商资源",
    );
    await expect(capacity.syncModal).toContainText("供应商接口");
    await expect(capacity.syncModal).toContainText("资源类型");
    await expect(
      capacity.syncModal.getByLabel(/库存数量|远端游标|接口地址|密钥/iu),
    ).toHaveCount(0);
    await capacity.selectSyncKind("镜像");
    await capacity.confirmSync();

    expect(state.syncRequests).toHaveLength(1);
    expect(state.syncRequests[0]?.method).toBe("POST");
    expect(state.syncRequests[0]?.url.pathname).toContain(
      "/channels/channel_mofang_prod/provider-capacity-observations",
    );
    expect(state.syncRequests[0]?.body).toEqual({ kind: "image" });
    await expect(
      capacity.operationRow("魔方云生产渠道").filter({ hasText: "采集中" }),
    ).toContainText("采集中");
  });

  test("TC-1e: 切换英文后使用完整英文运营文案", async ({
    adminPage,
    mainLayout,
  }) => {
    await installProviderCapacityMock(adminPage);
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();
    await mainLayout.switchLanguage("English");

    await expect(
      capacity.page
        .getByText("Supplier Resource Updates", { exact: true })
        .first(),
    ).toBeVisible();
    await expect(
      capacity.pageRoot.getByText("Observed items", { exact: true }),
    ).toBeVisible();
    await expect(capacity.operationRow("魔方云生产渠道")).toContainText(
      "Complete",
    );
    await expect(capacity.pageRoot).not.toContainText("plugin.idc-resource");
    await capacity.screenshot("provider-capacity-operations-en-us");
  });

  test("TC-1f: 只读权限不渲染任何同步入口", async ({ adminPage }) => {
    await installProviderCapacityMock(adminPage, { canSynchronize: false });
    const capacity = new ProviderCapacityPage(adminPage);
    await capacity.goto();

    await expect(
      adminPage.getByTestId("provider-capacity-sync-open"),
    ).toHaveCount(0);
    await expect(
      adminPage.getByTestId(
        "provider-capacity-sync-channel-channel_mofang_prod",
      ),
    ).toHaveCount(0);
    await expect(capacity.operationRow("魔方云生产渠道")).toBeVisible();
  });
});
