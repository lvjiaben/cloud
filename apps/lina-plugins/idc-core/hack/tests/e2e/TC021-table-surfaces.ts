import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerAssistancePage } from "../pages/CustomerAssistancePage";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { CustomerSupportPage } from "../pages/CustomerSupportPage";
import { TableSurfacePage } from "../pages/TableSurfacePage";
import { installCustomerSupportListMock } from "../support/customer-support-list-mock";

function contrast(foreground: string, background: string) {
  const luminance = (color: string) => {
    const channels = color.match(/[\d.]+/gu)!.slice(0, 3).map((value) => {
      const channel = Number(value) / 255;
      return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    });
    return channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722;
  };
  const a = luminance(foreground);
  const b = luminance(background);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

function expectWhite(results: Awaited<ReturnType<TableSurfacePage["inspect"]>>) {
  expect(results.length).toBeGreaterThan(0);
  for (const grid of results) {
    expect(grid.background).toBe("rgb(255, 255, 255)");
    expect(grid.areas.length).toBeGreaterThan(0);
    for (const area of grid.areas) {
      if (area.selector.includes("header")) {
        expect(area.color, area.selector).toBe(grid.headerSurface);
        expect(area.color, area.selector).not.toBe("rgb(255, 255, 255)");
      } else {
        expect(area.color, area.selector).toBe("rgb(255, 255, 255)");
      }
    }
    expect(grid.header).not.toBe(grid.layout);
    expect(grid.hover).not.toBe(grid.layout);
    expect(grid.selected).not.toBe(grid.layout);
    expect(grid.overflow).toBeLessThanOrEqual(1);
  }
}

test.describe("TC-21 表格白色面板与页面底色隔离", () => {
  test("TC-21a: 后台灰色表头与白色表体、空态、工具栏、分页和固定列", async ({ adminPage }) => {
    const surfaces = new TableSurfacePage(adminPage);
    for (const path of [
      "/idc/resource/provider-configuration",
      "/idc/resource/provider-capacity",
      "/idc/orders",
      "/idc/customers",
    ]) {
      await surfaces.gotoAdmin(path);
      await surfaces.theme("light");
      await surfaces.movePointerAway();
      await expect.poll(async () => (await surfaces.inspect())[0]?.background).toBe("rgb(255, 255, 255)");
      expectWhite(await surfaces.inspect());
    }
    const before = (await surfaces.inspect())[0]!;
    await surfaces.theme("dark");
    await expect.poll(async () => (await surfaces.inspect())[0]?.background).not.toBe("rgb(255, 255, 255)");
    const dark = (await surfaces.inspect())[0]!;
    expect(dark.header).not.toBe(dark.layout);
    await surfaces.theme("light");
    expect((await surfaces.inspect())[0]!.outer).toBe(before.outer);
    expectWhite(await surfaces.inspect());
  });

  test("TC-21b: 客户表体、搜索、工具栏、分页与空态全白且深色模式可用", async ({ adminPage }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");
    const popup = await customers.startAssistance("TC021 表格面板颜色回归");
    try {
      await new CustomerAssistancePage(popup).waitUntilReady();
      await installCustomerSupportListMock(popup);
      const support = new CustomerSupportPage(popup);
      const surfaces = new TableSurfacePage(popup);
      await support.goto();
      await expect(support.rows).toHaveCount(10);
      await surfaces.theme("light");
      await surfaces.movePointerAway();
      expectWhite(await surfaces.inspect());
      const outer = (await surfaces.inspect())[0]!.outer;
      expect(outer).toBe("214 28% 94%");
      await support.screenshot("customer-table-white-surfaces");
      await support.search("不存在的工单");
      await expect(support.rows).toHaveCount(0);
      await surfaces.movePointerAway();
      expectWhite(await surfaces.inspect());
      await surfaces.theme("dark");
      const dark = (await surfaces.inspect())[0]!;
      expect(dark.background).not.toBe("rgb(255, 255, 255)");
      expect(dark.header).not.toBe(dark.layout);
      expect(contrast(dark.emptyColor, dark.background)).toBeGreaterThanOrEqual(4.5);
      await support.screenshot("customer-table-dark-surfaces");
      await surfaces.theme("light");
      await support.resizeMobile();
      expectWhite(await surfaces.inspect());
      expect((await surfaces.inspect())[0]!.outer).toBe(outer);
      await support.screenshot("customer-table-white-mobile");
    } finally {
      await popup.close();
    }
  });
});
