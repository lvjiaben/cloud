import { mkdir } from "node:fs/promises";
import path from "node:path";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { PortalBrandPage } from "../pages/PortalBrandPage";
import { installWebsiteMessages } from "../support/website-i18n";

const common = {
  siteName: "测试官网",
  siteTitle: "测试标题",
  companyName: "测试公司",
  contactEmail: "help@example.test",
  contactPhone: "",
  logoUrl: "",
  description: "",
};
const plugins = [
  {
    pluginId: "website-carousel",
    name: "轮播官网",
    schema: {
      version: "v1",
      fields: [
        {
          key: "herotitle",
          type: "single_line",
          label: { zhCN: "首页标题", enUS: "Hero title" },
          required: true,
        },
      ],
    },
  },
  {
    pluginId: "website-menu",
    name: "菜单官网",
    schema: {
      version: "v1",
      fields: [
        {
          key: "menutext",
          type: "multi_line",
          label: { zhCN: "官网菜单", enUS: "Website menu" },
          description: { zhCN: "每行一个：商品|/products", enUS: "One per line: Products|/products" },
        },
      ],
    },
  },
];
async function screenshot(portal: PortalBrandPage, suffix: string) {
  const dir = path.resolve(process.cwd(), "../../temp/20261016");
  await mkdir(dir, { recursive: true });
  await portal.page.screenshot({
    path: path.join(dir, `${Date.now()}-website-${suffix}.png`),
    fullPage: true,
  });
}
test.describe("TC-1 官网配置", () => {
  test.beforeEach(async ({ adminPage }) => {
    await installWebsiteMessages(adminPage);
  });
  test("TC-1a: 插件只显示自身配置，一次保存启用并保留公共信息", async ({
    adminPage,
  }) => {
    const portal = new PortalBrandPage(adminPage);
    let state = {
      plugins,
      selectedPlugin: plugins[0].pluginId,
      common: structuredClone(common),
      config: { herotitle: "欢迎访问" } as Record<string, string>,
      configurations: {
        "website-carousel": { herotitle: "欢迎访问" },
        "website-menu": { menutext: "已有菜单|/help" },
      } as Record<string, Record<string, string>>,
    };
    let saves = 0;
    await adminPage.route(
      "**/x/idc-portal/api/v1/admin/site/configuration",
      async (route) => {
        if (route.request().method() === "PUT") {
          state = { ...state, ...route.request().postDataJSON() };
          state.configurations[state.selectedPlugin] = { ...state.config };
          saves++;
        }
        await route.fulfill({ json: { code: 0, message: "OK", data: state } });
      },
    );
    await portal.goto();
    await expect(
      portal.root.getByRole("heading", { name: "官网配置", exact: true }),
    ).toBeVisible();
    await expect(portal.field("网站名称")).toHaveValue("测试官网");
    await expect(portal.field("首页标题")).toHaveValue("欢迎访问");
    await portal.expectActivePlugin("轮播官网");
    await expect(portal.root).not.toContainText("plugin.idc-portal.");
    await expect(portal.root).not.toContainText("修订号");
    await screenshot(portal, "loaded");
    await portal.field("网站名称").fill("更新后的官网");
    await portal.root.getByRole("button", { name: "菜单官网" }).click();
    await expect(portal.field("首页标题")).toHaveCount(0);
    await expect(portal.field("官网菜单")).toHaveValue("已有菜单|/help");
    await expect(portal.field("官网菜单")).toHaveAttribute("placeholder", "每行一个：商品|/products");
    await portal.field("官网菜单").fill("产品|/products");
    await portal.expectActivePlugin("轮播官网");
    expect(saves).toBe(0);
    await portal.button("保存并启用").click();
    await expect(
      adminPage.getByText("官网配置已生效", { exact: true }),
    ).toBeVisible();
    expect(saves).toBe(1);
    expect(state.selectedPlugin).toBe("website-menu");
    expect(state.common.siteName).toBe("更新后的官网");
    expect(state.config).toEqual({ menutext: "产品|/products" });
    await portal.expectActivePlugin("菜单官网");
    await screenshot(portal, "saved");
    await portal.button("刷新").click();
    await expect(portal.field("官网菜单")).toHaveValue("产品|/products");
    await portal.root.getByRole("button", { name: "轮播官网" }).click();
    await expect(portal.field("首页标题")).toHaveValue("欢迎访问");
    await portal.expectActivePlugin("菜单官网");
    await adminPage.setViewportSize({ width: 390, height: 844 });
    await screenshot(portal, "mobile");
  });
  test("TC-1b: 加载失败可恢复，没装官网时不能启用", async ({ adminPage }) => {
    const portal = new PortalBrandPage(adminPage);
    let fail = true;
    await adminPage.route(
      "**/x/idc-portal/api/v1/admin/site/configuration",
      async (route) => {
        await route.fulfill({
          status: fail ? 500 : 200,
          json: fail
            ? { code: 500, message: "Test unavailable" }
            : {
                code: 0,
                message: "OK",
                data: { plugins: [], selectedPlugin: "", common, config: {} },
              },
        });
      },
    );
    await portal.goto();
    await expect(portal.root.locator(".website-error")).toContainText(
      "加载失败，请刷新重试。",
    );
    await expect(portal.button("保存并启用")).toBeDisabled();
    await screenshot(portal, "failure");
    fail = false;
    await portal.button("刷新").click();
    await expect(portal.root).toContainText("没有可用的官网插件");
    await expect(portal.root).toContainText("请先到插件管理安装并启用官网插件，然后回到这里刷新。");
    await expect(portal.root.getByRole("link", { name: "前往插件管理" })).toHaveAttribute("href", "/admin/system/plugin");
    await expect(portal.button("保存并启用")).toBeDisabled();
    await expect(portal.root.locator(".website-error")).toHaveCount(0);
    await screenshot(portal, "empty");
  });
  test("TC-1c: 保存失败保留填写内容供重试", async ({ adminPage }) => {
    const portal = new PortalBrandPage(adminPage);
    let fail = true;
    let savedName = "";
    await adminPage.route(
      "**/x/idc-portal/api/v1/admin/site/configuration",
      async (route) => {
        if (route.request().method() === "PUT" && fail) {
          await route.fulfill({
            status: 500,
            json: { code: 500, message: "保存失败，请重试" },
          });
          return;
        }
        if (route.request().method() === "PUT")
          savedName = route.request().postDataJSON().common.siteName;
        await route.fulfill({
          json: {
            code: 0,
            message: "OK",
            data: {
              plugins,
              selectedPlugin: plugins[0].pluginId,
              common,
              config: { herotitle: "欢迎" },
            },
          },
        });
      },
    );
    await portal.goto();
    await portal.field("网站名称").fill("需要保留的官网");
    await portal.selectPlugin("菜单官网");
    await portal.field("官网菜单").fill("失败后保留|/help");
    await portal.button("保存并启用").click();
    await expect(
      adminPage.getByText("保存失败，请重试", { exact: true }),
    ).toBeVisible();
    await expect(portal.field("网站名称")).toHaveValue("需要保留的官网");
    await expect(portal.field("官网菜单")).toHaveValue("失败后保留|/help");
    await portal.expectActivePlugin("轮播官网");
    fail = false;
    await portal.button("保存并启用").click();
    await expect(
      adminPage.getByText("官网配置已生效", { exact: true }),
    ).toBeVisible();
    expect(savedName).toBe("需要保留的官网");
    await portal.expectActivePlugin("菜单官网");
  });
  test("TC-1d: 系统配置删除旧官网入口", async ({ adminPage }) => {
    const portal = new PortalBrandPage(adminPage);
    await adminPage.goto("/admin/system/configuration-center");
    const center = adminPage.getByTestId("system-configuration-center");
    await expect(center).toBeVisible();
    await expect(
      center.getByRole("tab", { name: "业务设置", exact: true }),
    ).toBeVisible();
    await expect(center).not.toContainText("官网与用户中心品牌");
    await expect(center).not.toContainText("官网品牌");
    await screenshot(portal, "configuration-center");
  });
  test("TC-1e: 真实保存两个官网插件并投影到公开品牌", async ({ adminPage }) => {
    test.skip(process.env.E2E_WEBSITE_REAL !== "1", "仅在隔离安装实例中执行真实配置写入");
    const portal = new PortalBrandPage(adminPage);
    await portal.goto();
    const original = await portal.configuration();
    const standard = original.plugins.find((item) => item.pluginId === "website-default");
    const minimal = original.plugins.find((item) => item.pluginId === "website-minimal");
    expect(standard, "须安装启用简洁官网").toBeTruthy();
    expect(minimal, "须安装启用文字官网").toBeTruthy();
    const standardTitle = `官网回归简洁-${Date.now()}`;
    const minimalTitle = `官网回归文字-${Date.now()}`;
    try {
      await expect(portal.field("网站名称")).toBeVisible();
      await portal.selectPlugin(standard!.name);
      await portal.field("网站名称").fill("官网真实保存回归");
      await expect(portal.field("导航菜单")).toHaveAttribute("placeholder", "每行一个：商品|/products");
      await portal.field("首页标题").fill(standardTitle);
      await portal.field("首页图片地址").fill("https://example.test/banner.png");
      await portal.save();
      const saved = await portal.configuration();
      expect(saved.config.herotitle).toBe(standardTitle);
      expect(saved.config.heroimage).toBe("https://example.test/banner.png");
      const brand = await portal.publicBrand();
      expect(brand.templateRenderer).toBe("public-standard-v1");
      expect(brand.heroTitle.zhCN).toBe(standardTitle);
      expect(brand.heroImage).toBe("https://example.test/banner.png");
      expect(brand.siteName.zhCN).toBe("官网真实保存回归");
      await screenshot(portal, "real-standard-saved");
      await portal.selectPlugin(minimal!.name);
      const beforeSave = await portal.publicBrand();
      expect(beforeSave.templateRenderer).toBe("public-standard-v1");
      expect(beforeSave.heroTitle.zhCN).toBe(standardTitle);
      await expect(portal.field("首页图片地址")).toHaveCount(0);
      await portal.field("首页标题").fill(minimalTitle);
      await portal.save();
      const minimalBrand = await portal.publicBrand();
      expect(minimalBrand.templateRenderer).toBe("public-minimal-v1");
      expect(minimalBrand.heroTitle.zhCN).toBe(minimalTitle);
      await portal.button("刷新").click();
      await expect(portal.field("网站名称")).toHaveValue("官网真实保存回归");
      await expect(portal.field("首页标题")).toHaveValue(minimalTitle);
      await portal.selectPlugin(standard!.name);
      await expect(portal.field("首页标题")).toHaveValue(standardTitle);
      await expect(portal.field("首页图片地址")).toHaveValue("https://example.test/banner.png");
      await portal.expectActivePlugin(minimal!.name);
      await portal.save();
      await portal.expectActivePlugin(standard!.name);
      const returned = await portal.publicBrand();
      expect(returned.templateRenderer).toBe("public-standard-v1");
      expect(returned.heroTitle.zhCN).toBe(standardTitle);
      expect(returned.heroImage).toBe("https://example.test/banner.png");
      const stillInstalled = await portal.configuration();
      expect(stillInstalled.plugins.map((item) => item.pluginId)).toEqual(
        expect.arrayContaining([standard!.pluginId, minimal!.pluginId]),
      );
      await screenshot(portal, "real-independent-restored");
    } finally {
      for (const plugin of [standard!, minimal!]) {
        await portal.restoreConfiguration({
          selectedPlugin: plugin.pluginId,
          common: original.common,
          config: original.configurations?.[plugin.pluginId] ?? {},
        });
      }
      await portal.restoreConfiguration({
        selectedPlugin: original.selectedPlugin,
        common: original.common,
        config: original.config,
      });
      const restored = await portal.configuration();
      expect(restored.selectedPlugin).toBe(original.selectedPlugin);
      expect(restored.common).toEqual(original.common);
      expect(restored.config).toEqual(original.config);
    }
  });

});
