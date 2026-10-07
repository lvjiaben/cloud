import type { Page, Route } from "@host-tests/support/playwright";

type Body = Record<string, unknown>;

export const lifecycleInvoiceNo = "INV-LIFECYCLE-0001";
export const lifecycleOrderNo = "ORD-LIFECYCLE-0001";
export const lifecycleServiceNo = "SVC-LIFECYCLE-0001";

const now = 1_787_270_400_000;

function envelope(data: unknown) {
  return { code: 0, data, message: "OK" };
}

async function fulfill(route: Route, data: unknown, status = 200) {
  await route.fulfill({ json: envelope(data), status });
}

async function installCustomerShellMock(page: Page) {
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/config/public/frontend")) {
      await fulfill(route, {
        app: { name: "IDC 客户中心" },
        auth: {},
        ui: {},
        workspace: { basePath: "/" },
      });
      return;
    }
    if (path.endsWith("/i18n/runtime/locales")) {
      await fulfill(route, {
        enabled: true,
        items: [
          {
            isDefault: true,
            locale: "zh-CN",
            name: "简体中文",
            nativeName: "简体中文",
          },
          {
            isDefault: false,
            locale: "en-US",
            name: "English",
            nativeName: "English",
          },
        ],
        locale: "zh-CN",
      });
      return;
    }
    if (path.endsWith("/i18n/runtime/messages")) {
      await fulfill(route, { messages: {} });
      return;
    }
    if (path.endsWith("/auth/login")) {
      await fulfill(route, {
        accessToken: "customer-lifecycle-e2e-token",
        refreshToken: "customer-lifecycle-e2e-refresh",
        tenants: [],
      });
      return;
    }
    if (path.endsWith("/user/info")) {
      await fulfill(route, {
        avatar: "",
        homePath: "/account",
        id: 1001,
        realName: "测试客户",
        roles: [],
        username: "customer",
      });
      return;
    }
    await fulfill(route, {});
  });
}

function invoice(paid: boolean) {
  return {
    canMerge: false,
    canPayFromBalance: !paid,
    canPayFromCredit: false,
    createdAt: now,
    currencyCode: "CNY",
    invoiceNo: lifecycleInvoiceNo,
    issuedAt: now,
    itemCount: 1,
    orderNo: lifecycleOrderNo,
    outstandingAmountMinor: paid ? "0" : "8800",
    paidAmountMinor: paid ? "8800" : "0",
    paymentDueAt: now + 86_400_000,
    refundedAmountMinor: "0",
    refundableAmountMinor: paid ? "8800" : "0",
    settledAt: paid ? now + 1_000 : undefined,
    sourceCode: "order",
    statusCode: paid ? "paid" : "pending_payment",
    totalAmountMinor: "8800",
    waivedAmountMinor: "0",
    updatedAt: now,
  };
}

function invoiceDetail(paid: boolean) {
  return {
    invoice: invoice(paid),
    items: [
      {
        descriptionEnUS: "Lifecycle cloud server",
        descriptionZhCN: "生命周期云服务器",
        discountMinor: "0",
        invoiceItemNo: "INVITEM-LIFECYCLE-0001",
        itemTypeCode: "server_purchase",
        lineNumber: 1,
        netAmountMinor: "8800",
        sourceReference: lifecycleOrderNo,
        subtotalMinor: "8800",
        taxAmountMinor: "0",
        totalAmountMinor: "8800",
      },
    ],
    nextItemCursor: "",
  };
}

function commerceOrder(provisioningRead: number) {
  const provisioning = provisioningRead <= 1;
  return {
    deliveries: [
      {
        completedAt: provisioning ? undefined : now + 2_000,
        ongoingService: true,
        productNameEnUS: "Lifecycle cloud server",
        productNameZhCN: "生命周期云服务器",
        quantity: 1,
        statusCode: provisioning ? "processing" : "completed",
        updatedAt: now + 1_000,
      },
    ],
    invoice: invoice(true),
    items: [
      {
        orderItemNo: "OI-LIFECYCLE-0001",
        productNameEnUS: "Lifecycle cloud server",
        productNameZhCN: "生命周期云服务器",
        purchaseCycles: 1,
        quantity: 1,
        totalAmountMinor: "8800",
      },
    ],
    paymentPreparation: {
      contextNo: "PAYCTX-LIFECYCLE-0001",
      identityStatusCode: "verified",
      lastEvaluatedAt: now,
      reasonCode: "ready",
      statusCode: "ready",
    },
    reviews: [],
    serviceEntitlements: provisioning
      ? []
      : [
          {
            entitlementNo: "ENT-LIFECYCLE-0001",
            orderItemNo: "OI-LIFECYCLE-0001",
            productNameEnUS: "Lifecycle cloud server",
            productNameZhCN: "生命周期云服务器",
            startedAt: now + 2_000,
            statusCode: "active",
          },
        ],
    summary: {
      canCancel: false,
      canPreparePayment: false,
      createdAt: now,
      currencyCode: "CNY",
      itemCount: 1,
      orderNo: lifecycleOrderNo,
      paymentDueAt: now + 86_400_000,
      requiresContract: false,
      riskDecisionCode: "approved",
      statusCode: provisioning ? "provisioning" : "completed",
      totalAmountMinor: "8800",
    },
  };
}

function actionList(runtimeStatus: string) {
  return runtimeStatus === "stopped"
    ? ["start", "reset_password", "reinstall", "ip_change"]
    : ["stop", "reboot", "reset_password", "reinstall", "ip_change"];
}

function operation(action: string, status = "succeeded") {
  return {
    action,
    completedAt: status === "succeeded" ? now + 1_000 : undefined,
    confidentialOutputAvailable: false,
    createdAt: now,
    failureCode: "",
    operationNo: `OP-${action.toUpperCase().replaceAll("_", "-")}-0001`,
    status,
    updatedAt: now + 1_000,
  };
}

export interface ServerLifecycleMockState {
  authorizationBodies: Body[];
  balancePaymentBodies: Body[];
  checkoutBodies: Body[];
  commerceReads: number;
  operationBodies: Body[];
  quoteBodies: Body[];
}

export async function installServerLifecycleMock(page: Page) {
  await installCustomerShellMock(page);
  const state: ServerLifecycleMockState = {
    authorizationBodies: [],
    balancePaymentBodies: [],
    checkoutBodies: [],
    commerceReads: 0,
    operationBodies: [],
    quoteBodies: [],
  };
  let paid = false;
  let runtimeStatus = "running";
  let activeOperation: Body | undefined;
  let ipAddress = "203.0.113.10";
  const operations: Body[] = [];

  await page.route("**/x/idc-core/api/v1/account/billing/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    if (
      path.endsWith(
        `/account/billing/invoices/${lifecycleInvoiceNo}/balance-payments`,
      ) &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Body;
      state.balancePaymentBodies.push(body);
      paid = true;
      await fulfill(route, {
        payment: { invoiceNo: lifecycleInvoiceNo, statusCode: "paid" },
      });
      return;
    }
    if (
      path.endsWith(`/account/billing/invoices/${lifecycleInvoiceNo}`) &&
      method === "GET"
    ) {
      await fulfill(route, { detail: invoiceDetail(paid) });
      return;
    }
    await route.fallback();
  });

  await page.route(
    "**/x/idc-core/api/v1/account/commerce/**",
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (
        path.endsWith(`/account/commerce/orders/${lifecycleOrderNo}`) &&
        route.request().method() === "GET" &&
        paid
      ) {
        state.commerceReads += 1;
        await fulfill(route, { order: commerceOrder(state.commerceReads) });
        return;
      }
      await route.fallback();
    },
  );

  await page.route("**/x/idc-core/api/v1/account/server**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const server = {
      activeOperation,
      activeRefundNo: "",
      autoRenewEnabled: true,
      availableActions: actionList(runtimeStatus),
      capabilityObservedAt: now,
      customerAlias: "",
      customerRemark: "",
      entitlementExpiresAt: now + 365 * 86_400_000,
      entitlementStartedAt: now - 30 * 86_400_000,
      entitlementStatus: "active",
      periodCount: 1,
      periodUnit: "month",
      productNameEnUS: "Lifecycle cloud server",
      productNameZhCN: "生命周期云服务器",
      productType: "cloud_server",
      providerTaskStatus: activeOperation ? "accepted" : "succeeded",
      renewalCycles: 1,
      runtimeStatus,
      serviceNo: lifecycleServiceNo,
      updatedAt: now,
      version: 8,
    };

    if (path.endsWith("/account/servers") && method === "GET") {
      await fulfill(route, { items: [server], nextCursor: "" });
      return;
    }
    if (
      path.endsWith(`/${lifecycleServiceNo}/operation-authorizations`) &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Body;
      state.authorizationBodies.push(body);
      const required =
        body.action === "reinstall" || body.action === "ip_change";
      await fulfill(route, {
        expiresAt: required ? now + 120_000 : undefined,
        proofToken: required ? "lifecycle-proof" : "",
        required,
      });
      return;
    }
    if (
      path.endsWith(`/${lifecycleServiceNo}/operations`) &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Body;
      state.operationBodies.push(body);
      const action = String(body.action || "");
      if (activeOperation) {
        await route.fulfill({
          json: { code: 409, data: null, message: "operation in progress" },
          status: 409,
        });
        return;
      }
      if (action === "stop") runtimeStatus = "stopped";
      if (action === "start") runtimeStatus = "running";
      const created = operation(
        action,
        action === "reinstall" ? "executing" : "succeeded",
      );
      operations.unshift(created);
      if (action === "reinstall") activeOperation = created;
      await fulfill(route, { operation: created });
      return;
    }
    if (
      path.endsWith(`/${lifecycleServiceNo}/ip-change-quotes`) &&
      method === "POST"
    ) {
      state.quoteBodies.push(request.postDataJSON() as Body);
      await fulfill(route, {
        quote: {
          amountMinor: 0,
          currencyCode: "CNY",
          expiresAt: now + 300_000,
          freeChange: true,
          ipQuantity: 1,
          quoteNo: "IPQ-LIFECYCLE-0001",
          serviceNo: lifecycleServiceNo,
        },
      });
      return;
    }
    if (
      path.endsWith(
        "/account/server-ip-change-quotes/IPQ-LIFECYCLE-0001/checkout",
      ) &&
      method === "POST"
    ) {
      state.checkoutBodies.push(request.postDataJSON() as Body);
      ipAddress = "198.51.100.28";
      operations.unshift(operation("ip_change"));
      await fulfill(route, {
        checkout: {
          amountMinor: 0,
          currencyCode: "CNY",
          invoiceNo: "",
          operationNo: "OP-IP-CHANGE-0001",
          paymentDueAt: 0,
          paymentRequired: false,
          serviceNo: lifecycleServiceNo,
          status: "ready",
        },
      });
      return;
    }
    if (path.endsWith(`/${lifecycleServiceNo}`) && method === "GET") {
      await fulfill(route, {
        detail: {
          accessories: [
            {
              kind: "ip",
              name: "Primary IP",
              objectNo: "ip-lifecycle-0001",
              projection: { address: ipAddress },
              state: "active",
            },
          ],
          accessoriesExpiresAt: now + 300_000,
          accessoriesObservedAt: now,
          configuration: [],
          operations,
          reinstallImages: [
            {
              arch: "x86_64",
              bootMode: "UEFI",
              group: "Linux",
              name: "Ubuntu 24.04 LTS",
              osType: "Linux",
              reference: "101",
              selectable: true,
            },
          ],
          reinstallImagesObservedAt: now,
          server,
        },
      });
      return;
    }
    await fulfill(route, {}, 404);
  });
  return state;
}
