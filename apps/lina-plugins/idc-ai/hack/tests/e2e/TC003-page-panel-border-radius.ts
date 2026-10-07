import type { Locator, Page } from "@host-tests/support/playwright";

import { expect, test } from "@host-tests/fixtures/auth";
import {
  createAdminApiContext,
  findPlugin,
  prepareSourcePluginsBaseline,
  syncPlugins,
} from "@host-tests/fixtures/plugin";
import { workspacePath } from "@host-tests/fixtures/config";
import { ConfigPage } from "@host-tests/pages/ConfigPage";
import { waitForRouteReady } from "@host-tests/support/ui";

const aiPluginId = "idc-ai";

async function readBorderRadius(locator: Locator): Promise<string> {
  await expect(locator).toBeVisible();
  return locator.evaluate((element) => getComputedStyle(element).borderRadius);
}

function gridPanel(page: Page) {
  return page
    .locator('[id="__vben_main_content"] .vxe-grid')
    .first()
    .locator("xpath=..");
}

async function isAiPluginAvailable(): Promise<boolean> {
  const adminApi = await createAdminApiContext();
  try {
    await syncPlugins(adminApi);
    return Boolean(await findPlugin(adminApi, aiPluginId));
  } finally {
    await adminApi.dispose();
  }
}

test.describe("TC003 AI 页面与宿主方角契约", () => {
  test("TC003a: 宿主版本卡片和列表板块统一使用方角", async ({ adminPage }) => {
    await adminPage.goto("/about/system-info");
    await waitForRouteReady(adminPage);
    const content = adminPage.locator('[id="__vben_main_content"]');
    const firstCard = content.locator(".card-box").first();
    expect(await readBorderRadius(firstCard)).toBe("0px");

    const configPage = new ConfigPage(adminPage);
    await configPage.goto();
    const gridRadius = await readBorderRadius(gridPanel(adminPage));
    expect(gridRadius).toBe("0px");
  });

  test("TC003b: Ant Card 表单板块统一使用方角", async ({ adminPage }) => {
    await adminPage.goto("/system/message");
    await waitForRouteReady(adminPage);
    const antCard = adminPage
      .locator('[id="__vben_main_content"] .ant-card')
      .first();
    expect(await readBorderRadius(antCard)).toBe("0px");
  });

  test("TC003c: AI 运营助手板块与宿主统一使用方角", async ({ adminPage }) => {
    test.skip(
      !(await isAiPluginAvailable()),
      "idc-ai is unavailable in host-only environments",
    );
    await prepareSourcePluginsBaseline([aiPluginId]);
    await adminPage.goto(workspacePath("/idc/ai-copilot"));
    await waitForRouteReady(adminPage);
    const invocationPanel = adminPage.getByTestId("ai-copilot-panel").first();
    expect(await readBorderRadius(invocationPanel)).toBe("0px");
  });
});
