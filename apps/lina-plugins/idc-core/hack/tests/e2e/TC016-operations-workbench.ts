import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { OperationsDataPage } from "../pages/OperationsDataPage";

test.describe("TC-16 运营数据真实接口回归", () => {
  test("TC-16a: 真实汇总接口与页面一致且一级菜单显示运营数据", async ({
    adminPage,
  }) => {
    const view = new OperationsDataPage(adminPage);
    const responsePromise = adminPage.waitForResponse((response) =>
      /\/admin\/operations\/data(?:\?|$)/u.test(response.url()),
    );
    await view.goto();
    const response = await responsePromise;
    expect(response.ok()).toBe(true);
    const payload = await response.json();
    expect(payload.code).toBe(0);
    const result = payload.data;
    expect(result.periods).toHaveLength(4);
    expect(result.trend.length).toBeGreaterThanOrEqual(28);
    expect(result.trend.length).toBeLessThanOrEqual(31);
    await expect(view.root()).toBeVisible();
    await expect(adminPage.getByRole("menuitem", { name: "运营数据", exact: true })).toBeVisible();
    for (const period of result.periods) {
      const amount = BigInt(period.incomeMinor);
      const scale = 10n ** BigInt(result.currencyExponent);
      const integer = new Intl.NumberFormat("zh-CN").format(amount / scale);
      const fraction = result.currencyExponent
        ? `.${String(amount % scale).padStart(result.currencyExponent, "0")}`
        : "";
      await expect(
        view.period(period.key).locator(".income-link strong"),
      ).toHaveText(`${integer}${fraction}`);
      await expect(
        view.period(period.key).locator(".orders-link strong"),
      ).toHaveText(new Intl.NumberFormat("zh-CN").format(period.paidOrders));
    }
    await expect(
      adminPage.getByText("运营工作台", { exact: true }),
    ).toHaveCount(0);
    await expect(adminPage.getByText("工作台", { exact: true })).toHaveCount(0);
    await expect(view.root()).not.toContainText("plugin.idc-core.");
    await view.screenshot("real-loaded");
    await view
      .root()
      .getByRole("heading", { name: "本月商品销量排行", exact: true })
      .scrollIntoViewIfNeeded();
    await view.screenshot("real-services");
  });
});
