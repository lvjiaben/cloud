import type { Page, Route } from "@host-tests/support/playwright";

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export const supportTickets = {
  global: "TKT_SUPPORT_GLOBAL",
  local: "TKT_SUPPORT_LOCAL",
  readonly: "TKT_SUPPORT_READONLY",
  sourceGlobal: "TKT_SUPPORT_SOURCE_GLOBAL",
  sourceLocal: "TKT_SUPPORT_SOURCE_LOCAL",
} as const;

type TicketKey = "global" | "local" | "readonly";

function ticket(
  key: TicketKey,
  overrides: Record<string, unknown> = {},
) {
  const number = supportTickets[key];
  const subjects: Record<TicketKey, string> = {
    global: "跨队列重复网络问题",
    local: "本部门重复磁盘问题",
    readonly: "只读权限核对工单",
  };
  const versions: Record<TicketKey, number> = {
    global: 7,
    local: 5,
    readonly: 2,
  };
  return {
    assigneeUserId: 1001,
    baseStage: "in_progress",
    categoryNameEnUS: "Server operations",
    categoryNameZhCN: "服务器运维",
    categoryNo: "CAT_SUPPORT_SERVER",
    createdAt: 1_786_406_400_000,
    currentRound: 2,
    customerDisplayLabel: "示例客户",
    customerNo: "CUS_SUPPORT_0001",
    customerStage: "in_progress",
    customStatusNo: key === "readonly" ? "" : "STS_WAITING_DC",
    departmentNo: key === "global" ? "DEP_SUPPORT_NOC" : "DEP_SUPPORT_CS",
    followUp: false,
    lastActivityAt: 1_786_410_000_000,
    priority: key === "global" ? "high" : "normal",
    queue: "assigned",
    ...(key === "global"
      ? {
          relatedEntitlement: {
            currentAvailable: true,
            currentStatus: "terminated",
            currentNameEnUS: "Bare metal server · renamed",
            currentNameZhCN: "裸金属服务器 · 已改名",
            currentVersion: 9,
            entitlementNo: "ENTITLEMENT_HIDDEN_FROM_UI",
            snapshotStatus: "active",
            snapshotNameEnUS: "Bare metal server · original",
            snapshotNameZhCN: "裸金属服务器 · 创建时",
            snapshotVersion: 3,
          },
        }
      : {}),
    slaConfigured: key !== "global",
    slaRisk: "none",
    subject: subjects[key],
    ticketNo: number,
    type: "customer",
    updatedAt: 1_786_410_000_000,
    version: versions[key],
    visibility: "customer",
    ...overrides,
  };
}

function detail(
  key: TicketKey,
  relations: Array<Record<string, unknown>> = [],
) {
  const actions: Record<TicketKey, string[]> = {
    global: ["merge_all", "split", "upstream", "public_reply"],
    local: ["merge", "status_transition"],
    readonly: [],
  };
  return {
    assignmentHistory:
      key === "global"
        ? [
            {
              action: "transferred",
              fromDepartmentNo: "DEP_SUPPORT_CS",
              occurredAt: 1_786_409_700_000,
              reason: "转交网络组继续处理",
              toDepartmentNo: "DEP_SUPPORT_NOC",
            },
          ]
        : [],
    attachments:
      key === "global"
        ? [{ attachmentNo: "ATT_PUBLIC_1", createdAt: 1_786_409_000_000, fileName: "network-log.txt", hidden: false, mediaType: "text/plain", sizeBytes: 128, visibility: "customer_visible" }]
        : [],
    availableActions: actions[key],
    availableStatusTransitions:
      key === "local"
        ? [
            {
              baseStage: "waiting_customer",
              nameEnUS: "Waiting for customer confirmation",
              nameZhCN: "等待客户确认",
              requiresReason: false,
              statusNo: "STS_WAITING_CUSTOMER",
              version: 2,
            },
            {
              baseStage: "closed",
              nameEnUS: "Force closed",
              nameZhCN: "强制关闭",
              requiresReason: true,
              statusNo: "STS_FORCE_CLOSED",
              version: 4,
            },
          ]
        : [],
    escalations: [],
    internalNotes:
      key === "global"
        ? [{ attachments: [], authorType: "upstream", automated: false, body: "上游已完成线路检查。", createdAt: 1_786_410_050_000, hidden: false, messageNo: "MSG_UPSTREAM_1", type: "upstream_reply", visibility: "internal_only" }]
        : [],
    customFields:
      key === "global"
        ? [
            {
              definitionVersion: 2,
              descriptionEnUS: "Frozen when the ticket was created",
              descriptionZhCN: "创建工单时已经冻结",
              fieldNo: "FIELD_OLD_SELECT",
              fieldType: "single_select",
              nameEnUS: "Original urgency",
              nameZhCN: "原始紧急程度",
              normalizedValue: "OPTION_OLD_HIGH",
              required: true,
              selectedOptions: [
                {
                  nameEnUS: "High",
                  nameZhCN: "高",
                  optionNo: "OPTION_OLD_HIGH",
                  sortOrder: 10,
                  status: "disabled",
                  version: 3,
                },
              ],
              sortOrder: 10,
              visibility: "customer_visible",
            },
          ]
        : [],
    publicMessages:
      key === "global"
        ? [{ attachments: [], authorType: "customer", automated: false, body: "服务器网络间歇中断。", createdAt: 1_786_409_000_000, hidden: false, messageNo: "MSG_PUBLIC_1", type: "public_reply", visibility: "customer_visible" }]
        : [],
    relations,
    slaRounds:
      key === "global"
        ? []
        : [
            {
              firstResponseBreached: false,
              firstResponseDueAt: 1_786_413_600_000,
              pausedSeconds: 0,
              resolutionBreached: false,
              resolutionDueAt: 1_786_435_200_000,
              round: 2,
            },
          ],
    statusHistory:
      key === "global"
        ? [
            {
              customStatusNo: "STS_WAITING_DC",
              fromStage: "new",
              occurredAt: 1_786_409_600_000,
              reason: "已开始排查",
              source: "agent",
              toStage: "in_progress",
            },
          ]
        : [],
    ticket: ticket(key),
  };
}

export interface SupportDeskMockState {
  candidateRequests: URL[];
  createRequests: Array<Record<string, unknown>>;
  failNextStatusTransition: boolean;
  failNextSettingsWrite: boolean;
  mergeRequests: Array<{
    body: Record<string, unknown>;
    path: string;
  }>;
  settingsRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    path: string;
  }>;
  statusTransitionRequests: Array<Record<string, unknown>>;
  upstreamRequests: Array<{ body: Record<string, unknown>; path: string }>;
}

export async function installSupportDeskMock(
  page: Page,
): Promise<SupportDeskMockState> {
  const state: SupportDeskMockState = {
    candidateRequests: [],
    createRequests: [],
    failNextStatusTransition: false,
    failNextSettingsWrite: false,
    mergeRequests: [],
    settingsRequests: [],
    statusTransitionRequests: [],
    upstreamRequests: [],
  };
  let localTransition: Record<string, unknown> | undefined;
  function currentDetail(
    key: TicketKey,
    currentRelations: Array<Record<string, unknown>> = [],
  ) {
    const item = detail(key, currentRelations);
    if (key !== "local" || !localTransition) return item;
    return {
      ...item,
      availableActions: ["merge"],
      availableStatusTransitions: [],
      statusHistory: [
        ...item.statusHistory,
        {
          customStatusNo: localTransition.customStatusNo,
          fromStage: "in_progress",
          occurredAt: 1_786_410_200_000,
          reason: localTransition.reason,
          source: "staff_custom_status",
          toStage: "closed",
        },
      ],
      ticket: ticket("local", {
        baseStage: "closed",
        customStatusNo: localTransition.customStatusNo,
        version: 6,
      }),
    };
  }
  let automationPolicy = { autoCloseDays: 3, reopenDays: 7, version: 4 };
  const customFieldTypes = [
    "single_line",
    "multi_line",
    "number",
    "single_select",
    "multi_select",
    "date",
    "boolean",
  ] as const;
  const customFieldSettings = customFieldTypes.map((fieldType, index) => ({
    appliesCustomer: false,
    appliesInternal: true,
    appliesPresales: false,
    descriptionEnUS: `English help ${index + 1}`,
    descriptionZhCN: `中文说明 ${index + 1}`,
    fieldNo: `FIELD_INTERNAL_${index + 1}`,
    fieldType,
    nameEnUS: `Internal field ${index + 1}`,
    nameZhCN: `内部字段 ${index + 1}`,
    options: fieldType.includes("select")
      ? [
          {
            nameEnUS: "Option A",
            nameZhCN: "选项甲",
            optionNo: `FIELD_INTERNAL_${index + 1}_OPTION_A`,
            sortOrder: 10,
            status: "active",
            version: 1,
          },
          {
            nameEnUS: "Option B",
            nameZhCN: "选项乙",
            optionNo: `FIELD_INTERNAL_${index + 1}_OPTION_B`,
            sortOrder: 20,
            status: "active",
            version: 1,
          },
        ]
      : [],
    required: index < 3,
    sortOrder: (index + 1) * 10,
    status: "active",
    version: 1,
    visibility: "internal_only",
  }));
  const settingCollections: Record<string, Array<Record<string, unknown>>> = {
    "custom-fields": customFieldSettings,
    departments: [
      {
        assignmentMode: "manual_claim",
        departmentNo: "DEP_SUPPORT_CS",
        nameEnUS: "Customer support",
        nameZhCN: "客户支持部",
        parentDepartmentNo: "",
        status: "active",
        version: 4,
      },
      {
        assignmentMode: "least_unresolved",
        departmentNo: "DEP_SUPPORT_NOC",
        nameEnUS: "Network operations",
        nameZhCN: "网络运维部",
        parentDepartmentNo: "DEP_SUPPORT_CS",
        status: "active",
        version: 2,
      },
    ],
    "issue-categories": [
      {
        categoryNo: "CAT_SUPPORT_SERVER",
        departmentNo: "DEP_SUPPORT_NOC",
        nameEnUS: "Server operations",
        nameZhCN: "服务器运维",
        priority: "normal",
        sortOrder: 10,
        status: "active",
        version: 3,
      },
    ],
    "reply-presets": [
      {
        body: "您好，我们已经开始排查。",
        departmentNo: "DEP_SUPPORT_CS",
        locale: "zh-CN",
        presetNo: "PRESET_SUPPORT_CHECKING",
        sortOrder: 10,
        status: "active",
        title: "开始排查",
        version: 2,
      },
    ],
    "sla-calendars": [
      {
        calendarNo: "CAL_SUPPORT_24X7",
        mode: "continuous",
        name: "全天连续计时",
        status: "active",
        timezone: "",
        version: 2,
        workdays: [],
      },
      {
        calendarNo: "CAL_SUPPORT_BUSINESS",
        mode: "business_hours",
        name: "中国工作时间",
        status: "active",
        timezone: "Asia/Shanghai",
        version: 1,
        workdays: [1, 2, 3, 4, 5],
        workEndMinute: 1080,
        workStartMinute: 540,
      },
    ],
    "sla-rules": [
      {
        calendarNo: "CAL_SUPPORT_24X7",
        categoryNo: "",
        departmentNo: "DEP_SUPPORT_CS",
        firstResponseMinutes: 30,
        nearDueMinutes: [15],
        priority: "normal",
        resolutionMinutes: 240,
        ruleNo: "RULE_SUPPORT_CS_NORMAL",
        status: "active",
        version: 1,
      },
    ],
    statuses: [
      {
        baseStage: "in_progress",
        nameEnUS: "Waiting for data-center confirmation",
        nameZhCN: "等待机房确认",
        sortOrder: 10,
        status: "active",
        statusNo: "STS_WAITING_DC",
        version: 3,
      },
    ],
  };
  const holidays: Array<Record<string, unknown>> = [
    {
      calendarNo: "CAL_SUPPORT_BUSINESS",
      date: "2026-10-01",
      name: "国庆节",
    },
  ];
  const settingNumberKeys: Record<string, string> = {
    "custom-fields": "fieldNo",
    departments: "departmentNo",
    "issue-categories": "categoryNo",
    "reply-presets": "presetNo",
    "sla-calendars": "calendarNo",
    "sla-rules": "ruleNo",
    statuses: "statusNo",
  };
  const settingPrefixes: Record<string, string> = {
    "custom-fields": "FIELD_SUPPORT_NEW",
    departments: "DEP_SUPPORT_NEW",
    "issue-categories": "CAT_SUPPORT_NEW",
    "reply-presets": "PRESET_SUPPORT_NEW",
    "sla-calendars": "CAL_SUPPORT_NEW",
    "sla-rules": "RULE_SUPPORT_NEW",
    statuses: "STS_SUPPORT_NEW",
  };
  const relations: Record<TicketKey, Array<Record<string, unknown>>> = {
    global: [
      {
        createdAt: 1_786_409_800_000,
        reason: "之前已合并同类问题",
        sourceTicketNo: "TKT_SUPPORT_ARCHIVED",
        targetTicketNo: supportTickets.global,
        type: "merged_into",
      },
    ],
    local: [],
    readonly: [],
  };
  const upstreamForwards = [
    { attemptCount: 1, evidenceCode: "detail_read_succeeded", forwardNo: "UPF_SENT_1", providerTicketReference: "321", previewDigest: "a".repeat(64), resultClassification: "accepted", retryEligible: false, status: "succeeded", ticketNo: supportTickets.global, updatedAt: 1_786_410_100_000, version: 3 },
    { attemptCount: 1, evidenceCode: "create_rejected", forwardNo: "UPF_FAILED_1", providerTicketReference: "", previewDigest: "b".repeat(64), resultClassification: "definitive_not_received", retryEligible: true, status: "definitive_failure", ticketNo: supportTickets.global, updatedAt: 1_786_410_000_000, version: 2 },
  ];

  await page.route(
    "**/x/idc-core/api/v1/admin/support/**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;

      if (path.includes("/admin/support/settings/")) {
        const marker = "/admin/support/settings/";
        const relative = path.slice(path.indexOf(marker) + marker.length);
        const parts = relative.split("/").filter(Boolean);
        const resource = parts[0] ?? "";
        const method = request.method();
        const body = request.postData()
          ? (request.postDataJSON() as Record<string, unknown>)
          : {};

        if (method !== "GET" && state.failNextSettingsWrite) {
          state.failNextSettingsWrite = false;
          await route.fulfill({
            status: 409,
            json: { code: 409, data: {}, message: "version conflict" },
          });
          return;
        }

        if (resource === "automation-policy") {
          if (method === "GET") {
            await route.fulfill({ json: envelope({ policy: automationPolicy }) });
            return;
          }
          if (method === "PUT") {
            state.settingsRequests.push({ body, method, path });
            automationPolicy = {
              autoCloseDays: Number(body.autoCloseDays),
              reopenDays: Number(body.reopenDays),
              version: automationPolicy.version + 1,
            };
            await route.fulfill({ json: envelope({ policy: automationPolicy }) });
            return;
          }
        }

        if (resource === "upstream-channel") {
          if (method === "GET") {
            await route.fulfill({
              json: envelope({
                item: {
                  authMode: "zjmf_api_login",
                  baseUrl: "https://support.example.test",
                  configured: true,
                  credentialMaskedTail: "***42",
                  status: "enabled",
                  updatedAt: 1_786_410_000_000,
                  version: 2,
                },
              }),
            });
            return;
          }
          if (method === "PUT") {
            state.settingsRequests.push({ body, method, path });
            await route.fulfill({
              json: envelope({
                item: {
                  authMode: body.authMode,
                  baseUrl: body.baseUrl,
                  configured: true,
                  credentialMaskedTail: "***42",
                  status: body.status,
                  updatedAt: 1_786_410_100_000,
                  version: Number(body.expectedVersion ?? 0) + 1,
                },
              }),
            });
            return;
          }
        }

        if (["upstream-departments", "upstream-entitlements"].includes(resource)) {
          if (method === "GET") {
            await route.fulfill({ json: envelope({ item: null }) });
            return;
          }
          if (method === "PUT") {
            state.settingsRequests.push({ body, method, path });
            await route.fulfill({
              json: envelope({
                item: resource === "upstream-departments"
                  ? {
                      departmentNo: parts[1],
                      status: body.status,
                      upstreamDepartmentId: body.upstreamDepartmentId,
                      upstreamDepartmentName: body.upstreamDepartmentName,
                      version: Number(body.expectedVersion ?? 0) + 1,
                    }
                  : {
                      entitlementNo: parts[1],
                      status: body.status,
                      upstreamReference: body.upstreamReference,
                      version: Number(body.expectedVersion ?? 0) + 1,
                    },
              }),
            });
            return;
          }
        }

        if (resource === "sla-calendars" && parts[2] === "holidays") {
          const calendarNo = parts[1] ?? "";
          const date = parts[3] ?? "";
          if (method === "GET") {
            await route.fulfill({
              json: envelope({
                items: holidays.filter((item) => item.calendarNo === calendarNo),
              }),
            });
            return;
          }
          state.settingsRequests.push({ body, method, path });
          if (method === "PUT") {
            const item = { calendarNo, date, name: String(body.name ?? "") };
            const index = holidays.findIndex(
              (candidate) =>
                candidate.calendarNo === calendarNo && candidate.date === date,
            );
            if (index >= 0) holidays[index] = item;
            else holidays.push(item);
            await route.fulfill({ json: envelope({ item }) });
            return;
          }
          if (method === "DELETE") {
            const index = holidays.findIndex(
              (candidate) =>
                candidate.calendarNo === calendarNo && candidate.date === date,
            );
            if (index >= 0) holidays.splice(index, 1);
            await route.fulfill({ json: envelope({}) });
            return;
          }
        }

        const collection = settingCollections[resource];
        const numberKey = settingNumberKeys[resource];
        if (collection && numberKey) {
          if (method === "GET" && parts.length === 1) {
            await route.fulfill({ json: envelope({ items: collection }) });
            return;
          }
          state.settingsRequests.push({ body, method, path });
          if (method === "POST") {
            const item = {
              ...body,
              [numberKey]: `${settingPrefixes[resource]}_${collection.length + 1}`,
              version: 1,
            };
            collection.push(item);
            await route.fulfill({ json: envelope({ item }) });
            return;
          }
          const stableNumber = decodeURIComponent(parts[1] ?? "");
          const index = collection.findIndex(
            (item) => item[numberKey] === stableNumber,
          );
          if (index >= 0 && method === "PUT") {
            const { expectedVersion: _expectedVersion, ...payload } = body;
            const item = {
              ...collection[index],
              ...payload,
              version: Number(collection[index]?.version ?? 0) + 1,
            };
            collection[index] = item;
            await route.fulfill({ json: envelope({ item }) });
            return;
          }
          if (index >= 0 && method === "DELETE") {
            const item = {
              ...collection[index],
              status: "disabled",
              version: Number(collection[index]?.version ?? 0) + 1,
            };
            collection[index] = item;
            await route.fulfill({ json: envelope({ item }) });
            return;
          }
        }
      }

      if (request.method() === "GET" && path.endsWith("/departments")) {
        await route.fulfill({
          json: envelope({
            departments: [
              {
                assignmentMode: "manual_claim",
                departmentNo: "DEP_SUPPORT_CS",
                nameEnUS: "Customer support",
                nameZhCN: "客户支持部",
                status: "active",
                version: 1,
              },
              {
                assignmentMode: "least_unresolved",
                departmentNo: "DEP_SUPPORT_NOC",
                nameEnUS: "Network operations",
                nameZhCN: "网络运维部",
                status: "active",
                version: 2,
              },
            ],
          }),
        });
        return;
      }
      if (request.method() === "GET" && path.endsWith("/issue-categories")) {
        await route.fulfill({
          json: envelope({
            issueCategories: [
              {
                categoryNo: "CAT_SUPPORT_SERVER",
                nameEnUS: "Server operations",
                nameZhCN: "服务器运维",
                priority: "normal",
              },
            ],
          }),
        });
        return;
      }
      if (request.method() === "GET" && path.endsWith("/statuses")) {
        await route.fulfill({
          json: envelope({
            statuses: [
              {
                baseStage: "in_progress",
                nameEnUS: "Waiting for data-center confirmation",
                nameZhCN: "等待机房确认",
                status: "enabled",
                statusNo: "STS_WAITING_DC",
                version: 3,
              },
              {
                baseStage: "waiting_customer",
                nameEnUS: "Waiting for customer confirmation",
                nameZhCN: "等待客户确认",
                status: "enabled",
                statusNo: "STS_WAITING_CUSTOMER",
                version: 2,
              },
              {
                baseStage: "closed",
                nameEnUS: "Force closed",
                nameZhCN: "强制关闭",
                status: "enabled",
                statusNo: "STS_FORCE_CLOSED",
                version: 4,
              },
            ],
          }),
        });
        return;
      }
      if (
        request.method() === "GET" &&
        path.endsWith("/custom-fields/internal")
      ) {
        await route.fulfill({
          json: envelope({ items: customFieldSettings }),
        });
        return;
      }
      if (request.method() === "POST" && path.endsWith("/tickets")) {
        state.createRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({ json: envelope({ item: currentDetail("local") }) });
        return;
      }
      if (request.method() === "GET" && path.endsWith("/tickets")) {
        await route.fulfill({
          json: envelope({
            nextCursor: "",
            pageActions: ["create", "query"],
            tickets: [ticket("global"), ticket("local"), ticket("readonly")],
          }),
        });
        return;
      }

      for (const key of ["global", "local", "readonly"] as const) {
        if (
          request.method() === "GET" &&
          path.endsWith(`/tickets/${supportTickets[key]}`)
        ) {
          await route.fulfill({ json: envelope({ item: currentDetail(key, relations[key]) }) });
          return;
        }
      }

      if (request.method() === "GET" && path.endsWith(`/tickets/${supportTickets.global}/upstream-forwards`)) {
        await route.fulfill({ json: envelope({ items: upstreamForwards }) });
        return;
      }
      if (request.method() === "POST" && path.endsWith(`/tickets/${supportTickets.global}/upstream-previews`)) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.upstreamRequests.push({ body, path });
        await route.fulfill({ json: envelope({ item: { attachmentNos: body.attachmentNos, content: "服务器网络间歇中断。\n\nAttachments: network-log.txt", departmentId: 9, entitlementUpstreamRef: "opaque-88", forwardNo: "UPF_PREVIEW_1", messageNos: body.messageNos, previewDigest: "c".repeat(64), selectionDigest: "d".repeat(64), ticketNo: supportTickets.global, title: `[${supportTickets.global}] 跨队列重复网络问题`, version: 1 } }) });
        return;
      }
      if (request.method() === "POST" && (path.endsWith("/upstream-forwards/UPF_PREVIEW_1/confirmations") || path.endsWith("/upstream-forwards/UPF_FAILED_1/retries") || path.endsWith("/upstream-forwards/UPF_SENT_1/reply-syncs") || path.endsWith(`/tickets/${supportTickets.global}/upstream-replies/MSG_UPSTREAM_1/publications`))) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.upstreamRequests.push({ body, path });
        if (path.endsWith("/reply-syncs")) {
          await route.fulfill({ json: envelope({ added: 1 }) });
        } else if (path.endsWith("/publications")) {
          await route.fulfill({ json: envelope({ item: currentDetail("global", relations.global) }) });
        } else {
          await route.fulfill({ json: envelope({ item: upstreamForwards[0] }) });
        }
        return;
      }

      if (
        request.method() === "POST" &&
        path.endsWith(`/tickets/${supportTickets.local}/status-transitions`)
      ) {
        const transition = request.postDataJSON() as Record<string, unknown>;
        state.statusTransitionRequests.push(transition);
        if (state.failNextStatusTransition) {
          state.failNextStatusTransition = false;
          await route.fulfill({
            json: { code: 409, data: {}, message: "ticket version conflict" },
            status: 409,
          });
          return;
        }
        localTransition = transition;
        await route.fulfill({
          json: envelope({ item: currentDetail("local", relations.local) }),
        });
        return;
      }

      if (
        request.method() === "GET" &&
        path.endsWith(`/tickets/${supportTickets.global}/global-merge-candidates`)
      ) {
        state.candidateRequests.push(url);
        await route.fulfill({
          json: envelope({
            items: [
              {
                stage: "in_progress",
                subject: "另一队列的重复网络问题",
                ticketNo: supportTickets.sourceGlobal,
                version: 3,
              },
            ],
            nextCursor: "",
          }),
        });
        return;
      }
      if (
        request.method() === "GET" &&
        path.endsWith(`/tickets/${supportTickets.local}/merge-candidates`)
      ) {
        state.candidateRequests.push(url);
        await route.fulfill({
          json: envelope({
            items: [
              {
                stage: "new",
                subject: "本部门的重复磁盘问题",
                ticketNo: supportTickets.sourceLocal,
                version: 4,
              },
            ],
            nextCursor: "",
          }),
        });
        return;
      }
      if (
        request.method() === "POST" &&
        (path.endsWith(`/tickets/${supportTickets.global}/global-merges`) ||
          path.endsWith(`/tickets/${supportTickets.local}/merges`))
      ) {
        const body = request.postDataJSON() as Record<string, unknown>;
        state.mergeRequests.push({ body, path });
        const key: TicketKey = path.endsWith("/global-merges") ? "global" : "local";
        const sources = body.sources as Array<{ ticketNo?: string }>;
        relations[key].push({
          createdAt: 1_786_410_100_000,
          reason: body.reason,
          sourceTicketNo: sources[0]?.ticketNo ?? "",
          targetTicketNo: supportTickets[key],
          type: "merged_into",
        });
        await route.fulfill({ json: envelope({ item: currentDetail(key, relations[key]) }) });
        return;
      }

      await route.fulfill({ status: 404, json: envelope({}) });
    },
  );
  return state;
}
