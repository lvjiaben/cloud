import type { Page, Route } from "@host-tests/support/playwright";

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

const now = 1_788_000_000_123;

function taskDetail(manual = false) {
  return {
    managementPreparation: undefined as
      | {
          statusCode: "preparing" | "ready" | "needs_attention";
          attemptCount: number;
          failureCode?: string;
        }
      | undefined,
    execution: undefined as
      | {
          statusCode: string;
          phaseCode: string;
          lastResultCode: string;
          failureCode?: string;
          responseExcerpt?: string;
          attemptCount: number;
        }
      | undefined,
    canRepairManagement: false,
    canVerifyNonAcceptance: false,
    canRequestRefund: false,
    nonAcceptanceVerifications: [] as Array<{
      providerTaskNo: string;
      actorUserId: number;
      note: string;
      verifiedAt: number;
    }>,
    canManualConclude: manual,
    canReviewApprove: !manual,
    canReviewReject: !manual,
    canTakeover: false,
    canRecoverPreparation: false,
    recoveryReasons: [
      {
        code: "preparation_resolved",
        labelEnUS: "Preparation repaired",
        labelZhCN: "开通准备问题已处理",
      },
    ],
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
    recoveryConfigurations: [] as Array<{
      configuration: Record<string, unknown>;
      occurredAt: number;
    }>,
    financiallySettled: true,
    fulfillmentModeCode: manual ? "manual" : "approval_then_plugin",
    granularityCode: "per_quantity_unit",
    intentNo: "PFI-MOCK-1",
    manualCompletedReasons: manual
      ? [
          {
            code: "manual_fulfillment_completed",
            labelEnUS: "Delivered",
            labelZhCN: "人工履约已完成",
          },
        ]
      : [],
    manualUnableReasons: manual
      ? [
          {
            code: "manual_fulfillment_unable",
            labelEnUS: "Supplier unavailable",
            labelZhCN: "无法完成履约",
          },
        ]
      : [],
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
    statusCode: manual ? "manual_processing" : "awaiting_review",
    takeoverReasons: [],
    unitSequence: 1,
    updatedAt: now,
    version: 3,
  };
}

export interface ProductFulfillmentMockState {
  actionBodies: Array<Record<string, unknown>>;
  recoveryConfigurationRequests: URL[];
  failRecoveryConfiguration: boolean;
  listRequests: URL[];
  customerListRequests: URL[];
  failNextList: boolean;
}

export async function installProductFulfillmentMock(
  page: Page,
  options: {
    managementPreparation?: {
      statusCode: "preparing" | "ready" | "needs_attention";
      attemptCount: number;
      failureCode?: string;
    };
    orderNo?: string;
    managementRepair?: boolean;
    strictOrderFilter?: boolean;
    loseFirstActionResponse?: boolean;
    manual?: boolean;
    recovery?: boolean;
    recoveryUnavailable?: boolean;
    diagnosticCode?: string;
    failRecoveryConfiguration?: boolean;
    responseExcerpt?: string;
    verification?: boolean;
  } = {},
) {
  const state: ProductFulfillmentMockState = {
    actionBodies: [],
    recoveryConfigurationRequests: [],
    failRecoveryConfiguration: options.failRecoveryConfiguration ?? false,
    listRequests: [],
    customerListRequests: [],
    failNextList: false,
  };
  let loseFirstActionResponse = options.loseFirstActionResponse ?? false;
  let currentTask = taskDetail(options.manual);
  if (options.orderNo) currentTask.orderNo = options.orderNo;
  if (options.managementPreparation)
    currentTask = {
      ...currentTask,
      statusCode: "completed",
      canReviewApprove: false,
      canReviewReject: false,
      managementPreparation: options.managementPreparation,
      canRepairManagement: options.managementRepair ?? false,
    };
  if (options.recovery || options.recoveryUnavailable) {
    currentTask = {
      ...currentTask,
      canReviewApprove: false,
      canReviewReject: false,
      canRecoverPreparation: !options.recoveryUnavailable,
      canRequestRefund: !options.recoveryUnavailable,
      statusCode: options.recoveryUnavailable
        ? "manual_processing"
        : "not_completed",
      fulfillmentModeCode: "payment_confirmed_plugin",
    };
  }

  if (options.recovery || options.recoveryUnavailable) {
    currentTask.execution = {
      statusCode: options.recoveryUnavailable ? "result_unknown" : "failed",
      phaseCode: options.recoveryUnavailable
        ? "frozen_unknown"
        : "post_submit_query",
      lastResultCode: "provider_result",
      failureCode:
        options.diagnosticCode ??
        (options.recoveryUnavailable ? undefined : "upstream_request_rejected"),
      responseExcerpt: options.responseExcerpt,
      attemptCount: 3,
    };
  }

  if (options.verification) currentTask.canVerifyNonAcceptance = true;

  await page.route(
    "**/x/idc-core/api/v1/admin/customers**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.customerListRequests.push(url);
      await route.fulfill({
        json: envelope({
          customers: [
            {
              customerNo: "CUS_fulfillment_customer_0001",
              customerCode: "10009",
              displayName: "履约测试客户",
              primaryPhone: "139****8009",
            },
          ],
          nextCursor: "",
        }),
      });
    },
  );

  await page.route("**/api/v1/menus/all", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { list?: Array<Record<string, unknown>> };
    };
    if (!payload.data) throw new Error("menu payload is unavailable");
    const list = payload.data.list ?? [];
    const queue = [...list];
    let hasTaskMenu = false;
    while (queue.length) {
      const item = queue.shift()!;
      if (item.path === "/idc/product-fulfillment/tasks") hasTaskMenu = true;
      if (Array.isArray(item.children))
        queue.push(...(item.children as Array<Record<string, unknown>>));
    }
    if (!hasTaskMenu)
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
        if (state.failNextList) {
          state.failNextList = false;
          await route.fulfill({
            status: 503,
            json: {
              code: 503,
              message: "Order fulfillment temporarily unavailable",
            },
          });
          return;
        }
        const matches =
          !options.strictOrderFilter ||
          !url.searchParams.get("orderNo") ||
          url.searchParams.get("orderNo") === currentTask.orderNo;
        await route.fulfill({
          json: envelope({
            nextCursor: "",
            tasks:
              matches && Number(url.searchParams.get("page") || 1) === 1
                ? [currentTask]
                : [],
            total: matches ? 1 : 0,
          }),
        });
        return;
      }
      if (method === "GET" && suffix === "/PFI-MOCK-1") {
        if (url.searchParams.get("includeRecoveryConfiguration")) {
          state.recoveryConfigurationRequests.push(url);
          if (state.failRecoveryConfiguration) {
            await route.fulfill({
              status: 503,
              json: {
                code: 503,
                message: "Upstream configuration unavailable",
              },
            });
            return;
          }
        }
        const selection = {
          interfaceInstanceNo:
            url.searchParams.get("recoveryInterfaceNo") ||
            "pfi_11111111111111111111111111111111",
          interfaceConfigurationRevision:
            "rci_11111111111111111111111111111111",
          region: url.searchParams.has("recoveryRegion")
            ? url.searchParams.get("recoveryRegion")!
            : "1",
          line: url.searchParams.has("recoveryLine")
            ? url.searchParams.get("recoveryLine")!
            : "5",
          operatingSystem: url.searchParams.has("recoveryOperatingSystem")
            ? url.searchParams.get("recoveryOperatingSystem")!
            : "97",
          interfaceLabel: "南京魔方",
          regionLabel: "陕西",
          lineLabel: "西安电信",
          operatingSystemLabel: "Debian 13",
        };
        const editor = {
          original: {
            ...selection,
            region: "1",
            line: "5",
            operatingSystem: "97",
            lineLabel: "西安电信",
            operatingSystemLabel: "Debian 13",
          },
          selection,
          specifications: [
            { key: "cpu", value: "2" },
            { key: "memory", value: "4096" },
            { key: "system_disk", value: "40" },
            { key: "bandwidth", value: "100" },
          ],
          interfaces: [
            {
              value: selection.interfaceInstanceNo,
              labelZhCN: "南京魔方",
              labelEnUS: "Nanjing cloud",
            },
          ],
          regions: [{ value: "1", labelZhCN: "陕西", labelEnUS: "Shaanxi" }],
          lines: [
            { value: "5", labelZhCN: "西安电信", labelEnUS: "Xian telecom" },
            {
              value: "6",
              labelZhCN: "西安备用节点",
              labelEnUS: "Xian secondary node",
            },
          ],
          operatingSystems: [
            { value: "97", labelZhCN: "Debian 13", labelEnUS: "Debian 13" },
            { value: "98", labelZhCN: "Ubuntu 24", labelEnUS: "Ubuntu 24" },
          ],
        };
        if (selection.line === "6") selection.lineLabel = "西安备用节点";
        if (selection.operatingSystem === "98")
          selection.operatingSystemLabel = "Ubuntu 24";
        await route.fulfill({
          json: envelope({
            task: currentTask,
            ...(url.searchParams.get("includeRecoveryConfiguration")
              ? { recoveryConfiguration: editor }
              : {}),
          }),
        });
        return;
      }
      if (method === "POST" && suffix.endsWith("/management-repairs")) {
        state.actionBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        if (loseFirstActionResponse) {
          loseFirstActionResponse = false;
          await route.abort("connectionreset");
          return;
        }
        currentTask = {
          ...currentTask,
          canRepairManagement: false,
          managementPreparation: { statusCode: "preparing", attemptCount: 0 },
        };
        await route.fulfill({ json: envelope({ task: currentTask }) });
        return;
      }
      if (method === "POST" && suffix.endsWith("/review-approvals")) {
        state.actionBodies.push(
          request.postDataJSON() as Record<string, unknown>,
        );
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
        currentTask = updated;
        await route.fulfill({ json: envelope({ task: updated }) });
        return;
      }
      if (
        method === "POST" &&
        suffix === "/PFI-MOCK-1/non-acceptance-verifications"
      ) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.actionBodies.push(body);
        if (!body.confirmedNoCreation || String(body.note).trim().length < 10) {
          await route.fulfill({
            status: 400,
            json: { code: 400, message: "Invalid verification" },
          });
          return;
        }
        if (currentTask.canVerifyNonAcceptance) {
          currentTask = {
            ...currentTask,
            canVerifyNonAcceptance: false,
            canRecoverPreparation: true,
            canRequestRefund: true,
            statusCode: "not_completed",
            version: 4,
          };
          if (currentTask.execution)
            currentTask.execution = {
              ...currentTask.execution,
              statusCode: "failed",
              phaseCode: "post_submit_query",
              lastResultCode: "operator_non_acceptance_verified",
            };
          currentTask.nonAcceptanceVerifications.push({
            providerTaskNo: "task_0123456789abcdef0123456789abcdef",
            actorUserId: 101,
            note: String(body.note),
            verifiedAt: now + 1000,
          });
        }
        if (loseFirstActionResponse) {
          loseFirstActionResponse = false;
          await route.abort("connectionreset");
          return;
        }
        await route.fulfill({ json: envelope({ task: currentTask }) });
        return;
      }
      if (method === "POST" && suffix === "/PFI-MOCK-1/recoveries") {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.actionBodies.push(body);
        if (currentTask.canRecoverPreparation) {
          currentTask = {
            ...currentTask,
            canRecoverPreparation: false,
            statusCode: "processing",
            version: 4,
          };
          if (body.recoveryConfiguration)
            currentTask.recoveryConfigurations.push({
              configuration: structuredClone(
                body.recoveryConfiguration,
              ) as Record<string, unknown>,
              occurredAt: now + 1_000,
            });
          currentTask.events.push({
            eventCode: "preparation_recovered",
            occurredAt: now + 1_000,
            safeNote: body.note,
          } as (typeof currentTask.events)[number]);
        }
        if (loseFirstActionResponse) {
          loseFirstActionResponse = false;
          await route.abort("connectionreset");
          return;
        }
        await route.fulfill({ json: envelope({ task: currentTask }) });
        return;
      }
      if (method === "POST" && suffix === "/PFI-MOCK-1/manual-conclusions") {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.actionBodies.push(body);
        // Model a committed result whose first response is lost; replay must
        // preserve the action identity and must not append a second event.
        if (currentTask.canManualConclude) {
          currentTask = {
            ...currentTask,
            canManualConclude: false,
            statusCode:
              body.outcomeCode === "completed" ? "completed" : "not_completed",
            version: 4,
          };
          currentTask.events.push({
            eventCode:
              body.outcomeCode === "completed"
                ? "manual_completed"
                : "manual_unable_to_complete",
            occurredAt: now + 1_000,
            operatorDisplayName: "运营甲",
            safeNote: body.note,
            reasonCode: body.reasonCode,
            reasonLabelZhCN:
              body.outcomeCode === "completed"
                ? "人工履约已完成"
                : "无法完成履约",
          } as (typeof currentTask.events)[number]);
        }
        if (loseFirstActionResponse) {
          loseFirstActionResponse = false;
          await route.abort("connectionreset");
          return;
        }
        await route.fulfill({ json: envelope({ task: currentTask }) });
        return;
      }
      await route.fulfill({
        json: { code: 404, message: "not found" },
        status: 404,
      });
    },
  );
  return state;
}
