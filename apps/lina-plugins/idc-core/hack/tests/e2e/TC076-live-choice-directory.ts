// UI-only directory recovery preserves merchant drafts. No real supplier,
// transaction, product or service state is written by this fixture.
import { test, expect } from "../support/merchant-auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-76 当前配置的目录恢复", () => {
  test("TC-76a: 存储接口选项恢复后当前下拉可选新值，未保存名称和子项保留", async ({
    adminPage,
  }) => {
    test.setTimeout(60000);
    await installCatalogManagementMock(adminPage);
    const detail: any = mockProductDetail("product-manual-service");
    detail.version.status = "draft";
    detail.fulfillmentBinding = {
      interfaceInstanceNo: "storage-fixture",
      interfaceDisplayName: "存储接口",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    const option = (value: string, name: string) => ({
      value,
      label: { zhCN: name, enUS: value },
    });
    const local = {
      key: "local_plan",
      type: "single_select",
      required: true,
      label: { zhCN: "存储等级", enUS: "Storage profile" },
      staticCandidates: [option("standard", "标准存储")],
    };
    const note = {
      key: "local_note",
      type: "single_line",
      label: { zhCN: "服务备注", enUS: "Service note" },
    };
    const configuration: any = {
      version: 1,
      merchantValues: [{ fieldKey: "pool", optionTokens: ["source-old"] }],
      productDeclaration: {
        version: "v1",
        fields: [
          {
            key: "pool",
            type: "single_select",
            required: true,
            label: { zhCN: "存储池", enUS: "Storage pool" },
            dynamicCandidates: { version: "v1", method: "storage.options" },
          },
        ],
      },
      customerDeclaration: { version: "v1", fields: [local, note] },
      customerConfigurableFields: [],
      customerFieldSelections: [
        {
          fieldKey: local.key,
          localField: local,
          productFieldKey: "pool",
          productChoiceBindings: { standard: "source-old" },
        },
        { fieldKey: note.key, localField: note },
      ],
      productResolvedCandidates: { pool: [option("source-old", "原存储池")] },
      customerResolvedCandidates: {},
      upstreamOptionsStatus: "retained",
    };
    let recovered = false,
      recoveryReads = 0;
    const writes: unknown[] = [];
    await adminPage.route(
      "**/admin/catalog/products/product-manual-service",
      (route) => route.fulfill({ json: { code: 0, data: detail } }),
    );
    await adminPage.route(
      "**/admin/catalog/product-versions/*/configuration",
      (route) => {
        if (route.request().method() !== "GET")
          writes.push(route.request().postDataJSON());
        const view = structuredClone(configuration);
        if (recovered) {
          recoveryReads++;
          view.productResolvedCandidates.pool.push(
            option("source-new", "新存储池"),
          );
          // Retained status deliberately keeps the bounded polling active so
          // closing the editor must cancel the next scheduled read.
        }
        return route.fulfill({
          json: { code: 0, data: { configuration: view } },
        });
      },
    );
    const view = new MerchantOfferPage(adminPage);
    await view.goto();
    await view.openProductDetail("product-manual-service");
    await view.selectEditSection("购买配置");
    await view.chooseField("local_plan");
    await view.expectChildUpstream(0, "原存储池 · source-old");
    await view.input("配置名称（中文）").fill("我自己的存储配置");
    recovered = true;
    await expect
      .poll(() => recoveryReads, { timeout: 10000 })
      .toBeGreaterThan(0);
    await view.mapChildToUpstream(0, "新存储池 · source-new");
    await expect(view.input("配置名称（中文）")).toHaveValue(
      "我自己的存储配置",
    );
    await view.chooseField("local_note");
    await view.chooseField("local_plan");
    await view.expectChildUpstream(0, "新存储池 · source-new");
    await expect(
      view.children.getByTestId("catalog-option-child-value"),
    ).toHaveValue("standard");
    await expect(view.input("配置名称（中文）")).toHaveValue(
      "我自己的存储配置",
    );
    expect(writes).toHaveLength(0);
    await expect(
      view.offers.getByText("请输入前台展示方式", { exact: true }),
    ).toHaveCount(0);
    await expect(
      view.offers.getByText("请输入配置子项", { exact: true }),
    ).toHaveCount(0);
    await view.children.scrollIntoViewIfNeeded();
    await view.screenshot("live-storage-directory-recovered");
    await view.closeProductDetail();
    const stoppedAt = recoveryReads;
    // Opening another part of the page must not continue a closed editor's
    // directory polling. The assertion waits across the polling interval.
    const unexpectedRead = await adminPage
      .waitForRequest(
        (request) =>
          request.method() === "GET" &&
          new URL(request.url()).pathname.endsWith("/configuration"),
        { timeout: 2500 },
      )
      .then(
        () => true,
        () => false,
      );
    expect(unexpectedRead).toBe(false);
    expect(recoveryReads).toBe(stoppedAt);
  });
});
