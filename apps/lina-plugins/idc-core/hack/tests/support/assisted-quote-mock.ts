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
  customerRequests: URL[];
  decisionBodies: Array<Record<string, unknown>>;
  detailRequests: URL[];
  listRequests: URL[];
}

export async function installAssistedQuoteMock(
  page: Page,
): Promise<AssistedQuoteMockState> {
  const state: AssistedQuoteMockState = {
    checkoutBodies: [],
    customerRequests: [],
    decisionBodies: [],
    detailRequests: [],
    listRequests: [],
  };
  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route) => {
    const url = new URL(route.request().url());
    state.customerRequests.push(url);
    await route.fulfill({
      json: apiEnvelope({
        customers: [{ customerCode: "10005", customerNo: "CUS_ASSISTED_0001", displayName: "协助报价客户", primaryPhone: "13800138005" }],
        nextCursor: "",
      }),
    });
  });
  await page.route("**/x/idc-core/api/v1/public/catalog/products**", async (route) => {
    await route.fulfill({
      json: apiEnvelope({ availableCurrencies: ["CNY"], nextCursor: "", products: [] }),
    });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/commerce/assisted-quotes**",
    async (route: Route) => {
      const httpRequest = route.request();
      const url = new URL(httpRequest.url());
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
