import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { SSLCustomerPage } from "../pages/SSLCustomerPage";
import { installSSLCustomerSurfaceMock } from "../support/customer-surface-mock";

async function screenshot(page: import("@playwright/test").Page, name: string) {
  await page
    .locator("#__app-loading__")
    .waitFor({ state: "hidden", timeout: 10_000 });
  await page
    .locator(".bg-overlay-content:visible")
    .waitFor({ state: "hidden", timeout: 10_000 });
  await page
    .locator(".ant-notification-notice:visible")
    .waitFor({ state: "hidden", timeout: 10_000 });
  const directory = resolve(
    process.cwd(),
    "../../temp",
    new Date().toISOString().slice(0, 10).replaceAll("-", ""),
  );
  await mkdir(directory, { recursive: true });
  await page.screenshot({
    fullPage: false,
    path: resolve(
      directory,
      `${new Date().toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`,
    ),
  });
}

test.describe("TC-1 SSL客户证书动态页面", () => {
  test('TC-1g: 表格运行时首次失败后点击原有重试按钮恢复，不刷新页面或重放业务动作', async ({page}) => {
    const state = await installSSLCustomerSurfaceMock(page, {failFirstRuntime:true});
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    await ssl.open(true);
    const url = page.url();
    await expect(ssl.frame().getByTestId('ssl-error')).toBeVisible();
    expect(state.runtimeRequests()).toBe(1);
    await ssl.retry();
    await expect(ssl.frame().locator('.vxe-grid')).toHaveCount(1);
    await expect(ssl.frame().getByText('example.test',{exact:true})).toBeVisible();
    await expect(ssl.frame().getByTestId('ssl-error')).toBeHidden();
    expect(page.url()).toBe(url);
    expect(state.runtimeRequests()).toBe(2);
    expect(state.actionRequests()).toHaveLength(0);
    await screenshot(page, 'yunsu-ssl-runtime-retry');
  });

  test("TC-1a: 慢资源首次进入显示宿主骨架且加载后没有白屏、双页头或双滚动条", async ({
    page,
  }) => {
    await installSSLCustomerSurfaceMock(page, { slowAssetMs: 800 });
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    const group = page
      .locator("li.vben-sub-menu")
      .filter({ hasText: "产品与服务" })
      .first();
    await group.waitFor({ state: "visible", timeout: 15_000 });
    // Desktop customer sections are always expanded and cannot be clicked.
    await group.getByText("SSL 证书", { exact: true }).click();
    await expect(page.getByTestId("plugin-sandbox-skeleton")).toBeVisible();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId("plugin-sandbox-skeleton")).toHaveCount(0);
    await expect(ssl.frame().locator('.vxe-grid')).toHaveCount(1);
    expect(await ssl.frame().locator('.vxe-table--header-wrapper').first().evaluate(element => getComputedStyle(element).backgroundColor)).not.toBe('rgb(255, 255, 255)');
    expect(await ssl.frame().locator('.vxe-table--body-wrapper').first().evaluate(element => getComputedStyle(element).backgroundColor)).toBe('rgb(255, 255, 255)');
    await expect(
      page.locator('iframe[data-testid="plugin-sandbox-frame"]'),
    ).toHaveCount(1);
    await expect(ssl.frame().locator("h1")).toHaveCount(1);
    expect(
      await ssl
        .frame()
        .locator("html")
        .evaluate((element) => element.scrollWidth - element.clientWidth),
    ).toBeLessThanOrEqual(1);
    await screenshot(page, "yunsu-ssl-customer-first-load");
  });

  test("TC-1b: 再次进入、英文、暗色、窄屏和刷新保持同一宿主体验", async ({
    page,
  }) => {
    const state = await installSSLCustomerSurfaceMock(page);
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    await ssl.open();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    const firstColdReads = state.coldAssetReads();
    await page.getByText("客户概览", { exact: true }).first().click();
    await page.waitForURL(/\/account$/u);
    await ssl.open();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    expect(state.coldAssetReads()).toBe(firstColdReads);
    await page.getByTestId("language-toggle-trigger").click();
    await page.getByText("English", { exact: true }).click();
    await expect(
      ssl.frame().getByText("SSL certificates", { exact: true }),
    ).toBeVisible();
    const mode = await ssl.frame().locator("html").getAttribute("data-theme");
    await page
      .getByRole("button", { name: mode === "dark" ? "light" : "dark" })
      .click();
    await expect(ssl.frame().locator("html")).toHaveAttribute(
      "data-theme",
      mode === "dark" ? "light" : "dark",
    );
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
    await ssl.frame().getByRole("button", { name: "Refresh" }).click();
    await expect(ssl.frame().getByTestId('ssl-loading')).toBeHidden();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    await expect(
      ssl.frame().getByRole("button", { name: "View details" }),
    ).toBeVisible();
    await screenshot(page, "yunsu-ssl-customer-en-dark-narrow");
    await page.reload();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    await expect(page.locator("#__app-loading__")).toBeHidden();
  });

  test("TC-1c: 停止新业务仍保留已有证书入口，停用后权限刷新移除入口", async ({
    page,
  }) => {
    const state = await installSSLCustomerSurfaceMock(page);
    state.stopNewBusiness();
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    await ssl.open();
    await expect(
      ssl.frame().getByText("example.test", { exact: true }),
    ).toBeVisible();
    state.disable();
    await page.reload();
    await expect(
      page.getByText(/SSL 证书|SSL Certificates/u, { exact: true }),
    ).toHaveCount(0);
  });

  test("TC-1d: 客户可查看验证资料并通过受治理动作切换验证、重新验证和取消", async ({
    page,
  }) => {
    const state = await installSSLCustomerSurfaceMock(page);
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    await ssl.open();
    await ssl.viewDetails();
    await expect(
      ssl.frame().getByText("_dnsauth.example.test", { exact: true }),
    ).toBeVisible();
    await expect(
      ssl.frame().getByText("validation.example.test", { exact: true }),
    ).toBeVisible();
    await screenshot(page, "yunsu-ssl-customer-detail");
    await ssl.changeValidationToFile();
    await expect.poll(() => state.actionRequests().length).toBe(1);
    await ssl.revalidate();
    await expect.poll(() => state.actionRequests().length).toBe(2);
    await ssl.cancelApplication();
    await expect.poll(() => state.actionRequests().length).toBe(3);
    const actions = state.actionRequests();
    expect(actions.map((item) => item.path.split("/").at(-1))).toEqual([
      "update-validation",
      "revalidate",
      "cancel",
    ]);
    expect(actions[0]?.body).toMatchObject({
      input: { validationMethod: "file" },
    });
    expect(JSON.stringify(actions)).not.toMatch(
      /authorization|bearer|accessToken|api_secret/iu,
    );
  });

  test("TC-1f: 六十条不同状态证书继续使用真实Vben表格且游标加载不撑高页面", async ({page}) => {
    const state = await installSSLCustomerSurfaceMock(page,{pagedRecords:true});
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    const started = Date.now();
    await ssl.open();
    const coldEntryMs = Date.now()-started;
    const grid = ssl.frame().locator('.vxe-grid');
    await expect(grid).toHaveCount(1);
    await expect(ssl.frame().getByTestId('ssl-summary')).toHaveText('共 30 张证书');
    const before = await grid.boundingBox();
    const pagingStarted = Date.now();
    await ssl.frame().getByTestId('ssl-load-more').click();
    await expect(ssl.frame().getByTestId('ssl-summary')).toHaveText('共 60 张证书');
    const after = await grid.boundingBox();
    expect(Math.abs((after?.height ?? 0)-(before?.height ?? 0))).toBeLessThanOrEqual(1);
    expect(after?.height).toBeLessThanOrEqual(481);
    await expect(ssl.frame().getByTestId('ssl-load-more-wrap')).toBeHidden();
    expect(state.listRequests()).toEqual([{cursor:'',limit:'30'},{cursor:'ssl-second-page',limit:'30'}]);
    expect(await ssl.frame().locator('html').evaluate(element => element.scrollWidth-element.clientWidth)).toBeLessThanOrEqual(1);
    console.log(JSON.stringify({sample:'SSL bounded cursor table',coldEntryMs,pageAppendMs:Date.now()-pagingStarted,records:60,height:after?.height}));
    await screenshot(page,'yunsu-ssl-vben-sixty-records');
  });

  test("TC-1e: 已签发证书只通过短期票据下载且插件页面拿不到登录令牌", async ({
    page,
  }) => {
    const state = await installSSLCustomerSurfaceMock(page);
    state.issueCertificate();
    const ssl = new SSLCustomerPage(page);
    await ssl.login();
    await ssl.open();
    const download = await ssl.downloadCertificate();
    expect(download.suggestedFilename()).toBe("example-test.zip");
    await expect.poll(() => state.downloadRequests().length).toBe(2);
    expect(state.downloadRequests()[0]).toContain(
      "/downloads/certificate/tickets",
    );
    expect(state.downloadRequests()[1]).toContain(
      `/service-downloads/${"A".repeat(40)}`,
    );
    expect(JSON.stringify(state.downloadRequests())).not.toMatch(
      /authorization|bearer|accessToken|api_secret/iu,
    );
  });
});
