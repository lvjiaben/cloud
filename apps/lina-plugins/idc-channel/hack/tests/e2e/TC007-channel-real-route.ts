import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

test.describe("TC007 下游渠道真实入口", () => {
  test("TC007-1: 已登录管理员可以打开真实页面并读取渠道接口", async ({
    adminPage,
  }) => {
    const failedRequests: string[] = [];
    adminPage.on("response", (response) => {
      if (
        response.url().includes("/x/idc-channel/api/v1/") &&
        response.status() >= 400
      ) {
        failedRequests.push(`${response.status()} ${response.url()}`);
      }
    });

    await adminPage.goto("/admin/idc/channel", {
      waitUntil: "domcontentloaded",
    });

    const root = adminPage.getByTestId("idc-channel-admin-page");
    await expect(root).toBeVisible({ timeout: 15_000 });
    await expect(root).toContainText("下游渠道运营台");
    await expect(root).not.toContainText("暂时无法读取渠道数据");
    await expect(root).not.toContainText("当前账号已无权访问");
    expect(failedRequests).toEqual([]);
  });
});
