import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { SSLAdminPage } from "../pages/SSLAdminPage";
import { installSSLAdminSurfaceMock } from "../support/admin-surface-mock";

test.describe("TC-2 SSL后台证书动态页面", () => {
  test("TC-2a: 管理员可在隔离页面查看客户证书与验证资料", async ({ authenticatedPage: page }) => {
    await installSSLAdminSurfaceMock(page); const ssl = new SSLAdminPage(page); await ssl.open();
    await expect(ssl.frame().locator('.vxe-grid')).toHaveCount(1);
    expect(await ssl.frame().locator('.vxe-table--header-wrapper').first().evaluate(element => getComputedStyle(element).backgroundColor)).not.toBe('rgb(255, 255, 255)');
    expect(await ssl.frame().locator('.vxe-table--body-wrapper').first().evaluate(element => getComputedStyle(element).backgroundColor)).toBe('rgb(255, 255, 255)');
    await expect(ssl.frame().locator('.vxe-table--main-wrapper .vxe-table--body-wrapper').getByText("示例客户", { exact: true })).toBeVisible();
    await ssl.viewDetails();
    await expect(ssl.frame().getByText("_dnsauth.admin-example.test", { exact: true })).toBeVisible();
    await expect(page.locator('iframe[data-testid="plugin-sandbox-frame"]')).toHaveCount(1);
    await expect(ssl.frame().locator("h1")).toHaveCount(1);
    expect(await ssl.frame().locator("html").evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    const menuLoading = page.getByText(/加载菜单中|Loading menu/u).first();
    await menuLoading.waitFor({ state: "visible", timeout: 3_000 }).catch(() => undefined);
    await menuLoading.waitFor({ state: "hidden", timeout: 10_000 });
    const directory = resolve(process.cwd(), "../../temp", new Date().toISOString().slice(0, 10).replaceAll("-", ""));
    await mkdir(directory, { recursive: true });
    await page.screenshot({ path: resolve(directory, `${new Date().toISOString().slice(11, 19).replaceAll(":", "")}-yunsu-ssl-admin-detail.png`) });
  });
});
