// Real isolated UI, pricing, relation and execution checks for explicit choice aliases.
// All payments and provisioning remain in the named loopback-only fixture.
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

test.describe("TC-75 子项标识与上游值分离", () => {
  test.skip(!descriptor, "Requires the dedicated disposable commerce database");
  test("TC-75a: 本地子项独立计价，关联按对应校验，已付款请求保留原接口值", async ({
    browser,
  }) => {
    test.setTimeout(240000);
    const f = (await fixture()) as Awaited<ReturnType<typeof fixture>> & {
      adminOrigin: string;
      websiteOrigin: string;
    };
    const admin = await api(f, f.adminAuth);
    const context = await browser.newContext({ baseURL: f.adminOrigin });
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
    const slug = "mapped-offer-" + randomUUID();
    let productNo = "";
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    try {
      const ui = new IsolatedCatalogPage(
        await context.newPage(),
        f.adminOrigin,
      );
      await ui.login();
      await ui.goto();
      const created = await ui.create(slug, "商业子项独立对应套餐");
      productNo = created.productNo;
      await ui.configureAutomation();
      await ui.configureFixedParameters();
      await ui.configureMonthlyPrice();
      const offer = new MerchantOfferPage(ui.page);
      await offer.selectEditSection("购买配置");
      const prices = ui.page.getByTestId("catalog-offer-field-prices");
      for (const [key, choices] of [
        [
          "local_product_region",
          "retail-west|隔离陕西地区|Shaanxi\nretail-east|隔离华东地区|East China",
        ],
        [
          "local_product_line",
          "retail-line-west|隔离电信节点|Shaanxi line\nretail-line-east|隔离华东节点|East China line",
        ],
      ]) {
        await offer.chooseField(key!);
        await offer.setChildren(choices!);
        if (key === "local_product_region") {
          await offer.save();
          await expect(offer.offers).toContainText(
            "请为每个可售子项选择有效的接口对应",
          );
        }
        const upstream =
          key === "local_product_region"
            ? ["隔离陕西地区 · 2", "隔离华东地区 · 9"]
            : ["隔离电信节点 · 5", "隔离华东节点 · 7"];
        for (const [index, label] of upstream.entries())
          await offer.mapChildToUpstream(index, label);
        await expect(offer.children).toContainText("接口对应");
        await expect(offer.children).toContainText("子项值");
        await expect(
          prices.getByTestId("catalog-offer-price-amount"),
        ).toHaveCount(2);
        for (let index = 0; index < 2; index++) {
          await prices
            .getByTestId("catalog-offer-price-amount")
            .nth(index)
            .fill(key === "local_product_line" && index === 1 ? "3" : "0");
          await prices
            .getByTestId("catalog-offer-price-renewal")
            .nth(index)
            .fill(key === "local_product_line" && index === 1 ? "3" : "0");
        }
      }
      await offer.chooseField("local_product_region");
      await offer.expectChildUpstream(0, "隔离陕西地区 · 2");
      await offer.expectChildUpstream(1, "隔离华东地区 · 9");
      await offer.chooseField("local_product_line");
      await offer.expectChildUpstream(0, "隔离电信节点 · 5");
      await offer.expectChildUpstream(1, "隔离华东节点 · 7");
      await offer.compactField();
      await offer.expectChildUpstream(1, "隔离华东节点 · 7");
      await offer.children.scrollIntoViewIfNeeded();
      await ui.screenshot("mapped-offer-child-upstream-selection");
      // Unsaved merchant choices must already use the current directory facts.
      // This precedes the first save, so stored commercial filters cannot make
      // the preview pass accidentally.
      await offer.setPreviewVisible(true);
      const draftPreview = ui.page.getByTestId("catalog-offer-preview");
      await draftPreview
        .getByTestId("catalog-option-preview-local_product_region")
        .getByRole("combobox")
        .press("ArrowDown");
      await ui.page
        .locator(".ant-select-dropdown:visible")
        .getByText("隔离华东地区", { exact: true })
        .click();
      await expect(
        draftPreview.getByTestId("catalog-option-preview-local_product_line"),
      ).toContainText("隔离华东节点");
      await draftPreview
        .getByTestId("catalog-option-preview-local_product_line")
        .getByRole("combobox")
        .press("ArrowDown");
      await expect(
        ui.page.locator(".ant-select-dropdown:visible"),
      ).not.toContainText("隔离电信节点");
      await ui.page.keyboard.press("Escape");
      await ui.screenshot("mapped-offer-unsaved-preview");
      await offer.setPreviewVisible(false);
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      const productPath =
        "/x/idc-core/api/v1/admin/catalog/products/" + productNo;
      let detail = await get(admin, productPath);
      let configPath =
        "/x/idc-core/api/v1/admin/catalog/product-versions/" +
        detail.version.versionNo +
        "/configuration";
      let configuration = (await get(admin, configPath)).configuration;
      const lineRules = (value: any) =>
        value.customerDeclaration.conditions.filter((rule: any) =>
          rule.actions.some(
            (action: any) => action.field === "local_product_line",
          ),
        );
      expect(lineRules(configuration)).toHaveLength(2);
      expect(lineRules(configuration)).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            when: expect.objectContaining({
              field: "local_product_region",
              value: "retail-east",
            }),
            actions: [
              expect.objectContaining({
                action: "filter_candidates",
                field: "local_product_line",
                values: ["retail-line-east"],
              }),
            ],
          }),
        ]),
      );
      expect(
        configuration.customerFieldSelections.find(
          (row: any) => row.fieldKey === "local_product_region",
        ).productChoiceBindings,
      ).toEqual({ "retail-west": "2", "retail-east": "9" });
      const invalidSelections = structuredClone(
        configuration.customerFieldSelections,
      );
      invalidSelections.find(
        (row: any) => row.fieldKey === "local_product_region",
      ).productChoiceBindings["retail-east"] = "99999";
      const invalidMapping = await admin.put(configPath, {
        data: {
          expectedVersion: configuration.version,
          merchantValues: configuration.merchantValues,
          customerFieldSelections: invalidSelections,
        },
      });
      expect((await invalidMapping.json()).code).not.toBe(0);
      expect((await get(admin, configPath)).configuration.version).toBe(
        configuration.version,
      );
      // Browser DTOs cannot erase or replace owner-derived restrictions.
      const resaveRequest = ui.page.waitForRequest(
        (request) =>
          request.method() === "PUT" &&
          request.url().endsWith("/configuration"),
      );
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      expect(
        (await resaveRequest)
          .postDataJSON()
          .customerFieldSelections.every((row: any) => !("choiceRules" in row)),
      ).toBe(true);
      detail = await get(admin, productPath);
      configPath =
        "/x/idc-core/api/v1/admin/catalog/product-versions/" +
        detail.version.versionNo +
        "/configuration";
      configuration = (await get(admin, configPath)).configuration;
      expect(lineRules(configuration)).toHaveLength(2);
      await offer.setPreviewVisible(true);
      const preview = ui.page.getByTestId("catalog-offer-preview");
      await preview
        .getByTestId("catalog-option-preview-local_product_region")
        .getByRole("combobox")
        .press("ArrowDown");
      await ui.page
        .locator(".ant-select-dropdown:visible")
        .getByText("隔离华东地区", { exact: true })
        .click();
      await expect(
        preview.getByTestId("catalog-option-preview-local_product_line"),
      ).toContainText("隔离华东节点");
      await expect(preview).not.toContainText("plugin.idc-core.");
      await ui.screenshot("mapped-offer-admin-preview");
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
      await expect(
        ui.page.getByTestId("public-product-configurator"),
      ).toContainText("地区");
      await ui.page
        .getByRole("radio", { name: "隔离华东地区", exact: true })
        .check();
      await expect(
        ui.page.getByRole("radio", { name: "隔离华东节点", exact: true }),
      ).toBeChecked();
      await expect(
        ui.page.getByRole("radio", { name: "隔离电信节点", exact: true }),
      ).toHaveCount(0);
      await ui.screenshot("mapped-offer-website-selected-child");

      const key = "mapped-paid-" + randomUUID();
      client = await createCustomer(f, admin, key);
      const publicPath =
        "/x/idc-core/api/v1/public/catalog/products/" +
        slug +
        "/purchase-configuration";
      const initial = await get(client, publicPath);
      const region = initial.fields.find(
        (field: any) => field.labelZhCN === "地区",
      );
      const line = initial.fields.find(
        (field: any) => field.labelZhCN === "线路",
      );
      expect(region.options.map((option: any) => option.labelZhCN)).toEqual([
        "隔离陕西地区",
        "隔离华东地区",
      ]);
      expect(line.options.map((option: any) => option.labelZhCN)).toEqual([
        "隔离电信节点",
      ]);
      const values: any[] = [...(initial.values || [])];
      for (const field of initial.fields.filter(
        (row: any) => row.visible && !row.disabled,
      )) {
        if (values.some((row) => row.fieldToken === field.fieldToken)) continue;
        const value: any = { fieldToken: field.fieldToken, type: field.type };
        if (field.type === "single_select")
          value.optionTokens = [field.options[0].optionToken];
        else if (field.type === "integer")
          value.integerValue =
            field.numericChoices?.[0] || field.minimumInteger;
        else if (field.type === "single_line") value.textValue = "related-v1";
        else if (field.type === "boolean") value.booleanValue = false;
        else continue;
        values.push(value);
      }
      values.find((row) => row.fieldToken === region.fieldToken).optionTokens =
        [region.options[1].optionToken];
      const evaluatedResponse = await client.post(publicPath + "/evaluate", {
        data: { configurationRevision: initial.configurationRevision, values },
      });
      const evaluatedBody = await evaluatedResponse.json();
      expect(evaluatedBody.code, evaluatedBody.message).toBe(0);
      const evaluated = evaluatedBody.data;
      expect(evaluated.valid).toBe(true);
      const filtered = evaluated.fields.find(
        (field: any) => field.labelZhCN === "线路",
      );
      expect(filtered.options.map((option: any) => option.labelZhCN)).toEqual([
        "隔离华东节点",
      ]);
      expect(
        evaluated.values.find((row: any) => row.fieldToken === line.fieldToken)
          .optionTokens,
      ).toEqual([filtered.options[0].optionToken]);
      const badQuote = await client.post(publicPath + "/quote", {
        data: {
          configurationRevision: evaluated.configurationRevision,
          currencyCode: "CNY",
          values,
        },
      });
      expect((await badQuote.json()).code).not.toBe(0);
      const invalidValues = structuredClone(values);
      values.find((row) => row.fieldToken === line.fieldToken).optionTokens = [
        filtered.options[0].optionToken,
      ];
      // A caller can bypass the configurator/quote UI. Cart freezing must
      // reject its unchanged invalid combination, never silently default it.
      const correctQuoteResponse = await client.post(publicPath + "/quote", {
        data: {
          configurationRevision: evaluated.configurationRevision,
          currencyCode: "CNY",
          values,
        },
      });
      const correctQuote = await correctQuoteResponse.json();
      expect(correctQuote.code, correctQuote.message).toBe(0);
      const emptyCartResponse = await client.post(
        "/x/idc-core/api/v1/account/commerce/carts",
        { data: { currencyCode: "CNY" } },
      );
      const emptyCart = await emptyCartResponse.json();
      expect(emptyCart.code, emptyCart.message).toBe(0);
      const invalidCartResponse = await client.post(
        "/x/idc-core/api/v1/account/commerce/cart-items",
        {
          data: {
            expectedCartVersion: emptyCart.data.cart.version,
            quantity: 1,
            publicSlug: slug,
            configurationRevision: evaluated.configurationRevision,
            values: invalidValues,
            billingTermToken: correctQuote.data.terms[0].billingTermToken,
            currencyCode: "CNY",
            purchaseCycles: 1,
          },
        },
      );
      const invalidCart = await invalidCartResponse.json();
      expect(
        invalidCart.code,
        "Invalid relationship reached the cart freeze",
      ).not.toBe(0);
      await expect
        .poll(
          async () =>
            (await get(client!, "/x/idc-core/api/v1/account/commerce/cart"))
              .cart.items.length,
        )
        .toBe(0);
      const { orderNo } = await paidConfiguredOrder(
        f,
        client,
        key,
        {
          publicSlug: slug,
          configurationRevision: evaluated.configurationRevision,
          values,
        },
        "1400",
      );
      const tasks = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect
        .poll(
          async () => {
            const found = (await get(admin, tasks + "?orderNo=" + orderNo))
              .tasks;
            if (found.length !== 1) return "";
            task = (await get(admin, tasks + "/" + found[0].intentNo)).task;
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
      expect(forms[0]).toMatchObject({
        area: ["9"],
        node: ["7"],
        cpu: ["2"],
        memory: ["2000"],
      });
      const order = (
        await get(
          client,
          "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
        )
      ).order;
      expect(order.invoice.paidAmountMinor).toBe("1400");
      expect(order.invoice.refundedAmountMinor).toBe("0");
      await ui.goto();
      await ui.searchNamePrefix(slug);
      await ui.openProductDetail(productNo);
      await offer.selectEditSection("购买配置");
      await offer.chooseField("local_product_line");
      await expect(
        prices.getByTestId("catalog-offer-price-amount").nth(1),
      ).toHaveValue("3.00");
      await offer.mapChildToUpstream(1, "隔离备用节点 · 6");
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      detail = await get(admin, productPath);
      configuration = (
        await get(
          admin,
          "/x/idc-core/api/v1/admin/catalog/product-versions/" +
            detail.version.versionNo +
            "/configuration",
        )
      ).configuration;
      const commercialPrice = configuration.merchantPriceTargets.find(
        (row: any) =>
          row.fieldKey === "local_product_line" &&
          row.candidateValue === "retail-line-east",
      );
      expect(
        detail.prices.find(
          (row: any) =>
            row.configurationPriceDimensionNo === commercialPrice.dimensionNo &&
            row.status === "active",
        ).amountMinor,
      ).toBe("300");
      expect(
        configuration.customerFieldSelections.find(
          (row: any) => row.fieldKey === "local_product_line",
        ).productChoiceBindings["retail-line-east"],
      ).toBe("6");
      const unchanged = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(unchanged).toHaveLength(1);
      expect(unchanged[0].node).toEqual(["7"]);
      expect(
        (
          await get(
            client,
            "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
          )
        ).order.invoice.paidAmountMinor,
      ).toBe("1400");
      await expect(
        ui.page.getByTestId("catalog-customer-options-save"),
      ).toBeEnabled();
      await expect(
        prices.getByTestId("catalog-offer-price-amount").nth(1),
      ).toHaveValue("3.00");
      await prices.scrollIntoViewIfNeeded();
      await ui.screenshot("mapped-offer-final-commercial-price");
    } finally {
      if (productNo) {
        const detail = await get(
          admin,
          "/x/idc-core/api/v1/admin/catalog/products/" + productNo,
        );
        if (detail.product.status === "active") {
          const result = await admin.put(
            "/x/idc-core/api/v1/admin/catalog/products/" +
              productNo +
              "/sale-state",
            {
              data: {
                versionNo: detail.product.currentVersionNo,
                expectedVersion: detail.product.descriptionVersion,
                active: false,
              },
            },
          );
          const body = await result.json();
          expect(body.code, body.message).toBe(0);
        }
      }
      await client?.dispose();
      await context.close();
      await admin.dispose();
    }
  });
});
