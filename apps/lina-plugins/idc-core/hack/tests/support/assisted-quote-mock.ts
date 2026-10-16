import type { Page, Route } from "@host-tests/support/playwright";

const requestNo = "AQR_20260811_0001";

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function request(statusCode = "pending_approval") {
  return {
    canApprove: statusCode === "pending_approval",
    canCheckout: statusCode === "approved",
    canReject: statusCode === "pending_approval",
    createdAt: 1_786_406_400_000,
    currencyCode: "CNY",
    customerNo: "CUS_ASSISTED_0001",
    customerCode: "10005",
    customerDisplayName: "协助报价客户",
    customerPrimaryPhone: "13800138005",
    decisionAt: statusCode === "pending_approval" ? undefined : 1_786_406_460_000,
    decisionBy: statusCode === "pending_approval" ? undefined : 2002,
    decisionCode: statusCode === "pending_approval" ? undefined : "approved",
    decisionReason:
      statusCode === "pending_approval" ? undefined : "Independent review complete",
    discountMinor: statusCode === "pending_approval" ? "0" : "10000",
    expiresAt: 1_786_492_800_000,
    internalNote: "Renewal save request",
    items: [
      {
        lineNumber: 1,
        productNameEnUS: "High-memory cloud server",
        productNameZhCN: "高内存云服务器",
        purchaseCycles: 12,
        requestItemNo: "AQI_20260811_0001",
        quantity: 2,
      },
    ],
    manualDiscountMinor: "10000",
    orderNo: statusCode === "ordered" ? "ORD_ASSISTED_0001" : undefined,
    orderStatusCode: statusCode === "ordered" ? "manual_review" : undefined,
    paymentDueSeconds: 172_800,
    paymentTermCode: "extended_prepay",
    quoteNo: statusCode === "pending_approval" ? undefined : "QUO_ASSISTED_0001",
    reason: "Customer renewal retention request",
    requestNo,
    requestedBy: 1001,
    reused: false,
    statusCode,
    subtotalMinor: statusCode === "pending_approval" ? "0" : "900719925474099301",
    totalAmountMinor: statusCode === "pending_approval" ? "0" : "900719925474089301",
  };
}

export interface AssistedQuoteMockState {
  checkoutBodies: Array<Record<string, unknown>>;
  createBodies: Array<Record<string, unknown>>;
  createFailures: number;
  created: boolean;
  customerRequests: URL[];
  decisionBodies: Array<Record<string, unknown>>;
  detailRequests: URL[];
  listRequests: URL[];
}

export async function installAssistedQuoteMock(
  page: Page,
  options: { createFailures?: number; withoutCreate?: boolean; withoutCustomerQuery?: boolean; file?: "required" | "optional" } = {},
): Promise<AssistedQuoteMockState> {
  const state: AssistedQuoteMockState = {
    checkoutBodies: [],
    createBodies: [],
    createFailures: options.createFailures ?? 0,
    created: false,
    customerRequests: [],
    decisionBodies: [],
    detailRequests: [],
    listRequests: [],
  };
  function createdRequest() {
    const body = state.createBodies.at(-1) || {};
    const items = body.items as Array<{ quantity: number; purchaseCycles: number }>;
    return {
      ...request("ordered"), decisionBy: undefined, decisionCode: undefined, decisionReason: undefined, decisionAt: undefined, canCheckout: false, orderStatusCode: "pending_payment", requestNo: "AQR_CREATED_0001", canApprove: false, canReject: false,
      currencyCode: String(body.currencyCode || "CNY"), manualDiscountMinor: String(body.manualDiscountMinor || "0"),
      reason: String(body.reason || ""), internalNote: String(body.internalNote || ""),
      paymentDueSeconds: Number(body.paymentDueSeconds || 0), paymentTermCode: String(body.paymentTermCode || "standard_prepay"),
      createdAt: Date.now(), expiresAt: Number(body.paymentDueAt),
      items: request().items.map((item, index) => ({ ...item, quantity: items?.[index]?.quantity || 1, purchaseCycles: items?.[index]?.purchaseCycles || 1 })),
    };
  }
  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route) => {
    const url = new URL(route.request().url());
    state.customerRequests.push(url);
    if (url.pathname.endsWith("/assisted-order-options")) { await route.fulfill({ json: apiEnvelope({ currencies: ["CNY", "USD"] }) }); return; }
    await route.fulfill({
      json: apiEnvelope({
        customers: [{ customerCode: "10005", customerNo: "CUS_ASSISTED_0001", displayName: "协助报价客户", primaryPhone: "13800138005", status: "active", type: "individual", verification: "unverified", tagCount: 0, createdAt: Date.now() }],
        nextCursor: "", total: 1,
      }),
    });
  });
  if (options.withoutCreate || options.withoutCustomerQuery) {
    // Narrow only the authenticated administrator's visible permissions; no identity is fabricated.
    const permissions = ["idc:admin:commerce:assisted-quote:list", "idc:admin:commerce:assisted-quote:query", "idc:admin:customer:list"];
    if (!options.withoutCreate) permissions.push("idc:admin:commerce:assisted-quote:create");
    await page.route("**/api/v1/user/info", async route => {
      const response = await route.fetch();
      const payload = await response.json();
      payload.data.permissions = permissions;
      await route.fulfill({ response, json: payload });
    });
    const updateStoredPermissions = (codes: string[]) => {
      for (const key of Object.keys(localStorage).filter(key => key.endsWith("-core-access"))) {
        const state = JSON.parse(localStorage.getItem(key) || "{}");
        localStorage.setItem(key, JSON.stringify({ ...state, accessCodes: codes }));
      }
    };
    if (page.url() === "about:blank") await page.addInitScript(updateStoredPermissions, permissions);
    else await page.evaluate(updateStoredPermissions, permissions);
  }
  const fields: Array<Record<string, unknown>> = [{ fieldToken: "field-hostname", type: "single_line", labelZhCN: "主机名称", labelEnUS: "Host name", descriptionZhCN: "", descriptionEnUS: "", visible: true, required: true, disabled: false, minimumLength: 3, maximumLength: 64 }];
  if (options.file) fields.push({ fieldToken: "field-document", type: "file", labelZhCN: "申请资料", labelEnUS: "Supporting document", descriptionZhCN: "", descriptionEnUS: "", visible: true, required: options.file === "required", disabled: false, uploadPolicy: { id: "file-policy", allowedExtensions: [".pdf"], allowedMimeTypes: ["application/pdf"], maximumFileBytes: 1048576, maximumFiles: 1, maximumTotalBytes: 1048576, safetyScanRequired: false } });
  await page.route("**/x/idc-core/api/v1/public/catalog/products**", async route => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/purchase-configuration/evaluate")) {
      const values = route.request().postDataJSON().values || [];
      const valid = values.some((value: { textValue?: string }) => (value.textValue || "").length >= 3);
      await route.fulfill({ json: apiEnvelope({ configurationRevision: "revision-test-1", fields, values, valid, issues: valid ? [] : [{ fieldToken: "field-hostname", issueCode: "required" }] }) });
    } else if (url.pathname.endsWith("/purchase-configuration/quote")) {
      await route.fulfill({ json: apiEnvelope({ terms: [{ billingTermToken: "term-monthly", labelZhCN: "每月", labelEnUS: "Monthly", minimumPurchaseCycles: 1, status: "final", setupAmount: "0", subtotalAmount: "10000", taxAmount: "0", totalAmount: "10000" }] }) });
    } else if (url.pathname.endsWith("/purchase-configuration")) {
      await route.fulfill({ json: apiEnvelope({ configurationRevision: "revision-test-1", fields, values: [] }) });
    } else {
      await route.fulfill({ json: apiEnvelope({ availableCurrencies: ["CNY", "USD"], nextCursor: "", products: [{ publicSlug: "assisted-server", nameZhCN: "高内存云服务器", nameEnUS: "High-memory cloud server", currencies: ["CNY", "USD"], availability: "available" }] }) });
    }
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/commerce/assisted-quotes**",
    async (route: Route) => {
      const httpRequest = route.request();
      const url = new URL(httpRequest.url());
      if (httpRequest.method() === "POST" && url.pathname.endsWith("/assisted-quotes")) {
        state.createBodies.push(httpRequest.postDataJSON() as Record<string, unknown>);
        if (state.createFailures > 0) {
          state.createFailures--;
          await route.fulfill({ json: { code: 500, message: "报价提交暂时失败，请重试", data: null } });
          return;
        }
        state.created = true;
        await route.fulfill({ json: apiEnvelope({ request: createdRequest() }) });
        return;
      }
      if (url.pathname.endsWith(`/${requestNo}/approval`)) {
        state.decisionBodies.push(httpRequest.postDataJSON() as Record<string, unknown>);
        await route.fulfill({ json: apiEnvelope({ request: request("approved") }) });
        return;
      }
      if (url.pathname.endsWith(`/${requestNo}/checkout`)) {
        state.checkoutBodies.push(httpRequest.postDataJSON() as Record<string, unknown>);
        await route.fulfill({ json: apiEnvelope({ request: request("ordered") }) });
        return;
      }
      if (url.pathname.endsWith("/AQR_CREATED_0001")) {
        state.detailRequests.push(url);
        await route.fulfill({ json: apiEnvelope({ request: createdRequest() }) });
        return;
      }
      if (url.pathname.endsWith(`/${requestNo}`)) {
        state.detailRequests.push(url);
        await route.fulfill({ json: apiEnvelope({ request: request() }) });
        return;
      }
      state.listRequests.push(url);
      const directory = Array.from({ length: 11 }, (_, index) => ({
        ...request(),
        requestNo: index === 0 ? requestNo : `AQR_20260811_${String(index + 1).padStart(4, "0")}`,
      }));
      if (state.created) directory.unshift(createdRequest());
      const pageNumber = Number(url.searchParams.get("page") || 1);
      const pageSize = Number(url.searchParams.get("limit") || 10);
      await route.fulfill({
        json: apiEnvelope({
          nextCursor: "",
          requests: directory.slice((pageNumber - 1) * pageSize, pageNumber * pageSize),
          total: directory.length,
        }),
      });
    },
  );
  return state;
}

export const assistedRequestNo = requestNo;
