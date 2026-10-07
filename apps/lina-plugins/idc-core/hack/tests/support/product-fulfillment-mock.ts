import type { Page, Route } from "@host-tests/support/playwright";

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

const now = 1_788_000_000_123;

function taskDetail() {
  return {
    canManualConclude: false,
    canReviewApprove: true,
    canReviewReject: true,
    canTakeover: false,
    customerNo: "CUS_fulfillment_customer_0001",
    customerCode: "10009",
    customerDisplayName: "履约测试客户",
    customerPrimaryPhone: "139****8009",
    events: [
      {
        eventCode: "intent_created",
        occurredAt: now,
      },
    ],
    financiallySettled: true,
    fulfillmentModeCode: "approval_then_plugin",
    granularityCode: "per_quantity_unit",
    intentNo: "PFI-MOCK-1",
    manualCompletedReasons: [],
    manualUnableReasons: [],
    orderItemNo: "ITEM-MOCK-1",
    orderNo: "ORD-MOCK-1",
    productNameEnUS: "Certificate",
    productNameZhCN: "证书",
    quantity: 2,
    reviewApprovalReasons: [
      { code: "verified", labelEnUS: "Verified", labelZhCN: "已核对" },
    ],
    reviewRejectionReasons: [
      {
        code: "needs_manual_handling",
        labelEnUS: "Needs manual handling",
        labelZhCN: "需要人工处理",
      },
    ],
    statusCode: "awaiting_review",
    takeoverReasons: [],
    unitSequence: 1,
    updatedAt: now,
    version: 3,
  };
}

export interface ProductFulfillmentMockState {
  actionBodies: Array<Record<string, unknown>>;
  listRequests: URL[];
  customerListRequests: URL[];
}

export async function installProductFulfillmentMock(
  page: Page,
  options: { loseFirstActionResponse?: boolean } = {},
) {
  const state: ProductFulfillmentMockState = {
    actionBodies: [],
    listRequests: [],
    customerListRequests: [],
  };
  let loseFirstActionResponse = options.loseFirstActionResponse ?? false;

  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route: Route) => {
    const url = new URL(route.request().url());
    state.customerListRequests.push(url);
    await route.fulfill({ json: envelope({ customers: [{ customerNo: "CUS_fulfillment_customer_0001", customerCode: "10009", displayName: "履约测试客户", primaryPhone: "139****8009" }], nextCursor: "" }) });
  });

  await page.route("**/api/v1/menus/all", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { list?: Array<Record<string, unknown>> };
    };
    if (!payload.data) throw new Error("menu payload is unavailable");
    const list = payload.data.list ?? [];
    list.push({
      component: "#/views/system/plugin/dynamic-page",
      id: -15_011,
      meta: { title: "商品履约任务" },
      name: "ProductFulfillmentMockContract",
      parentId: 0,
      path: "/idc/product-fulfillment/tasks",
    });
    payload.data.list = list;
    await route.fulfill({ json: payload, response });
  });

  await page.route(
    "**/x/idc-core/api/v1/admin/product-fulfillment/tasks**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const method = request.method();
      const suffix = url.pathname.split("/product-fulfillment/tasks")[1] ?? "";
      if (method === "GET" && suffix === "") {
        state.listRequests.push(url);
        await route.fulfill({
          json: envelope({ nextCursor: "", tasks: Number(url.searchParams.get("page") || 1) === 1 ? [taskDetail()] : [], total: 1 }),
        });
        return;
      }
      if (method === "GET" && suffix === "/PFI-MOCK-1") {
        await route.fulfill({ json: envelope({ task: taskDetail() }) });
        return;
      }
      if (method === "POST" && suffix.endsWith("/review-approvals")) {
        state.actionBodies.push(request.postDataJSON() as Record<string, unknown>);
        if (loseFirstActionResponse) {
          loseFirstActionResponse = false;
          await route.abort("connectionreset");
          return;
        }
        const updated = taskDetail();
        updated.canReviewApprove = false;
        updated.canReviewReject = false;
        updated.statusCode = "processing";
        updated.version = 4;
        updated.events.push({
          eventCode: "review_approved",
          occurredAt: now + 1_000,
          operatorDisplayName: "运营甲",
          reasonCode: "verified",
          reasonLabelEnUS: "Verified",
          reasonLabelZhCN: "已核对",
          safeNote: "",
        } as (typeof updated.events)[number]);
        await route.fulfill({ json: envelope({ task: updated }) });
        return;
      }
      await route.fulfill({ json: { code: 404, message: "not found" }, status: 404 });
    },
  );
  return state;
}
