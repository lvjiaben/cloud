import { test as baseTest } from "../support/catalog-auth";
import { expect } from "@host-tests/fixtures/auth";
import { MainLayout } from "@host-tests/pages/MainLayout";
import { CustomerWorkspacePage } from "../pages/CustomerWorkspacePage";
import {
  installCustomerWorkspaceMock,
  installProductManagementLocales,
  workspaceCustomerNo,
} from "../support/customer-workspace-mock";

// Attach current-build routes before the first navigation; preserve the normal authenticated storage.
const test = baseTest.extend({ adminPage: async ({ authenticatedPage }, use) => { await use(authenticatedPage); } });

test.describe("TC-39 客户钱包、产品和API概览", () => {
  test("TC-39j: 证书刷新撤回验证方式动作后，不保留旧表单或发送过期动作", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    await installProductManagementLocales(adminPage);
    await adminPage.route("**/api/v1/plugins/dynamic", async route => {
      const response = await route.fetch(); const body = await response.json();
      body.data.list = body.data.list.filter((item: {id:string}) => item.id !== "yunsu-ssl");
      body.data.list.push({id:"yunsu-ssl",installed:1,enabled:1});
      await route.fulfill({json:body});
    });
    let reads = 0; let writes = 0;
    await adminPage.route("**/x/idc-core/api/v1/admin/service-entitlements/ENT_SSL**", route => {
      if (route.request().method() !== "GET") { writes++; return route.fulfill({status:403,json:{code:403}}); }
      reads++;
      return route.fulfill({json:{code:0,data:{item:{entitlementNo:"ENT_SSL",primaryDomain:"customer.example.com",customerName:"当前客户",productNameZhCn:"SSL证书",productNameEnUs:"SSL certificate",statusCode:reads === 1 ? "validating" : "issued",validationMethod:"dns",validation:[],availableActions:reads === 1 ? ["update-validation"] : [],issuedAt:Date.now(),expiresAt:Date.now()+86400000,updatedAt:Date.now()}}}});
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto(); await view.openOverview(workspaceCustomerNo); await view.tab("产品"); await view.manageProduct("ENT_SSL");
    const certificate = view.management.getByTestId("customer-certificate-management");
    await certificate.getByRole("button",{name:"切换验证方式",exact:true}).click();
    await expect(certificate.getByRole("button",{name:/^保\s*存$/u})).toBeVisible();
    await certificate.getByRole("button",{name:/^刷\s*新$/u}).click();
    await expect(certificate).toContainText("已签发");
    await expect(certificate.getByRole("button",{name:/^保\s*存$/u})).toHaveCount(0);
    await expect(certificate.getByRole("combobox")).toHaveCount(0);
    expect(reads).toBe(2); expect(writes).toBe(0);
    await view.screenshot("workspace-certificate-action-withdrawn");
  });
  test("TC-39i: A后B打开管理的目录响应乱序时，B停用不能被A晚到结果覆盖", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    await installProductManagementLocales(adminPage);
    await adminPage.route("**/x/idc-core/api/v1/admin/customers/*/products**", route => route.fulfill({ json: { code: 0, data: { total: 2, items: ["A", "B"].map(key => ({
      number: `SVC_${key}`, deliveryKind: "server", productType: "cloud_server", pluginId: "idc-resource",
      nameZhCn: `服务器${key}`, nameEnUs: `Server ${key}`, orderNo: "ORD_1", productNo: "PRD_1", information: "4核 8GB", amountMinor: "12000", currencyCode: "CNY", periodUnit: "month", periodCount: 1, createdAt: Date.now(), expiresAt: null, status: "active",
    })) } } }));
    let waiting = false; let detailReads = 0;
    const replies: Array<(enabled: number) => Promise<void>> = [];
    await adminPage.route("**/api/v1/plugins/dynamic", async route => {
      const response = await route.fetch(); const body = await response.json();
      body.data.list = body.data.list.filter((item: {id:string}) => item.id !== "idc-resource");
      if (!waiting) { body.data.list.push({id:"idc-resource",installed:1,enabled:1}); return route.fulfill({json:body}); }
      await new Promise<void>(done => replies.push(async enabled => {
        body.data.list.push({id:"idc-resource",installed:1,enabled});
        await route.fulfill({json:body}); done();
      }));
    });
    await adminPage.route("**/x/idc-core/api/v1/admin/resource/server-accessories/*", route => { detailReads++; return route.fulfill({status:403,json:{code:403}}); });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto(); await view.openOverview(workspaceCustomerNo); await view.tab("产品");
    await expect(view.products.getByTestId("customer-product-manage-SVC_A")).toBeVisible();
    waiting = true;
    await view.products.getByTestId("customer-product-manage-SVC_A").click();
    await expect.poll(() => replies.length).toBe(1);
    await view.products.getByTestId("customer-product-manage-SVC_B").click();
    await expect.poll(() => replies.length).toBe(2);
    await replies[1]!(0);
    await expect(view.products.getByTestId("customer-product-manage-SVC_B")).toHaveCount(0);
    const late = adminPage.waitForResponse(response => response.url().includes("/api/v1/plugins/dynamic"));
    await replies[0]!(1); await late;
    await adminPage.evaluate(() => new Promise<void>(done => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
    await expect(view.management).toHaveCount(0);
    await expect(view.products.getByTestId("customer-product-manage-SVC_A")).toHaveCount(0);
    expect(detailReads).toBe(0);
    await view.screenshot("workspace-management-entry-late-response");
  });
  for (const mode of ["failed-retry", "disabled-at-open"] as const) {
    test(`TC-39h: 产品管理插件状态${mode}时可恢复且不会误开旧入口`, async ({ adminPage }) => {
      await installCustomerWorkspaceMock(adminPage);
      await installProductManagementLocales(adminPage);
      let stage = "normal"; let changedReads = 0; let detailReads = 0;
      await adminPage.route("**/api/v1/plugins/dynamic", async route => {
        if (stage !== "normal") changedReads++;
        if (stage === "failed") return route.fulfill({ status: 503, json: { code: 51, message: "状态读取暂不可用" } });
        const response = await route.fetch(); const body = await response.json();
        body.data.list = body.data.list.filter((item: {id: string}) => item.id !== "idc-resource");
        body.data.list.push({ id: "idc-resource", enabled: stage === "disabled" ? 0 : 1, installed: 1 });
        await route.fulfill({ json: body });
      });
      await adminPage.route("**/x/idc-core/api/v1/admin/resource/server-accessories/*", route => { detailReads++; return route.fulfill({ json: { code: 0, data: { detail: { serviceNo: "SVC_SERVER", accessories: [], availableActions: [] } } } }); });
      const view = new CustomerWorkspacePage(adminPage);
      await view.goto(); await view.openOverview(workspaceCustomerNo);
      if (mode === "failed-retry") stage = "failed";
      await view.tab("产品"); await expect(view.products).toContainText("SVC_SERVER");
      if (mode === "failed-retry") {
        await expect(view.products).toContainText("暂时无法确认产品管理插件的状态", { timeout: 3000 });
        await expect(view.products.getByTestId("customer-product-manage-SVC_SERVER")).toHaveCount(0);
        await view.screenshot("workspace-plugin-status-read-failed");
        stage = "normal";
        await view.retryProducts();
        await expect(view.products.getByTestId("customer-product-manage-SVC_SERVER")).toBeVisible();
        await expect(view.products).not.toContainText("暂时无法确认产品管理插件的状态");
      } else {
        stage = "disabled";
        await view.products.getByTestId("customer-product-manage-SVC_SERVER").click();
        await expect.poll(() => changedReads, { timeout: 3000 }).toBeGreaterThan(0);
        await expect(view.products.getByTestId("customer-product-manage-SVC_SERVER")).toHaveCount(0);
        await expect(view.management).toHaveCount(0);
        await expect(view.products).not.toContainText("暂时无法确认产品管理插件的状态");
      }
      expect(detailReads).toBe(0);
      await view.screenshot(`workspace-plugin-${mode}`);
    });
  }
  for (const mode of ["detail", "tasks", "none"] as const) {
    test(`TC-39g: 动态服务器真实产品投影的 ${mode} 权限按对象管理`, async ({ adminPage }) => {
      await installCustomerWorkspaceMock(adminPage);
      await installProductManagementLocales(adminPage);
      await adminPage.route("**/api/v1/user/info", async route => {
        const response = await route.fetch(); const body = await response.json();
        body.data.homePath = "/idc/customers";
        body.data.permissions = ["idc:admin:customer:query", "idc:admin:customer:list", ...(mode === "detail" ? ["idc:admin:resource:server-batch:view", "idc:admin:resource:server-batch:safe", "idc:admin:resource:task:list"] : mode === "tasks" ? ["idc:admin:resource:task:list"] : [])];
        await route.fulfill({ json: body });
      });
      await adminPage.route("**/api/v1/plugins/dynamic", async route => {
        const response = await route.fetch(); const body = await response.json();
        body.data.list = body.data.list.filter((item: { id: string }) => item.id !== "idc-zjmfcloud");
        body.data.list.push({ id: "idc-zjmfcloud", enabled: 1, installed: 1 });
        await route.fulfill({ json: body });
      });
      await adminPage.route("**/x/idc-core/api/v1/admin/customers/*/products**", route => route.fulfill({ json: { code: 0, data: { total: 1, items: [{
        number: "ENT_NATIVE", deliveryKind: "entitlement", productType: "idc-resource", pluginId: "idc-zjmfcloud",
        nameZhCn: "动态云服务器", nameEnUs: "Native server", orderNo: "ORD_NATIVE", productNo: "PRD_NATIVE", information: "4核 8GB", amountMinor: "12000", currencyCode: "CNY", periodUnit: "month", periodCount: 1, createdAt: Date.now(), expiresAt: null, status: "active",
      }] } } }));
      const nativeReads: URL[] = []; let failed = true; let legacyReads = 0;
      await adminPage.route("**/x/idc-core/api/v1/admin/resource/server-accessories**", route => { legacyReads++; return route.fulfill({ status: 400, json: {} }); });
      await adminPage.route("**/x/idc-resource/api/v1/admin/resource/zjmfcloud-servers?**", route => {
        nativeReads.push(new URL(route.request().url()));
        return route.fulfill(failed ? { status: 503, json: { code: 51, message: "服务暂时不可用" } } : { json: { code: 0, data: { items: [{ serverNo: `SVR-${"A".repeat(32)}`, boundAt: Date.now(), updatedAt: Date.now(), productNameZhCn: "动态云服务器", productNameEnUs: "Native server", channelName: "当前接口", poolName: "当前资源池", state: "ready", kind: "cloud_server", latestOperation: "idle" }], total: 1, nextCursor: "" } } });
      });
      const taskReads: URL[] = [];
      const renewals: Array<{ url: URL; body?: { entitlementNo: string; serverNos: string[]; requestKey: string; renewalCycles: number } }> = [];
      await adminPage.route("**/x/idc-resource/api/v1/admin/resource/server-renewals**", route => {
        const url = new URL(route.request().url());
        const body = route.request().method() === "POST" ? route.request().postDataJSON() : undefined;
        renewals.push({ url, body });
        const serverNo = `SVR-${"A".repeat(32)}`;
        const item = { serverNo, productNameZhCN: "动态云服务器", productNameEnUS: "Native server", eligible: true, failureCode: "", currencyCode: "CNY", totalAmountMinor: "12000", originalExpiresAt: Date.now(), projectedExpiresAt: Date.now() + 86400000, quoteExpiresAt: Date.now() + 300000, status: "pending_payment", succeeded: true };
        return route.fulfill({ json: { code: 0, data: body ? { requestKey: body.requestKey, renewalCycles: body.renewalCycles, confirmationDigest: "a".repeat(64), items: [item] } : { items: [item], total: 1, nextCursor: "" } } });
      });
      await adminPage.route("**/x/idc-resource/api/v1/admin/resource/tasks?**", route => { taskReads.push(new URL(route.request().url())); return route.fulfill({ json: { code: 0, data: { items: [], total: 0, nextCursor: "" } } }); });
      const view = new CustomerWorkspacePage(adminPage);
      await view.goto(); await view.openOverview(workspaceCustomerNo); await view.tab("产品");
      if (mode === "none") { await expect(view.products.getByTestId("customer-product-manage-ENT_NATIVE")).toHaveCount(0); return; }
      await view.manageProduct("ENT_NATIVE");
      if (mode === "detail") {
        await expect(view.management.getByTestId("customer-native-server-detail")).toContainText("无法查看这台服务器");
        failed = false;
        await view.management.getByRole("button", { name: "重新读取", exact: true }).click();
        await expect(view.management).toContainText("当前接口");
        expect(nativeReads.every(url => url.searchParams.get("entitlementNo") === "ENT_NATIVE")).toBe(true);
        await expect(adminPage.locator(".ant-message-notice:visible")).toHaveCount(0);
        await view.screenshot("workspace-native-server-management");
        await view.managementTab("续费");
        const renewal = view.management.getByTestId("server-batch-renewal-page");
        await expect(renewal).toContainText(`SVR-${"A".repeat(32)}`);
        expect(renewals[0]?.url.searchParams.get("entitlementNo")).toBe("ENT_NATIVE");
        await renewal.locator("tr", { hasText: `SVR-${"A".repeat(32)}` }).locator(".ant-checkbox-input").click();
        await renewal.getByTestId("server-renewal-preview").click();
        const confirmation = adminPage.getByRole("dialog", { name: "确认服务器续费", exact: true });
        await expect(confirmation).toContainText(/120\.00/);
        await confirmation.getByLabel("已核对服务器、周期、费用和到期时间").check();
        await confirmation.getByRole("button", { name: /确\s*认/u }).click();
        await expect.poll(() => renewals.filter(item => item.url.pathname.endsWith("/confirmations")).length).toBe(1);
        expect(renewals.filter(item => item.body).every(item => item.body?.entitlementNo === "ENT_NATIVE" && item.body.serverNos.length === 1 && item.body.serverNos[0] === `SVR-${"A".repeat(32)}`)).toBe(true);
        await view.screenshot("workspace-native-server-renewal-result");
        await adminPage.keyboard.press("Escape");
        await view.managementTab("操作记录");
      } else {
        await expect(view.management.getByRole("tab", { name: "管理", exact: true })).toHaveCount(0);
        expect(nativeReads).toHaveLength(0);
      }
      await expect.poll(() => taskReads.length).toBeGreaterThan(0);
      expect(taskReads.every(url => url.searchParams.get("serviceNo") === "ENT_NATIVE")).toBe(true);
      expect(legacyReads).toBe(0);
      await view.screenshot(`workspace-native-server-${mode}-tasks`);
    });
  }
  test.afterEach(async ({ adminPage }) => {
    await adminPage.unrouteAll({ behavior: "wait" });
  });
  test("TC-39e: 客户产品内管理服务器和精确任务，不查询全局产品", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    await installProductManagementLocales(adminPage);
    const reads: URL[] = [];
    await adminPage.route("**/x/idc-core/api/v1/admin/resource/server-accessories/*", route => {
      reads.push(new URL(route.request().url()));
      return route.fulfill({ json: { code: 0, data: { detail: {
        serviceNo: "SVC_SERVER", customerNo: workspaceCustomerNo,
        productNameZhCN: "云服务器", productNameEnUS: "Cloud server",
        entitlementStatus: "active", runtimeStatus: "running", providerTaskStatus: "succeeded",
        accessories: [], availableActions: [], activeOperationNo: "",
      } } } });
    });
    const tasks: URL[] = [];
    await adminPage.route("**/x/idc-resource/api/v1/admin/resource/tasks?**", route => {
      const url = new URL(route.request().url()); tasks.push(url);
      return route.fulfill({ json: { code: 0, data: { items: [], total: 0, nextCursor: "" } } });
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto(); await view.openOverview(workspaceCustomerNo);
    await expect(view.overviewDrawer.getByTestId("customer-360-tab-contracts")).toHaveCount(0);
    await expect(view.overviewDrawer.getByTestId("customer-360-tab-tax-invoices")).toContainText("发票");
    await view.tab("产品");
    const originalURL = adminPage.url();
    await view.manageProduct("SVC_SERVER");
    await expect.poll(() => reads.length).toBeGreaterThan(0);
    expect(reads.every(url => url.pathname.endsWith("/SVC_SERVER"))).toBe(true);
    await expect(view.management.getByTestId("server-accessory-service-no")).toHaveCount(0);
    await expect(view.management).toContainText("SVC_SERVER");
    await expect(view.management).toContainText("运行中");
    expect(adminPage.url()).toBe(originalURL);
    await view.screenshot("workspace-product-server-management");
    await view.managementTab("操作记录");
    await expect.poll(() => tasks.length).toBeGreaterThan(0);
    expect(tasks.every(url => url.searchParams.get("serviceNo") === "SVC_SERVER")).toBe(true);
    await expect(view.management).not.toContainText("plugin.idc-resource");
    await view.screenshot("workspace-product-management-tasks");
    await view.managementTab("对接日志");
    await expect(view.management).toContainText("该产品暂无可关联的对接日志");
    await view.screenshot("workspace-product-management-logs");
  });
  test("TC-39f: SSL在客户产品内精确管理，读取失败保留重试且不打开全局证书目录", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    await installProductManagementLocales(adminPage);
    await adminPage.route("**/api/v1/plugins/dynamic", async route => {
      const response = await route.fetch(); const body = await response.json();
      body.data.list = body.data.list.filter((item: { id: string }) => item.id !== "yunsu-ssl");
      body.data.list.push({ id: "yunsu-ssl", enabled: 1, installed: 1 });
      await route.fulfill({ json: body });
    });
    let failed = true; const reads: string[] = []; const actions: Array<{ path: string; body: unknown }> = [];
    await adminPage.route("**/x/idc-core/api/v1/admin/service-entitlements**", route => {
      const url = new URL(route.request().url()); reads.push(url.pathname);
      if (route.request().method() === "POST") {
        actions.push({ path: url.pathname, body: route.request().postDataJSON() });
        return route.fulfill({ json: { code: 0, data: { task: { status: "succeeded" } } } });
      }
      if (failed) return route.fulfill({ status: 503, json: { code: 51, message: "服务暂时不可用" } });
      return route.fulfill({ json: { code: 0, data: { item: {
        entitlementNo: "ENT_SSL", primaryDomain: "customer.example.com", customerName: "当前客户",
        productNameZhCn: "单域名SSL证书", productNameEnUs: "SSL certificate", statusCode: "issued",
        validationMethod: "dns", validation: [{ domain: "customer.example.com", method: "dns", host: "_verify", value: "current-customer-value" }],
        availableActions: ["sync"], issuedAt: Date.now(), expiresAt: Date.now() + 86400000, updatedAt: Date.now(),
      } } } });
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto(); await view.openOverview(workspaceCustomerNo); await view.tab("产品");
    const originalURL = adminPage.url(); await view.manageProduct("ENT_SSL");
    await expect(view.management).toContainText("暂时无法读取证书");
    failed = false;
    await view.management.getByRole("button", { name: /^重\s*试$/u }).click();
    await expect(view.management).toContainText("customer.example.com");
    await expect(view.management).toContainText("已签发");
    await expect(view.management).toContainText("current-customer-value");
    expect(reads.every(path => path.includes("/service-entitlements/ENT_SSL"))).toBe(true);
    await view.management.getByRole("button", { name: "同步状态", exact: true }).click();
    await expect.poll(() => actions.length).toBe(1);
    expect(actions[0].path).toMatch(/ENT_SSL\/actions\/sync$/);
    expect(actions[0].body).toMatchObject({ input: {}, idempotencyKey: expect.any(String) });
    expect(adminPage.url()).toBe(originalURL);
    await expect(adminPage.locator(".ant-message-notice:visible")).toHaveCount(0);
    await view.screenshot("workspace-product-certificate-management");
  });
  test("TC-39d: 三条待开通产品和长编号反复切换保持可读列宽", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    const number = "ORI_f86ab14998d562b9e504edd3dad3cefe";
    await adminPage.route("**/x/idc-core/api/v1/admin/customers/*/products**", async route => {
      await route.fulfill({ json: { code: 0, message: "success", data: {
        total: 3,
        items: ["pending_payment", "paid", "paid"].map((status, index) => ({
          number: index === 0 ? number : `ORI_${index}${"a".repeat(32)}`,
          nameZhCn: "一次性支付协议验收", nameEnUs: "One-time product",
          information: "", informationEnUs: "", deliveryKind: "pending",
          pluginId: "", productType: "other", amountMinor: index === 0 ? "200" : "100",
          currencyCode: "CNY", periodCount: 1, periodUnit: "one_time",
          createdAt: 1791772800000, expiresAt: 0, status,
        })),
      } } });
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    for (let index = 0; index < 3; index++) {
      await adminPage.setViewportSize({ width: index === 1 ? 900 : 1280, height: 800 });
      await view.tab("产品");
      const firstRow = view.products.locator(".vxe-table--main-wrapper .vxe-body--row").first();
      await expect(firstRow).toBeVisible();
      await expect.poll(async () => firstRow.locator("td").first().evaluate(el => el.getBoundingClientRect().width)).toBeGreaterThanOrEqual(220);
      await expect.poll(async () => firstRow.locator("td").nth(2).evaluate(el => el.getBoundingClientRect().width)).toBeGreaterThanOrEqual(150);
      await expect(firstRow).toContainText("CNY 2.00");
      await expect(firstRow.getByRole("button", { name: number, exact: true })).toHaveCSS("white-space", "nowrap");
      await expect(view.products).toContainText("产品名称");
      await view.screenshot(`workspace-sparse-products-${index}`);
      await view.tab("客户概览");
    }
  });
  test("TC-39a: 逐币种钱包、实际收支与产品分页，备注和配置使用对应流程", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage);
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await expect(view.customerRow(workspaceCustomerNo)).toContainText(
      "CNY 170.00",
    );
    await expect(view.customerRow(workspaceCustomerNo)).toContainText(
      "USD 30.00",
    );
    await expect(view.customerRow(workspaceCustomerNo)).toContainText("赠送金");
    await view.openOverview(workspaceCustomerNo);
    const statistics = adminPage.getByTestId("customer-statistics");
    const productStatistics = adminPage.getByTestId(
      "customer-product-statistics",
    );
    await expect(productStatistics.getByRole("heading")).toHaveText(
      "产品/服务",
    );
    await expect(productStatistics).toContainText("即将到期");
    await expect(productStatistics).toContainText("已到期");
    await expect(
      adminPage.getByTestId("customer-category-cloudServer"),
    ).toHaveText("22云服务器");
    await expect(adminPage.getByTestId("customer-category-ssl")).toHaveText(
      "1SSL证书",
    );
    await expect(productStatistics).not.toContainText("魔方云");
    await expect(statistics).toContainText("总收入");
    await expect(statistics).toContainText("CNY 120.00");
    await expect(statistics).toContainText("USD 50.00");
    await expect(statistics).toContainText("CNY 15.00");
    await expect(statistics).toContainText("CNY 70.00");
    await view.screenshot("workspace-overview");
    await view.showProductStatistics();
    await view.screenshot("workspace-product-statistics");
    const width = (await view.overviewDrawer.boundingBox())?.width;
    await view.viewProductsFromOverview();
    await expect(view.products).toContainText("单域名SSL证书");
    await expect(view.products).toContainText("4核 / 8GB内存");
    await expect(view.products).toContainText("使用中");
    await expect(view.products).toContainText("金额 / 周期");
    await expect(view.products).toContainText("订购 / 到期");
    await expect(
      view.products
        .getByRole("button", { name: "SVC_SERVER", exact: true })
        .first(),
    ).toBeVisible();
    expect((await view.overviewDrawer.boundingBox())?.width).toBe(width);
    const firstProduct = view.products.locator(".vxe-table--main-wrapper .vxe-body--row").first();
    const nameCell = firstProduct.locator(".product-cell").first();
    const cellBounds = await nameCell.boundingBox();
    const rowBounds = await firstProduct.boundingBox();
    expect(cellBounds && rowBounds && cellBounds.y >= rowBounds.y && cellBounds.y + cellBounds.height <= rowBounds.y + rowBounds.height).toBe(true);
    await view.screenshot("workspace-products");
    await view.editServer();
    await expect(view.editor).toContainText("调整配置");
    await expect(view.editor).toContainText("处理器");
    await expect(view.editor).toContainText("4核");
    await view.saveNote("客户服务器备注");
    await expect.poll(() => state.metadata.length).toBe(1);
    expect(state.metadata[0]).toMatchObject({
      remark: "客户服务器备注",
      expectedVersion: 8,
    });
    expect(state.metadata[0]).not.toHaveProperty("amountMinor");
    await view.previewChange();
    await expect(view.editor).toContainText("CNY 20.00");
    await expect(view.editor).toContainText("升级需客户支付账单后生效。");
    await view.screenshot("workspace-server-quote");
    await view.confirmChange(false);
    expect(state.checkouts).toHaveLength(0);
    await view.confirmChange();
    await expect.poll(() => state.checkouts.length).toBe(1);
    expect(state.quotes[0]).toMatchObject({
      currencyCode: "CNY",
      revision: "revision-current",
      selections: [{ optionNo: "OPT_4", quantity: 1 }],
    });
    await expect(view.editor).toBeHidden();
    await view.nextProducts();
    await expect
      .poll(() => state.productPages.at(-1)?.searchParams.get("page"))
      .toBe("2");
    await expect(
      view.products.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
  });
  test("TC-39b: 全部调用分页，确认和失败重置，密钥仅本次展示", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage);
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    await view.tab("API概览");
    await expect(view.api).toContainText("APP_CUSTOMER");
    await expect(view.api).toContainText("查询支持功能");
    await expect(view.api).toContainText("查询商品");
    await view.screenshot("workspace-api");
    await view.nextCalls();
    await view.nextCalls();
    await expect
      .poll(() => state.callPages.at(-1)?.searchParams.get("page"))
      .toBe("3");
    await expect(
      view.api.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(3);
    await view.resetSecret(false);
    expect(state.resets).toHaveLength(0);
    state.resetFailures = 1;
    await view.resetSecret();
    await expect.poll(() => state.resets.length).toBe(1);
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveCount(0);
    await view.screenshot("workspace-api-reset-error");
    // The failed confirmation remains available for a deliberate retry.
    await view.retrySecretConfirm();
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveText(
      "new-secret-visible-once",
    );
    expect(state.resets[1]).toEqual({ tokenId: 42 });
    await view.screenshot("workspace-api-secret");
    await view.tab("产品");
    await view.tab("API概览");
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveCount(0);
  });
  test("TC-39c: 插件停用隐藏操作，加载失败可重试，英文窄屏稳定", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage, {
      channel: false,
      resource: false,
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    await expect(
      view.overviewDrawer.getByRole("tab", { name: "API概览", exact: true }),
    ).toHaveCount(0);
    state.productFailures = 1;
    await view.tab("产品");
    await expect(view.products).toContainText("加载失败");
    await view.retryProducts();
    await expect(view.products).toContainText("云服务器");
    await expect(
      view.products.getByTestId("customer-product-edit-SVC_SERVER"),
    ).toHaveCount(0);
    await view.screenshot("workspace-products-plugin-disabled");
    await view.closeOverview();
    const layout = new MainLayout(adminPage);
    await layout.switchLanguage("English");
    await adminPage.setViewportSize({ width: 900, height: 800 });
    await view.openOverview(workspaceCustomerNo);
    await expect(
      adminPage.getByTestId("customer-product-statistics").getByRole("heading"),
    ).toHaveText("Products & services");
    await view.tab("Products");
    await expect(view.products).toContainText("Amount / cycle");
    await expect(view.products).toContainText("Ordered / expires");
    await expect(view.products).toContainText("Product information");
    await expect(view.products).not.toContainText("plugin.idc-core");
    await view.screenshot("workspace-products-en-narrow");
  });
});
