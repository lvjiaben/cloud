// Marginal configuration pricing through real isolated authoring, checkout and
// execution. Only the named local fixture receives writes or payment callbacks.
import { createHash, randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect } from "@host-tests/support/playwright";
import {
  api,
  descriptor,
  fixture,
  get,
  saved,
} from "../support/isolated-commerce";
import {
  createCustomer,
  paidConfiguredOrder,
} from "../support/isolated-paid-order";
import { IsolatedCatalogPage } from "../pages/IsolatedCatalogPage";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-78 配置分段累加", () => {
  test.skip(!descriptor, "Requires the dedicated disposable commerce database");
  test("TC-78a: 统一配置内维护免费首段，实际核价、付款与开通都保留十五单位", async ({
    browser,
  }) => {
    test.setTimeout(240000);
    const f = (await fixture()) as Awaited<ReturnType<typeof fixture>> & {
      adminOrigin: string;
      websiteOrigin: string;
    };
    const admin = await api(f, f.adminAuth),
      context = await browser.newContext({ baseURL: f.adminOrigin });
    context.setDefaultTimeout(20000);
    const allowed = new Set([
      f.adminOrigin,
      f.websiteOrigin,
      f.baseURL,
      f.mockOrigin,
    ]);
    await context.route("**/*", (route) =>
      allowed.has(new URL(route.request().url()).origin)
        ? route.continue()
        : route.abort(),
    );
    let customer: Awaited<ReturnType<typeof createCustomer>> | undefined;
    const slug = "graduated-" + randomUUID();
    let productNo = "";
    try {
      const ui = new IsolatedCatalogPage(
        await context.newPage(),
        f.adminOrigin,
      );
      await ui.login();
      await ui.goto();
      productNo = (await ui.create(slug, "隔离分段累加云服务")).productNo;
      await ui.configureAutomation();
      await ui.configureFixedParameters();
      await ui.configureMonthlyPrice();
      const offer = new MerchantOfferPage(ui.page);
      await offer.selectEditSection("购买配置");
      await offer.chooseField("local_product_bandwidth");
      await offer.control("数字输入框");
      await offer.input("最小可售数量").fill("1");
      await offer.input("最大可售数量").fill("50");
      await offer.setPreviewVisible(false);
      const prices = ui.page.getByTestId("catalog-offer-field-prices");
      const mode = prices.getByRole("combobox", {
        name: "计价方式",
        exact: true,
      });
      await ui.choose(mode, "分段累加");
      await expect(prices).toContainText("选择15单位的附加费用为10元");
      await prices.getByTestId("catalog-offer-price-upper").fill("10");
      await prices.getByTestId("catalog-offer-price-amount").fill("0");
      await prices.getByTestId("catalog-offer-price-renewal").fill("0");
      await prices.getByTestId("catalog-offer-price-setup").fill("0.50");
      await prices
        .getByRole("button", { name: "增加一档", exact: true })
        .click();
      await prices.getByTestId("catalog-offer-price-amount").nth(1).fill("2");
      await prices.getByTestId("catalog-offer-price-renewal").nth(1).fill("3");
      await prices.getByTestId("catalog-offer-price-setup").nth(1).fill("0.01");
      await offer.save();
      await expect(offer.offers).toContainText(
        "分段累加只在第一段收取一次性费用",
      );
      await prices.getByTestId("catalog-offer-price-setup").nth(1).fill("0");
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      await expect(
        ui.page.getByTestId("catalog-customer-options-save"),
      ).toBeEnabled({ timeout: 3000 });
      await expect(prices).toContainText("分段累加");
      await prices.scrollIntoViewIfNeeded();
      await ui.screenshot("graduated-bands-merchant-editor");
      await ui.configureTax();
      await ui.closeProductDetail();
      await ui.searchNamePrefix(slug);
      await ui.openSaleStateConfirmation(productNo);
      await ui.writeThrough(
        () =>
          ui.page
            .locator(".ant-popover:visible")
            .getByRole("button", { name: /确\s*定/ })
            .click(),
        "/sale-state",
        "PUT",
      );
      await ui.page.goto(f.websiteOrigin + "/products/" + slug);
      const quantityInput = ui.page.getByRole("spinbutton", {
        name: "带宽（Mbps）",
        exact: true,
      });
      await quantityInput.fill("15");
      await quantityInput.press("Tab");
      await ui.page
        .getByRole("button", { name: "查看准确价格", exact: true })
        .click();
      await expect(ui.page.getByTestId("public-quote-total")).toContainText(
        "21.50",
      );
      await expect(
        ui.page.getByTestId("purchase-selected-configuration"),
      ).toContainText("15");
      await ui.page.getByTestId("public-quote-total").scrollIntoViewIfNeeded();
      await ui.screenshot("graduated-bands-website-fifteen-units");
      const key = "graduated-paid-" + randomUUID();
      customer = await createCustomer(f, admin, key);
      const publicPath =
        "/x/idc-core/api/v1/public/catalog/products/" +
        slug +
        "/purchase-configuration";
      const initial = await get(customer, publicPath),
        values: any[] = structuredClone(initial.values || []);
      for (const field of initial.fields.filter(
        (row: any) => row.visible && !row.disabled,
      )) {
        let value = values.find((row) => row.fieldToken === field.fieldToken);
        if (!value) {
          value = { fieldToken: field.fieldToken, type: field.type };
          values.push(value);
        }
        if (field.type === "integer")
          value.integerValue =
            field.labelZhCN === "带宽（Mbps）"
              ? "15"
              : field.numericChoices?.[0] || field.minimumInteger;
        else if (field.type === "single_select" && !value.optionTokens?.length)
          value.optionTokens = [field.options[0].optionToken];
        else if (field.type === "single_line" && !value.textValue)
          value.textValue = "graduated-v1";
        else if (field.type === "boolean") value.booleanValue = false;
      }
      const evaluatedResponse = await customer.post(publicPath + "/evaluate", {
        data: { configurationRevision: initial.configurationRevision, values },
      });
      const evaluated = await evaluatedResponse.json();
      expect(evaluated.code, evaluated.message).toBe(0);
      expect(evaluated.data.valid).toBe(true);
      const purchase = {
        publicSlug: slug,
        configurationRevision: evaluated.data.configurationRevision,
        values: evaluated.data.values,
      };
      // 1.00 base + 10.00 base setup + five charged units at 2.00 + 0.50 setup.
      const { orderNo } = await paidConfiguredOrder(
        f,
        customer,
        key,
        purchase,
        "2150",
      );
      const tasks = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect
        .poll(
          async () => {
            const rows = (await get(admin, tasks + "?orderNo=" + orderNo))
              .tasks;
            if (rows.length !== 1) return "";
            task = (await get(admin, tasks + "/" + rows[0].intentNo)).task;
            return task.statusCode;
          },
          { timeout: 120000, intervals: [1000] },
        )
        .toBe("completed");
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            task.intentNo +
            "\0" +
            task.orderItemNo +
            "\0" +
            task.unitSequence,
        )
        .digest("hex");
      const forms = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(forms).toHaveLength(1);
      expect(forms[0].out_bw).toEqual(["15"]);
      expect(forms[0].in_bw).toEqual(["15"]);
      const order = (
        await get(
          customer,
          "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
        )
      ).order;
      expect(order.invoice.paidAmountMinor).toBe("2150");
      expect(order.invoice.refundedAmountMinor).toBe("0");
      const detail = await get(
        admin,
        "/x/idc-core/api/v1/admin/catalog/products/" + productNo,
      );
      expect(
        detail.prices.some(
          (row: any) => row.configurationPricingMode === "graduated",
        ),
      ).toBe(true);
    } finally {
      if (productNo) {
        const current = await get(
          admin,
          "/x/idc-core/api/v1/admin/catalog/products/" + productNo,
        );
        if (current.product.status === "active")
          await admin.put(
            "/x/idc-core/api/v1/admin/catalog/products/" +
              productNo +
              "/sale-state",
            {
              data: {
                expectedVersion: current.product.descriptionVersion,
                versionNo: current.product.currentVersionNo,
                active: false,
              },
            },
          );
      }
      await customer?.dispose();
      await context.close();
      await admin.dispose();
    }
  });
});
