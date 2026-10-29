import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { test, expect } from "../support/catalog-auth";
import { CustomerOptionsPage } from "../pages/CustomerOptionsPage";

// Explicit local integration mode: use a new, unpublished copy and clean only
// that owned fixture. The user's source product is fingerprinted before/after.
test.describe("TC-60 真实客户配置保存", () => {
  test.skip(
    !process.env.IDC_CATALOG_REAL_SOURCE_PRODUCT,
    "Set an explicit local source product for the isolated copy smoke test",
  );
  test("TC-60a: 真实副本修改内存下拉子项、回填并保存配置价格，原商品不变", async ({
    adminPage,
  }) => {
    test.setTimeout(120000);
    const source = process.env.IDC_CATALOG_REAL_SOURCE_PRODUCT!;
    const slug = "e2e-local-config-" + randomUUID().replaceAll("-", "");
    const database = fileURLToPath(
      new URL("../support/catalog-fixture-database.py", import.meta.url),
    );
    execFileSync("python3", [database, "probe", source], { encoding: "utf8" });
    const before = execFileSync("python3", [database, "fingerprint", source], {
      encoding: "utf8",
    }).trim();
    const workspace = new CustomerOptionsPage(adminPage);
    try {
      await workspace.goto();
      await workspace.openCopy(source);
      await workspace.fillCopy(
        "E2E配置副本 " + slug,
        "E2E local configuration copy",
        slug,
      );
      const copied = adminPage.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.url().endsWith(`/products/${source}/copies`),
      );
      await workspace.confirmCopy();
      const result = await (await copied).json();
      expect(result.code).toBe(0);
      const product = result.data.productNo;
      await expect(workspace.copyModal).toBeHidden();
      await workspace.openProductDetail(product);
      await workspace.openOptions();
      const memoryButton = workspace.optionsForm.getByRole("button", {
        name: "内存（GiB）",
        exact: true,
      });
      await memoryButton.click();
      const control = workspace.optionsForm.locator(
        '[data-testid$="_control"]',
      );
      await control.click();
      await adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText("下拉选择", { exact: true })
        .click();
      await workspace.optionsForm.locator('[data-testid$="_step"]').fill("1");
      await workspace.optionsForm
        .locator('[data-testid$="_numbers"]')
        .fill("2|2 GiB|2 GiB");
      const pricing = workspace.optionsForm.locator(
        '[data-testid$="_numeric_pricing"]',
      );
      await pricing.click();
      await adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText("每个规格子项独立固定价", { exact: true })
        .click();
      const saving = adminPage.waitForResponse(
        (r) =>
          r.request().method() === "PUT" && r.url().endsWith("/configuration"),
      );
      await workspace.confirmOptions();
      const saved = await (await saving).json();
      expect(saved.code).toBe(0);
      await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();
      await workspace.reopenOptions(product);
      await memoryButton.click();
      await expect(
        workspace.optionsForm.locator('[data-testid$="_numbers"]'),
      ).toHaveValue("2|2 GiB|2 GiB");
      await workspace.screenshot("real-discrete-memory-saved");
      await workspace.selectEditSection('价格与税务');
      const priceModal=adminPage.getByTestId('catalog-price-table-form');
      await adminPage.getByTestId("catalog-price-table-field").click();
      await adminPage
        .locator(".ant-select-dropdown:visible")
        .getByText("内存（GiB）", { exact: true })
        .click();
      await priceModal
        .getByRole("textbox", { name: "新购价格", exact: true })
        .first()
        .fill("5");
      await priceModal
        .getByRole("textbox", { name: "续费价格", exact: true })
        .first()
        .fill("5");
      const priceSaved = adminPage.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          r.url().endsWith("/configuration-price-batches"),
      );
      await adminPage.getByTestId("catalog-price-table-save").click();
      expect((await (await priceSaved).json()).code).toBe(0);
      await expect(priceModal).toBeVisible();
      await expect(priceModal.getByRole("textbox",{name:"新购价格",exact:true}).first()).toHaveValue("5.00");
      await workspace.screenshot("real-fixed-specification-price-saved");
      const sourceAfter = execFileSync(
        "python3",
        [database, "fingerprint", source],
        { encoding: "utf8" },
      ).trim();
      expect(sourceAfter).toBe(before);
    } finally {
      execFileSync("python3", [database, "cleanup", slug], {
        encoding: "utf8",
      });
      expect(
        execFileSync("python3", [database, "fingerprint", source], {
          encoding: "utf8",
        }).trim(),
      ).toBe(before);
    }
  });
});
