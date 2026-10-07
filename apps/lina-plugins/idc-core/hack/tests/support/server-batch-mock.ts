import type { Page } from "@host-tests/support/playwright";

type Body = Record<string, unknown>;

const now = 1_787_184_000_000;

function envelope(data: unknown) {
  return { code: 0, data, message: "OK" };
}

function item(serviceNo: string, lineNumber: number, dangerous: boolean) {
  return {
    actionAmountMinor: dangerous ? 600 : 0,
    currencyCode: "CNY",
    customerNo: `CUS-00${lineNumber}`,
    entitlementExpiresAt: now + lineNumber * 86_400_000,
    entitlementStatus: "active",
    executionStatus: "pending",
    failureCode: "",
    irreversible: dangerous,
    lineNumber,
    notificationEventNo: "",
    operationNo: "",
    previewStatus: "eligible",
    providerStatus: "none",
    resourceSummary: `cloud_server / pool-${lineNumber}`,
    runtimeStatus: "running",
    serviceNo,
    serviceVersion: 3,
    unpaidAmountMinor: lineNumber * 1200,
  };
}

export async function installServerBatchMock(page: Page) {
  const state = {
    classificationWrites: [] as Body[],
    confirmations: [] as Body[],
    executions: [] as Array<{ body: Body; path: string }>,
    previews: [] as Array<{ body: Body; path: string }>,
  };
  const classifications = [{ classificationNo: "scl_core", nameEnUS: "Business critical", nameZhCN: "核心业务", status: "active", version: 1 }];
  await page.route("**/x/idc-core/api/v1/admin/resource/server-classifications**", async (route) => {
    const request = route.request();
    const body = request.postDataJSON() as Body;
    if (request.method() === "GET") {
      await route.fulfill({ json: envelope({ items: classifications }) });
      return;
    }
    state.classificationWrites.push(body);
    await route.fulfill({ json: envelope({ item: classifications[0] }) });
  });
  await page.route("**/x/idc-core/api/v1/admin/resource/server-batches/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const body = request.postDataJSON() as Body;
    if (path.endsWith("/safe/previews") || path.endsWith("/dangerous/previews")) {
      state.previews.push({ body, path });
      const dangerous = path.endsWith("/dangerous/previews");
      const inputItems = body.items as Array<{ serviceNo: string }>;
      await route.fulfill({
        json: envelope({
          batch: {
            action: body.action,
            batchNo: dangerous ? "sbt_danger" : "sbt_safe",
            createdAt: now,
            dangerous,
            items: inputItems.map((entry, index) => item(entry.serviceNo, index + 1, dangerous)),
            previewDigest: "a".repeat(64),
            status: "previewed",
          },
        }),
      });
      return;
    }
    if (path.endsWith("/confirmation")) {
      state.confirmations.push(body);
      await route.fulfill({ json: envelope({ expiresAt: now + 120_000, proofToken: "recent-login-proof" }) });
      return;
    }
    if (path.endsWith("-executions")) {
      state.executions.push({ body, path });
      const dangerous = path.endsWith("/dangerous-executions");
      await route.fulfill({
        json: envelope({
          batch: {
            action: dangerous ? "terminate" : "renewal_reminder",
            batchNo: dangerous ? "sbt_danger" : "sbt_safe",
            createdAt: now,
            dangerous,
            items: [
              { ...item("SVC-SERVER-0001", 1, dangerous), executionStatus: "succeeded", operationNo: "operation-1" },
              { ...item("SVC-SERVER-0002", 2, dangerous), executionStatus: dangerous ? "manual_review" : "failed", failureCode: "upstream_unavailable" },
            ],
            previewDigest: "a".repeat(64),
            status: "completed",
          },
        }),
      });
      return;
    }
    await route.fallback();
  });
  return state;
}
