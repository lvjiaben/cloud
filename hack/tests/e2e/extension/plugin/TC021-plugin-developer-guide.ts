import { mkdir } from "node:fs/promises";
import path from "node:path";

import { test, expect, type Page } from "../../../fixtures/auth";
import { PluginPage } from "../../../pages/PluginPage";

async function captureEvidence(page: Page, name: string) {
  // Let the drawer and collapse transitions finish so the screenshot records
  // the stable text contrast and layout instead of an animation frame.
  await page.waitForTimeout(500);
  const now = new Date();
  const stamp = now
    .toLocaleString("sv-SE", { hour12: false, timeZone: "Asia/Shanghai" })
    .replaceAll("-", "")
    .replaceAll(":", "")
    .replace(" ", "-");
  const day = now
    .toLocaleDateString("sv-SE", { timeZone: "Asia/Shanghai" })
    .replaceAll("-", "");
  const dir = path.resolve(process.cwd(), "..", "..", "temp", day);
  await mkdir(dir, { recursive: true });
  await page.screenshot({
    fullPage: false,
    path: path.join(dir, `${stamp}-${name}.png`),
  });
}

test.describe("TC-21 插件开发指南", () => {
  test("TC-21a: 后台插件管理可展开完整中文开发指南", async ({ adminPage }) => {
    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await captureEvidence(adminPage, "plugin-management-guide-entry-zh");
    await pluginPage.openDeveloperGuide();
    await expect(pluginPage.developerGuide).not.toContainText(
      "pages.system.plugin.guide",
    );
    await captureEvidence(adminPage, "plugin-developer-guide-zh-open");

    await expect(pluginPage.developerGuide).toContainText("从零开始");
    await expect(pluginPage.developerGuide).toContainText("开发动态 WASM 插件");
    await expect(pluginPage.developerGuide).toContainText(
      "具体业务配置仍回到对应业务页面",
    );
    await expect(pluginPage.developerGuide).toContainText("通用商品履约");
    await pluginPage.openDeveloperGuideSection("2. 分开清单四个维度");
    await expect(pluginPage.developerGuide).toContainText(
      "业务扩展点和前端表面",
    );
    await pluginPage.openDeveloperGuideSection("9. 正确设计接口实例");
    await expect(pluginPage.developerGuide).toContainText(
      "不限制插件自有的新闻、订单、证书",
    );
    await pluginPage.openDeveloperGuideSection("11. 把秘密留在保密出站通道中");
    await expect(pluginPage.developerGuide).toContainText("禁止把 API Key");
    await pluginPage.openDeveloperGuideSection("13. 构建 WASM 插件包");
    await expect(pluginPage.developerGuide).toContainText("linactl wasm");
    await expect(pluginPage.developerGuide).toContainText("必须遵守的边界");

    const metrics = await pluginPage.developerGuide.evaluate((element) => {
      const dialog = element.closest('[role="dialog"]');
      const rect = dialog?.getBoundingClientRect();
      return {
        bodyScrollbars:
          document.documentElement.scrollWidth > window.innerWidth,
        dialogWidth: rect?.width ?? 0,
        viewportWidth: window.innerWidth,
      };
    });
    expect(metrics.bodyScrollbars).toBeFalsy();
    expect(metrics.dialogWidth).toBeGreaterThan(600);
    expect(metrics.dialogWidth).toBeLessThanOrEqual(metrics.viewportWidth);
    await captureEvidence(adminPage, "plugin-developer-guide-zh");
  });

  test("TC-21b: 切换英文后指南同步翻译且窄屏不横向溢出", async ({
    adminPage,
  }) => {
    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await adminPage.getByTestId("language-toggle-trigger").click();
    await adminPage.getByRole("menuitem", { name: "English" }).click();
    await adminPage.setViewportSize({ height: 844, width: 768 });
    await pluginPage.openDeveloperGuide();

    await expect(pluginPage.developerGuide).toContainText("Start from scratch");
    await expect(pluginPage.developerGuide).toContainText(
      "Build a dynamic WASM plugin",
    );
    await expect(pluginPage.developerGuide).toContainText(
      "hidden built-in foundations",
    );
    await pluginPage.openDeveloperGuideSection(
      "2. Keep four manifest dimensions separate",
    );
    await pluginPage.openDeveloperGuideSection(
      "9. Model interface instances correctly",
    );
    await expect(pluginPage.developerGuide).toContainText("singleton");
    await pluginPage.openDeveloperGuideSection("8. Use the secure page bridge");
    await expect(pluginPage.developerGuide).toContainText(
      "receives neither the host access token",
    );
    await expect(pluginPage.developerGuide).toContainText(
      "Mandatory boundaries",
    );
    await expect
      .poll(() =>
        pluginPage.developerGuide.evaluate(
          (element) => element.scrollWidth <= element.clientWidth + 1,
        ),
      )
      .toBe(true);
    await captureEvidence(adminPage, "plugin-developer-guide-en-narrow");
  });
});
