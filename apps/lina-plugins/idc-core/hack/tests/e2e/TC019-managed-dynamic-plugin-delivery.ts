import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import {
  createAdminApiContext,
  ensureSourcePluginEnabledViaAPI,
  findPlugin,
  syncPlugins,
} from "../../../../../../hack/tests/fixtures/plugin";

const requiredDynamicPlugins = [
  "idc-payment-alipay",
  "idc-payment-epay",
  "idc-payment-unionpay",
  "idc-payment-wechatpay",
  "idc-zjmfmigrate",
  "yunsu-ssl",
] as const;

test.describe("TC019 托管动态插件交付与页面", () => {
  test("TC019-1: 关键动态插件安装启用后同步不会把状态重置", async () => {
    const adminApi = await createAdminApiContext();
    try {
      for (const pluginID of requiredDynamicPlugins) {
        await ensureSourcePluginEnabledViaAPI(adminApi, pluginID);
      }
      await syncPlugins(adminApi);

      for (const pluginID of requiredDynamicPlugins) {
        const plugin = await findPlugin(adminApi, pluginID);
        expect(plugin, `未发现动态插件 ${pluginID}`).toBeTruthy();
        expect(plugin?.type, `${pluginID} 类型错误`).toBe("dynamic");
        expect(plugin?.distribution, `${pluginID} 分发方式错误`).toBe(
          "managed",
        );
        expect(plugin?.installed, `${pluginID} 未安装`).toBe(1);
        expect(plugin?.enabled, `${pluginID} 未启用`).toBe(1);
      }
    } finally {
      await adminApi.dispose();
    }
  });

  test("TC019-2: 通知公告内置源码页面可达、紧凑且使用内部滚动", async ({
    adminPage,
  }) => {
    await adminPage.goto("/system/notice", {
      waitUntil: "domcontentloaded",
    });
    await expect(adminPage.locator("body")).not.toContainText("插件页面未找到");
    await expect(
      adminPage.locator('[data-testid="plugin-sandbox-frame"]'),
    ).toHaveCount(0);
    const root = adminPage.getByTestId("notice-management-page");
    await expect(root).toBeVisible({ timeout: 15_000 });
    const layout = await root.evaluate((node) => {
      const table = node.querySelector<HTMLElement>(".vxe-grid")!;
      return {
        pageOverflow:
          document.documentElement.scrollWidth -
          document.documentElement.clientWidth,
        tableHeight: Math.round(table.getBoundingClientRect().height),
      };
    });
    expect(layout.pageOverflow).toBeLessThanOrEqual(1);
    expect(layout.tableHeight).toBeGreaterThanOrEqual(260);
    expect(layout.tableHeight).toBeLessThanOrEqual(680);

    await root.getByTestId("notice-add").click();
    const dialog = adminPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByPlaceholder("请输入公告标题")).toBeVisible();
    await expect(
      dialog.locator('.tiptap[contenteditable="true"]'),
    ).toBeVisible();
  });
});
