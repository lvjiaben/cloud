import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const catalogs = {
  "en-US": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/en-US/plugin.json", import.meta.url),
      "utf8",
    ),
  ),
  "zh-CN": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
      "utf8",
    ),
  ),
} as Record<string, Record<string, string>>;

function envelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const parts = key.split(".");
    let cursor = root;
    for (const part of parts.slice(0, -1)) {
      if (!cursor[part] || typeof cursor[part] !== "object") cursor[part] = {};
      cursor = cursor[part] as Record<string, unknown>;
    }
    cursor[parts.at(-1)!] = value;
  }
  return root;
}

const observedAt = 1_786_248_000_000;
const taskNo = "task_0123456789abcdef";
const evidenceOne = "evidence_11111111111111111111111111111111";
const evidenceTwo = "evidence_22222222222222222222222222222222";

export const baseTask = {
  attemptCount: 1,
  capabilityCode: "server.start",
  channelName: "魔方云生产渠道",
  channelNo: "channel_mofang_prod",
  createdAt: observedAt - 60_000,
  localResourceNo: "resource_0123456789abcdef",
  maxAttempts: 8,
  nextObserveAt: observedAt + 30_000,
  observationAttemptCount: 4,
  resultUnknown: true,
  status: "result_unknown",
  takeover: null,
  taskNo,
  updatedAt: observedAt,
  version: 3,
};

const evidence = [
  {
    evidenceKind: "task_observation",
    evidenceNo: evidenceOne,
    observedAt,
    reference: "900100",
    sourceKind: "provider_task",
    summary: "task.running",
  },
  {
    evidenceKind: "resource_observation",
    evidenceNo: evidenceTwo,
    observedAt: observedAt - 10_000,
    reference: "600100",
    sourceKind: "provider_resource",
    summary: "resource.power_state_on",
  },
];

export const providerTaskFixture = { evidenceOne, evidenceTwo, taskNo };

export interface ProviderTaskMockState {
  detailRequests: URL[];
  listRequests: URL[];
  takeoverRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    url: URL;
  }>;
}

export async function installProviderTaskMock(
  page: Page,
  options: { canTakeOver?: boolean; withEvidence?: boolean } = {},
): Promise<ProviderTaskMockState> {
  const state: ProviderTaskMockState = {
    detailRequests: [],
    listRequests: [],
    takeoverRequests: [],
  };
  let currentTask: Record<string, unknown> = structuredClone(baseTask);
  const currentEvidence = options.withEvidence === false ? [] : evidence;
  let currentLocale: keyof typeof catalogs = "zh-CN";

  const detail = () => ({
    evidence: currentEvidence,
    task: currentTask,
    upstream: {
      externalResourceRef: "600100",
      externalTaskRef: "900100",
      observedAt,
      progress: 40,
      statusCode: "running",
    },
  });

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (payload.data) {
      payload.data.permissions = [
        "idc:admin:resource:task:list",
        "idc:admin:resource:task:query",
      ];
      if (options.canTakeOver !== false) {
        payload.data.permissions.push(
          "idc:admin:resource:task:manual-takeover",
        );
      }
    }
    await route.fulfill({ json: payload, response });
  });
  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    if (route.request().headers()["accept-language"]?.includes("en")) {
      currentLocale = "en-US";
    }
    await route.fulfill({
      json: envelope({
        enabled: true,
        items: [
          {
            direction: "ltr",
            isDefault: true,
            locale: "zh-CN",
            name: "Simplified Chinese",
            nativeName: "简体中文",
          },
          {
            direction: "ltr",
            isDefault: false,
            locale: "en-US",
            name: "English",
            nativeName: "English",
          },
        ],
        locale: currentLocale,
      }),
    });
  });
  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const locale =
        new URL(route.request().url()).searchParams.get("lang") === "en-US"
          ? "en-US"
          : "zh-CN";
      currentLocale = locale;
      await route.fulfill({
        json: envelope({ messages: nestedMessages(catalogs[locale]!) }),
      });
    },
  );
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({
      json: envelope({
        list: [
          {
            children: [
              {
                component: "system/plugin/dynamic-page",
                meta: {
                  icon: "lucide:list-checks",
                  title: "plugin.idc-resource.tasks.title",
                },
                name: "IDCResourceProviderTasks",
                path: "/idc/resource/tasks",
              },
            ],
            component: "BasicLayout",
            meta: { title: "IDC Operations" },
            name: "IDCOperations",
            path: "/idc",
          },
        ],
      }),
    });
  });
  await page.route(
    "**/x/idc-resource/api/v1/admin/resource/tasks**",
    async (route: Route) => {
      state.listRequests.push(new URL(route.request().url()));
      await route.fulfill({
        json: envelope({ items: [currentTask], nextCursor: "", total: 1 }),
      });
    },
  );
  await page.route(
    `**/x/idc-resource/api/v1/admin/resource/tasks/${taskNo}`,
    async (route: Route) => {
      state.detailRequests.push(new URL(route.request().url()));
      await route.fulfill({ json: envelope({ item: detail() }) });
    },
  );
  await page.route(
    `**/x/idc-resource/api/v1/admin/resource/tasks/${taskNo}/manual-takeover`,
    async (route: Route) => {
      const request = route.request();
      const body = request.postDataJSON() as Record<string, unknown>;
      state.takeoverRequests.push({
        body,
        method: request.method(),
        url: new URL(request.url()),
      });
      currentTask = {
        ...currentTask,
        nextObserveAt: null,
        resultUnknown: false,
        status: "manual_takeover",
        takeover: {
          assignedUserId: 1001,
          createdAt: observedAt + 1_000,
          evidenceCount: (body.evidenceNos as string[]).length,
          fromStatus: "result_unknown",
          reasonCode: body.reasonCode,
          takeoverNo: "takeover_0123456789abcdef0123456789abcdef",
          taskVersion: body.expectedVersion,
        },
        updatedAt: observedAt + 1_000,
        version: 4,
      };
      await route.fulfill({ json: envelope({ item: detail() }) });
    },
  );
  return state;
}
