// 只在独立数据库创建专用商品，检查选配清空后仍使用服务端首个可售值与报价。
import { randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect } from "@host-tests/support/playwright";
import { api, descriptor, elasticProduct, webFixture } from "../support/isolated-web-commerce";
import { LiveCommercePage } from "../pages/LiveCommercePage";

test.describe("TC-3 数量选配首值", () => {
  test.skip(!descriptor,"Requires the explicit isolated local commerce database");
  test("TC-3a: 数量首次显示及清空都采用首个可售值，重新选择后按实际数量核价", async ({browser}) => {
    const f=await webFixture(), admin=await api(f,f.adminAuth);
    const slug=await elasticProduct(admin,"quantity-default-"+randomUUID());
    const context=await browser.newContext();
    context.setDefaultTimeout(15_000);
    const allowed=new Set([f.websiteOrigin,f.customerOrigin,f.rendererOrigin,f.baseURL,f.mockOrigin]);
    await context.route("**/*",r=>allowed.has(new URL(r.request().url()).origin)?r.continue():r.abort());
    try {
      const page=await context.newPage(),view=new LiveCommercePage(page,f.websiteOrigin);
      await view.open(slug);await view.selectOS();
      const cpu=view.configurator.locator(".ant-form-item").filter({hasText:"处理器核心数"}).getByRole("spinbutton");
      const memory=page.getByRole("spinbutton",{name:"内存（GiB）",exact:true});
      await expect(cpu).toHaveValue("1");await expect(memory).toHaveValue("1");
      await expect(view.configurator.getByTestId("purchase-plan-default")).toHaveCount(0);
      await expect(memory).not.toHaveAttribute("placeholder",/套餐默认/);
      await view.quote();await expect(view.total).toContainText("12.50");
      await cpu.fill("4");await memory.fill("2");await memory.blur();
      await view.quote();await expect(view.total).toContainText("16.00");
      await cpu.fill("");await memory.fill("");await memory.blur();
      await view.screenshot("optional-quantity-cleared");
      await view.quote();await expect(view.total).toContainText("12.50");
      await expect(cpu).toHaveValue("1");await expect(memory).toHaveValue("1");
      await expect(view.configurator.getByTestId("purchase-selected-configuration")).toContainText("内存（GiB）");
      await cpu.fill("4");await memory.fill("2");await memory.blur();
      await view.quote();await expect(view.total).toContainText("16.00");
      await view.screenshot("optional-quantity-selected-again");
    } finally {await context.close();await admin.dispose();}
  });
});
