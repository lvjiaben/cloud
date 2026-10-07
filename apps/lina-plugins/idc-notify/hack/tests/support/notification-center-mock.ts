import { readFileSync } from "node:fs";

import type { Page, Route } from "@host-tests/support/playwright";

const notificationPrefix = "/x/idc-notify/api/v1/";
const adminPrefix = `${notificationPrefix}admin/notifications`;
const staffPrefix = `${notificationPrefix}staff/notifications`;
const fixedNow = 1_786_406_400_000;

const localeCatalogs = {
  "en-US": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/en-US/plugin.json", import.meta.url),
      "utf8",
    ),
  ) as Record<string, string>,
  "zh-CN": JSON.parse(
    readFileSync(
      new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url),
      "utf8",
    ),
  ) as Record<string, string>,
};

type JsonRecord = Record<string, unknown>;

interface MockChannel {
  balanceState: string;
  code: string;
  configuration: JsonRecord;
  configurationGeneration: number;
  configured: boolean;
  credentialMaskedTail: string;
  displayName: string;
  kind: string;
  pendingQueryCount: number;
  providerCode: string;
  status: string;
  updatedAt: number;
  version: number;
}

interface MockDelivery {
  businessObjectNo: string;
  businessObjectType: string;
  chainNo: string;
  channelCode: string;
  completedAt?: number;
  eventCode: string;
  externalBoundaryCrossed: boolean;
  isChainTail: boolean;
  locale: string;
  manualResendCount: number;
  nextAttemptAt?: number;
  previousTaskNo: string;
  queryDeadline?: number;
  queryDeadlineSource: string;
  queuedAt: number;
  recipientMask: string;
  resultClass: string;
  retrySequence: number;
  status: string;
  taskNo: string;
  topic: string;
  version: number;
}

export interface NotificationCenterMockState {
  actionRequests: Array<{ body: JsonRecord; method: string; path: string }>;
  allNotificationRequests: string[];
  configurationRequests: Array<{ body: JsonRecord; path: string }>;
  deliveryListRequests: URL[];
  eligibilityRequests: string[];
  eventPolicyRequests: Array<{ body: JsonRecord; path: string }>;
  templateRequests: JsonRecord[];
  testRequests: Array<{ body: JsonRecord; path: string }>;
  closureHistory: Array<{ actor: number; operationNo: string; reason: string; taskNo: string }>;
  getDeliveryCount: () => number;
  weComBindingRequests: Array<{ body: JsonRecord; path: string }>;
  unexpectedRequests: string[];
}

export interface NotificationCenterMockOptions {
  canManage?: boolean;
  eligibilityDelayCalls?: number[];
  eligibilityDeniedCalls?: number[];
  eligibilityFailureCalls?: number[];
  eligibilityReasons?: string[];
  includeQueuedEmail?: boolean;
  pendingEmailQueries?: number;
  recentLogin?: boolean;
  resendMode?: "allowed" | "cooldown" | "limit";
  weComBound?: boolean;
}

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function nestedMessages(catalog: Record<string, string>) {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const segments = key.split(".");
    let current = result;
    for (const segment of segments.slice(0, -1)) {
      const child = current[segment];
      if (!child || typeof child !== "object" || Array.isArray(child)) {
        current[segment] = {};
      }
      current = current[segment] as Record<string, unknown>;
    }
    const leaf = segments.at(-1);
    if (leaf) current[leaf] = value;
  }
  return result;
}

function baseChannels(): MockChannel[] {
  return [
    {
      balanceState: "sufficient",
      code: "in_app",
      configuration: {},
      configurationGeneration: 1,
      configured: true,
      credentialMaskedTail: "",
      displayName: "In-app",
      kind: "in_app",
      pendingQueryCount: 0,
      providerCode: "builtin",
      status: "enabled",
      updatedAt: fixedNow,
      version: 1,
    },
    {
      balanceState: "sufficient",
      code: "email",
      configuration: {
        smtp: {
          fromAddress: "notice@example.com",
          fromName: "Lina IDC",
          host: "smtp.notify.example",
          port: 465,
          security: "implicit_tls",
          username: "mailer@example.com",
        },
      },
      configurationGeneration: 3,
      configured: true,
      credentialMaskedTail: "7a9c",
      displayName: "SMTP",
      kind: "smtp",
      pendingQueryCount: 0,
      providerCode: "smtp",
      status: "enabled",
      updatedAt: fixedNow,
      version: 3,
    },
    {
      balanceState: "unknown",
      code: "sms",
      configuration: {
        tencentSms: {
          region: "ap-guangzhou",
          signName: "LinaIDC",
          smsSdkAppId: "1400999999",
        },
      },
      configurationGeneration: 2,
      configured: true,
      credentialMaskedTail: "31ef",
      displayName: "Tencent Cloud SMS",
      kind: "sms",
      pendingQueryCount: 0,
      providerCode: "tencent_cloud_sms_v20210111",
      status: "enabled",
      updatedAt: fixedNow,
      version: 2,
    },
    {
      balanceState: "sufficient",
      code: "wechat_official",
      configuration: {
        weChatOfficial: {
          appId: "wx-official-public-id",
          callbackVerified: true,
          callbackVerifiedAt: fixedNow,
        },
      },
      configurationGeneration: 4,
      configured: true,
      credentialMaskedTail: "c84d",
      displayName: "WeChat Official",
      kind: "wechat_official",
      pendingQueryCount: 0,
      providerCode: "wechat_official",
      status: "enabled",
      updatedAt: fixedNow,
      version: 4,
    },
    {
      balanceState: "sufficient",
      code: "wechat_work",
      configuration: {
        weCom: { agentId: 1_000_001, corpId: "ww-corporation-public-id" },
      },
      configurationGeneration: 2,
      configured: true,
      credentialMaskedTail: "82ba",
      displayName: "WeCom",
      kind: "wechat_work",
      pendingQueryCount: 0,
      providerCode: "wechat_work",
      status: "enabled",
      updatedAt: fixedNow,
      version: 2,
    },
    {
      balanceState: "sufficient",
      code: "signed_webhook",
      configuration: {},
      configurationGeneration: 0,
      configured: false,
      credentialMaskedTail: "",
      displayName: "Signed webhook",
      kind: "signed_webhook",
      pendingQueryCount: 0,
      providerCode: "signed_webhook",
      status: "unconfigured",
      updatedAt: fixedNow,
      version: 1,
    },
    {
      balanceState: "sufficient",
      code: "dingtalk",
      configuration: {},
      configurationGeneration: 0,
      configured: false,
      credentialMaskedTail: "",
      displayName: "DingTalk",
      kind: "dingtalk",
      pendingQueryCount: 0,
      providerCode: "dingtalk_spi",
      status: "unconfigured",
      updatedAt: fixedNow,
      version: 1,
    },
    {
      balanceState: "sufficient",
      code: "feishu",
      configuration: {},
      configurationGeneration: 0,
      configured: false,
      credentialMaskedTail: "",
      displayName: "Feishu",
      kind: "feishu",
      pendingQueryCount: 0,
      providerCode: "feishu_spi",
      status: "unconfigured",
      updatedAt: fixedNow,
      version: 1,
    },
  ];
}

const events = [
  {
    code: "account.password.reset",
    objectType: "customer_account",
    optionalVariables: ["display_name"],
    requiredVariables: ["reset_url", "expires_minutes"],
    securityRequired: true,
    topic: "security",
    version: 1,
  },
  {
    code: "support.ticket.updated",
    objectType: "support_ticket",
    requiredVariables: ["ticket_no", "title"],
    optionalVariables: ["status", "message"],
    securityRequired: false,
    topic: "support",
    version: 1,
  },
  {
    code: "service.expiry.reminder",
    objectType: "service",
    optionalVariables: ["service_name"],
    requiredVariables: ["service_no", "expires_at"],
    securityRequired: false,
    topic: "service",
    version: 1,
  },
  {
    code: "billing.payment.result",
    objectType: "payment",
    optionalVariables: [],
    requiredVariables: ["invoice_no", "amount", "currency"],
    securityRequired: false,
    topic: "billing",
    version: 1,
  },
  {
    code: "marketing.campaign",
    objectType: "campaign",
    optionalVariables: ["unsubscribe_url"],
    requiredVariables: ["campaign_name"],
    securityRequired: false,
    topic: "marketing",
    version: 1,
  },
];

const initialEventPolicies = [
  {
    channelCode: "email",
    enabled: true,
    eventCode: "support.ticket.updated",
    recipientKind: "customer_contact",
    topic: "support",
    updatedAt: fixedNow,
    version: 1,
  },
  {
    channelCode: "signed_webhook",
    enabled: false,
    eventCode: "support.ticket.updated",
    recipientKind: "customer_contact",
    topic: "support",
    updatedAt: fixedNow,
    version: 1,
  },
];

const initialTemplates = [
  {
    body: "工单 {{ticket_no}} 已更新：{{title}}。",
    channelCode: "wechat_work",
    createdAt: fixedNow,
    createdBy: 1,
    eventCode: "support.ticket.updated",
    locale: "zh-CN",
    status: "active",
    subject: "工单更新",
    templateNo: "NTF_WECOM_TICKET_ZH",
    version: 1,
  },
  {
    body: "工单 {{ticket_no}} 已更新为 {{status}}。",
    channelCode: "email",
    createdAt: fixedNow,
    createdBy: 1,
    eventCode: "support.ticket.updated",
    locale: "zh-CN",
    status: "active",
    subject: "工单更新",
    templateNo: "NTF_EMAIL_TICKET_ZH",
    version: 2,
  },
  {
    body: "Your ticket {{ticket_no}} is now {{status}}.",
    channelCode: "email",
    createdAt: fixedNow,
    createdBy: 1,
    eventCode: "support.ticket.updated",
    locale: "en-US",
    status: "active",
    subject: "Ticket update",
    templateNo: "NTF_EMAIL_TICKET_EN",
    version: 1,
  },
  {
    body: "服务 {{service_no}} 将于 {{expires_at}} 到期。",
    channelCode: "sms",
    createdAt: fixedNow,
    createdBy: 1,
    eventCode: "service.expiry.reminder",
    locale: "zh-CN",
    status: "active",
    subject: "",
    templateNo: "NTF_SMS_EXPIRY_ZH",
    sms: {
      dataMapping: {},
      providerCode: "tencent_cloud_sms_v20210111",
      templateId: "2299001",
      variableOrder: ["service_no", "expires_at"],
    },
    version: 3,
  },
  {
    body: "账单 {{invoice_no}} 支付结果：{{amount}} {{currency}}。",
    channelCode: "wechat_official",
    createdAt: fixedNow,
    createdBy: 1,
    eventCode: "billing.payment.result",
    locale: "zh-CN",
    status: "active",
    subject: "支付结果",
    templateNo: "NTF_WECHAT_PAYMENT_ZH",
    version: 4,
    weChatOfficial: {
      dataMapping: {
        amount1: "amount",
        character_string1: "invoice_no",
        thing2: "currency",
      },
      templateId: "wechat-template-public-id",
    },
  },
];

function initialDeliveries(includeQueuedEmail = false): MockDelivery[] {
  const items: MockDelivery[] = [
    {
      businessObjectNo: "TICKET-20260815-001",
      businessObjectType: "support_ticket",
      chainNo: "CHAIN-FAILED-001",
      channelCode: "email",
      completedAt: fixedNow - 60_000,
      eventCode: "support.ticket.updated",
      externalBoundaryCrossed: false,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      previousTaskNo: "",
      queryDeadlineSource: "",
      queuedAt: fixedNow - 120_000,
      recipientMask: "a***@example.com",
      resultClass: "definitive_not_received_temporary",
      retrySequence: 3,
      status: "failed",
      taskNo: "TASK-FAILED-001",
      topic: "support",
      version: 5,
    },
    {
      businessObjectNo: "PAYMENT-20260815-001",
      businessObjectType: "payment",
      chainNo: "CHAIN-UNKNOWN-001",
      channelCode: "wechat_official",
      eventCode: "billing.payment.result",
      externalBoundaryCrossed: true,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      previousTaskNo: "",
      queryDeadline: fixedNow + 600_000,
      queryDeadlineSource: "provider_callback",
      queuedAt: fixedNow - 180_000,
      recipientMask: "oP***8z",
      resultClass: "result_unknown",
      retrySequence: 0,
      status: "manual_review",
      taskNo: "TASK-UNKNOWN-001",
      topic: "billing",
      version: 2,
    },
    {
      businessObjectNo: "SERVICE-001",
      businessObjectType: "service",
      chainNo: "CHAIN-RETRY-001",
      channelCode: "sms",
      eventCode: "service.expiry.reminder",
      externalBoundaryCrossed: false,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      nextAttemptAt: fixedNow + 300_000,
      previousTaskNo: "",
      queryDeadlineSource: "",
      queuedAt: fixedNow - 240_000,
      recipientMask: "+86 138****8000",
      resultClass: "definitive_not_received_temporary",
      retrySequence: 2,
      status: "retry_wait",
      taskNo: "TASK-RETRY-001",
      topic: "service",
      version: 3,
    },
    {
      businessObjectNo: "ACCOUNT-001",
      businessObjectType: "customer_account",
      chainNo: "CHAIN-RETRY-ONE-001",
      channelCode: "email",
      eventCode: "account.password.reset",
      externalBoundaryCrossed: false,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      nextAttemptAt: fixedNow + 60_000,
      previousTaskNo: "",
      queryDeadlineSource: "",
      queuedAt: fixedNow - 200_000,
      recipientMask: "a***@example.com",
      resultClass: "definitive_not_received_temporary",
      retrySequence: 1,
      status: "retry_wait",
      taskNo: "TASK-RETRY-ONE-001",
      topic: "security",
      version: 2,
    },
    {
      businessObjectNo: "TICKET-RETRY-THREE-001",
      businessObjectType: "support_ticket",
      chainNo: "CHAIN-RETRY-THREE-001",
      channelCode: "email",
      eventCode: "support.ticket.created",
      externalBoundaryCrossed: false,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      nextAttemptAt: fixedNow + 1_800_000,
      previousTaskNo: "",
      queryDeadlineSource: "",
      queuedAt: fixedNow - 220_000,
      recipientMask: "a***@example.com",
      resultClass: "definitive_not_received_temporary",
      retrySequence: 3,
      status: "retry_wait",
      taskNo: "TASK-RETRY-THREE-001",
      topic: "support",
      version: 4,
    },
  ];
  if (includeQueuedEmail) {
    items.unshift({
      businessObjectNo: "TICKET-QUEUED-001",
      businessObjectType: "support_ticket",
      chainNo: "CHAIN-QUEUED-001",
      channelCode: "email",
      eventCode: "support.ticket.created",
      externalBoundaryCrossed: false,
      isChainTail: true,
      locale: "zh-CN",
      manualResendCount: 0,
      previousTaskNo: "",
      queryDeadlineSource: "",
      queuedAt: fixedNow - 10_000,
      recipientMask: "q***@example.com",
      resultClass: "none",
      retrySequence: 0,
      status: "queued",
      taskNo: "TASK-QUEUED-001",
      topic: "support",
      version: 1,
    });
  }
  return items;
}

function notificationMenus(canManage: boolean) {
  const embeddedMeta = (title: string, routePath: string, icon: string) => ({
    icon,
    query: {
      embeddedSrc: "/x-assets/idc-notify/v0.1.4/mount.js",
      pluginAccessMode: "embedded-mount",
      routePath,
    },
    title,
  });
  const children = [
    {
      component: "system/plugin/dynamic-page",
      meta: embeddedMeta(
        "My WeCom Binding",
        "/idc/notifications/wechat-work-binding",
        "lucide:scan-qr-code",
      ),
      name: "IDCNotificationWeComSelfBinding",
      path: "/idc/notifications/wechat-work-binding",
    },
  ];
  if (canManage) children.push(
    {
      component: "system/plugin/dynamic-page",
      meta: embeddedMeta(
        "Notification Channels",
        "/idc/notifications/channels",
        "lucide:send",
      ),
      name: "IDCNotificationChannels",
      path: "/idc/notifications/channels",
    },
    {
      component: "system/plugin/dynamic-page",
      meta: embeddedMeta(
        "Notification Templates",
        "/idc/notifications/templates",
        "lucide:layout-template",
      ),
      name: "IDCNotificationTemplates",
      path: "/idc/notifications/templates",
    },
    {
      component: "system/plugin/dynamic-page",
      meta: embeddedMeta(
        "Notification Deliveries",
        "/idc/notifications/deliveries",
        "lucide:list-checks",
      ),
      name: "IDCNotificationDeliveries",
      path: "/idc/notifications/deliveries",
    },
  );
  return [
    {
      children,
      component: "BasicLayout",
      meta: { icon: "lucide:server-cog", order: 30, title: "IDC Operations" },
      name: "IDCOperations",
      path: "/idc",
    },
  ];
}

async function reject(route: Route, status: number, message: string) {
  await route.fulfill({
    json: { code: status, data: null, message },
    status,
  });
}

export async function installNotificationCenterMock(
  page: Page,
  options: NotificationCenterMockOptions = {},
): Promise<NotificationCenterMockState> {
  const canManage = options.canManage !== false;
  const recentLogin = options.recentLogin !== false;
  const weComBound = options.weComBound === true;
  const channels = baseChannels();
  const email = channels.find((item) => item.code === "email");
  if (email && (options.pendingEmailQueries ?? 0) > 0) {
    email.pendingQueryCount = options.pendingEmailQueries ?? 0;
    email.status = "disabled";
  }
  const eventPolicies = structuredClone(initialEventPolicies);
  const templates = structuredClone(initialTemplates);
  const deliveries = initialDeliveries(options.includeQueuedEmail === true);
  const resendReplays = new Map<string, MockDelivery>();
  const state: NotificationCenterMockState = {
    actionRequests: [],
    allNotificationRequests: [],
    closureHistory: [],
    configurationRequests: [],
    deliveryListRequests: [],
    eligibilityRequests: [],
    eventPolicyRequests: [],
    getDeliveryCount: () => deliveries.length,
    templateRequests: [],
    testRequests: [],
    weComBindingRequests: [],
    unexpectedRequests: [],
  };

  function channel(code: string) {
    return channels.find((item) => item.code === code);
  }

  function track(route: Route) {
    const request = route.request();
    const url = new URL(request.url());
    state.allNotificationRequests.push(url.pathname);
    return { request, url };
  }

  async function authorize(route: Route, requiresRecentLogin = false) {
    if (!canManage) {
      await reject(route, 403, "notification permission denied");
      return false;
    }
    if (requiresRecentLogin && !recentLogin) {
      await reject(route, 403, "recent full login required");
      return false;
    }
    return true;
  }

  // Register the deny-by-default route first. Playwright evaluates routes in
  // reverse registration order, so every explicitly supported endpoint below
  // wins and all accidental contracts remain visible failures.
  await page.route(`**${notificationPrefix}**`, async (route: Route) => {
    const request = route.request();
    const url = new URL(request.url());
    state.allNotificationRequests.push(url.pathname);
    state.unexpectedRequests.push(`${request.method()} ${url.pathname}${url.search}`);
    await reject(route, 501, "undefined notification E2E API");
  });

  await page.route(
    (url) => url.pathname.startsWith("/api/v1/admin/notifications"),
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      state.unexpectedRequests.push(
        `${request.method()} escaped plugin prefix: ${url.pathname}${url.search}`,
      );
      await reject(route, 501, "notification API must use the plugin prefix");
    },
  );

  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as { data?: { permissions?: string[] } };
    if (payload.data) {
      payload.data.permissions = canManage
        ? ["idc:admin:notification:channel:manage"]
        : [];
    }
    await route.fulfill({ json: payload, response });
  });

  await page.route("**/api/v1/menus/all", async (route: Route) => {
    await route.fulfill({ json: apiEnvelope({ list: notificationMenus(canManage) }) });
  });

  await page.route("**/api/v1/i18n/runtime/locales**", async (route: Route) => {
    const locale = route.request().headers()["accept-language"]?.includes("en")
      ? "en-US"
      : "zh-CN";
    await route.fulfill({
      headers: { ETag: `"notification-e2e-${locale}"` },
      json: apiEnvelope({
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
        locale,
      }),
    });
  });

  await page.route("**/api/v1/i18n/runtime/messages**", async (route: Route) => {
    const requested = new URL(route.request().url()).searchParams.get("lang");
    const locale = requested === "en-US" ? "en-US" : "zh-CN";
    await route.fulfill({
      headers: {
        "Cache-Control": "private, must-revalidate",
        ETag: `"notification-e2e-${locale}"`,
      },
      json: apiEnvelope({ messages: nestedMessages(localeCatalogs[locale]) }),
    });
  });

  await page.route(`**${adminPrefix}/channels`, async (route: Route) => {
    track(route);
    if (!(await authorize(route))) return;
    if (route.request().method() !== "GET") {
      await reject(route, 405, "method not allowed");
      return;
    }
    await route.fulfill({ json: apiEnvelope({ items: channels }) });
  });

  let weComBinding = {
    bound: weComBound,
    boundAt: weComBound ? fixedNow : 0,
    reused: false,
    status: weComBound ? "active" : "unbound",
    version: weComBound ? 1 : 0,
  };

  await page.route(`**${staffPrefix}/wechat-work/binding`, async (route: Route) => {
    const { request } = track(route);
    if (request.method() !== "GET") {
      await reject(route, 405, "method not allowed");
      return;
    }
    await route.fulfill({ json: apiEnvelope({ item: weComBinding }) });
  });

  await page.route(`**${staffPrefix}/wechat-work/oauth-authorizations`, async (route: Route) => {
    const { request, url } = track(route);
    if (!recentLogin) {
      await reject(route, 403, "recent full login required");
      return;
    }
    const body = request.postDataJSON() as JsonRecord;
    state.weComBindingRequests.push({ body, path: url.pathname });
    await route.fulfill({
      json: apiEnvelope({
        authorizationUrl: "https://open.weixin.qq.com/connect/oauth2/authorize?appid=ww-e2e&state=opaque-state",
        expiresAt: fixedNow + 300_000,
        qrLoginUrl: "https://login.work.weixin.qq.com/wwlogin/sso/login?appid=ww-e2e&state=opaque-state",
        reused: false,
      }),
    });
  });

  await page.route(`**${staffPrefix}/wechat-work/binding-removals`, async (route: Route) => {
    const { request, url } = track(route);
    if (!recentLogin) {
      await reject(route, 403, "recent full login required");
      return;
    }
    const body = request.postDataJSON() as JsonRecord;
    state.weComBindingRequests.push({ body, path: url.pathname });
    weComBinding = { bound: false, boundAt: 0, reused: false, status: "unbound", version: Number(body.expectedVersion ?? 0) + 1 };
    await route.fulfill({ json: apiEnvelope({ item: weComBinding }) });
  });

  await page.route(`**${adminPrefix}/events`, async (route: Route) => {
    track(route);
    if (!(await authorize(route))) return;
    await route.fulfill({ json: apiEnvelope({ items: events }) });
  });

  await page.route(`**${adminPrefix}/event-policies?*`, async (route: Route) => {
    const { request } = track(route);
    if (!(await authorize(route))) return;
    if (request.method() !== "GET") {
      await reject(route, 405, "method not allowed");
      return;
    }
    await route.fulfill({ json: apiEnvelope({ items: eventPolicies }) });
  });

  await page.route(`**${adminPrefix}/event-policies/**`, async (route: Route) => {
    const { request, url } = track(route);
    if (!(await authorize(route))) return;
    if (request.method() !== "PUT") {
      await reject(route, 405, "method not allowed");
      return;
    }
    const body = request.postDataJSON() as JsonRecord;
    const segments = url.pathname.split("/");
    const channelCode = segments.at(-1) ?? "";
    const recipientKind = segments.at(-2) ?? "";
    const eventCode = decodeURIComponent(segments.at(-3) ?? "");
    const current = eventPolicies.find((item) =>
      item.eventCode === eventCode &&
      item.recipientKind === recipientKind &&
      item.channelCode === channelCode,
    );
    if (!current || Number(body.expectedVersion) !== current.version) {
      await reject(route, 409, "event policy version conflict");
      return;
    }
    state.eventPolicyRequests.push({ body, path: url.pathname });
    current.enabled = body.enabled === true;
    current.version += 1;
    current.updatedAt = fixedNow;
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  await page.route(`**${adminPrefix}/templates?*`, async (route: Route) => {
    track(route);
    if (!(await authorize(route))) return;
    await route.fulfill({ json: apiEnvelope({ items: templates }) });
  });

  await page.route(`**${adminPrefix}/templates`, async (route: Route) => {
    const { request } = track(route);
    if (!(await authorize(route, request.method() !== "GET"))) return;
    if (request.method() === "GET") {
      await route.fulfill({ json: apiEnvelope({ items: templates }) });
      return;
    }
    const body = request.postDataJSON() as JsonRecord;
    state.templateRequests.push(body);
    const currentVersion = Number(body.expectedVersion ?? 0);
    const created = {
      ...body,
      createdAt: fixedNow,
      createdBy: 1,
      status: String(body.status ?? "draft"),
      templateNo: String(body.templateNo || `NTF-E2E-${templates.length + 1}`),
      version: currentVersion + 1,
    };
    templates.unshift(created as (typeof templates)[number]);
    await route.fulfill({ json: apiEnvelope({ item: created }) });
  });

  await page.route(`**${adminPrefix}/templates/*/preview`, async (route: Route) => {
    track(route);
    if (!(await authorize(route))) return;
    await route.fulfill({
      json: apiEnvelope({
        body: "服务 SERVICE-001 将于 2026-09-01 到期。",
        subject: "服务到期提醒",
      }),
    });
  });

  const configurationRoutes: Array<[string, string, JsonRecord, string?]> = [
    ["email/smtp-configuration", "email", { smtp: {} }],
    ["sms/tencent-cloud-configuration", "sms", { tencentSms: {} }, "tencent_cloud_sms_v20210111"],
    ["sms/aliyun-configuration", "sms", { aliyunSms: {} }, "aliyun_sms_v20170525"],
    ["sms/smsbao-configuration", "sms", { smsBao: {} }, "smsbao_http_v1"],
    [
      "wechat-official/service-account-configuration",
      "wechat_official",
      { weChatOfficial: { callbackVerified: false } },
    ],
    ["wechat-work/custom-app-configuration", "wechat_work", { weCom: {} }],
    ["signed-webhook/configuration", "signed_webhook", { signedWebhook: {} }],
  ];
  for (const [suffix, code, configuration, providerCode] of configurationRoutes) {
    await page.route(`**${adminPrefix}/channels/${suffix}`, async (route: Route) => {
      const { request, url } = track(route);
      if (!(await authorize(route, true))) return;
      const body = request.postDataJSON() as JsonRecord;
      const current = channel(code);
      if (!current) {
        await reject(route, 404, "channel not found");
        return;
      }
      if (Number(body.expectedVersion) !== current.version) {
        await reject(route, 409, "channel version conflict");
        return;
      }
      if (code === "signed_webhook") {
        let endpoint: URL | undefined;
        try {
          endpoint = new URL(String(body.endpointUrl ?? ""));
        } catch {
          endpoint = undefined;
        }
        const deniedHost = endpoint?.hostname === "localhost" || endpoint?.hostname === "127.0.0.1" ||
          endpoint?.hostname === "::1" || endpoint?.hostname.startsWith("10.") ||
          endpoint?.hostname.startsWith("192.168.") || /^172\.(1[6-9]|2\d|3[01])\./u.test(endpoint?.hostname ?? "");
        if (!endpoint || endpoint.protocol !== "https:" || deniedHost || String(body.signingSecret ?? "").length < 32) {
          await reject(route, 400, "signed webhook endpoint is unsafe");
          return;
        }
      }
      state.configurationRequests.push({ body, path: url.pathname });
      current.configured = true;
      current.configurationGeneration += 1;
      current.credentialMaskedTail = "4f2a";
      current.status = "disabled";
      current.version += 1;
      if (code === "email") {
        current.configuration = {
          smtp: {
            fromAddress: body.fromAddress,
            fromName: body.fromName,
            host: body.host,
            port: body.port,
            security: body.security,
            username: body.username,
          },
        };
      } else if (providerCode === "tencent_cloud_sms_v20210111") {
        current.providerCode = providerCode;
        current.configuration = {
          tencentSms: {
            region: body.region,
            signName: body.signName,
            smsSdkAppId: body.smsSdkAppId,
          },
        };
      } else if (providerCode === "aliyun_sms_v20170525") {
        current.providerCode = providerCode;
        current.configuration = { aliyunSms: { signName: body.signName } };
      } else if (providerCode === "smsbao_http_v1") {
        current.providerCode = providerCode;
        current.configuration = {
          smsBao: { productId: body.productId, signName: body.signName },
        };
      } else {
        current.configuration = configuration;
      }
      await route.fulfill({ json: apiEnvelope({ item: current }) });
    });
  }

  await page.route(`**${adminPrefix}/channels/*/tests`, async (route: Route) => {
    const { request, url } = track(route);
    if (!(await authorize(route, true))) return;
    const routeCode = url.pathname.split("/").at(-2) ?? "";
    const code = routeCode === "wechat-work" ? "wechat_work" : routeCode;
    if (!["email", "sms", "wechat_official", "wechat_work"].includes(code)) {
      await reject(route, 400, "channel test is unavailable");
      return;
    }
    const body = request.postDataJSON() as JsonRecord;
    if (code === "wechat_work" && ("target" in body || "userId" in body)) {
      await reject(route, 400, "WeCom self test must not accept a destination");
      return;
    }
    if (code === "wechat_work" && !weComBinding.bound) {
      await reject(route, 409, "current staff WeCom binding is unavailable");
      return;
    }
    state.testRequests.push({ body, path: url.pathname });
    const target = String(body.target ?? "");
    const targetMask = code === "sms"
      ? "+86 138****8000"
      : code === "wechat_official"
        ? "oP***8z"
        : code === "wechat_work"
          ? "current_staff_binding"
          : "t***@example.com";
    await route.fulfill({
      json: apiEnvelope({
        item: {
          channelCode: code,
          completedAt: fixedNow,
          operationNo: `TEST-${code.toUpperCase()}-001`,
          resultClass: "accepted",
          reused: false,
          targetMask: code === "wechat_work" ? targetMask : target ? targetMask : "",
        },
      }),
    });
  });

  for (const actionSuffix of [
    "disablement",
    "enablement",
    "configuration-clearance",
  ]) {
    await page.route(
      `**${adminPrefix}/channels/*/${actionSuffix}`,
      async (route: Route) => {
        const { request, url } = track(route);
        if (!(await authorize(route, true))) return;
        const body = request.postDataJSON() as JsonRecord;
        const segments = url.pathname.split("/");
        const action = segments.at(-1);
        const code = segments.at(-2) ?? "";
        const current = channel(code);
        if (!current) {
          await reject(route, 404, "channel not found");
          return;
        }
        if (body.confirmed !== true) {
          await reject(route, 400, "explicit confirmation required");
          return;
        }
        if (Number(body.expectedVersion) !== current.version) {
          await reject(route, 409, "channel version conflict");
          return;
        }
        state.actionRequests.push({ body, method: request.method(), path: url.pathname });
        if (action === "configuration-clearance" && current.pendingQueryCount > 0) {
          await reject(route, 409, "pending provider result checks exist");
          return;
        }
        if (action === "disablement") {
          for (const delivery of deliveries) {
            if (delivery.channelCode === code && !delivery.externalBoundaryCrossed &&
              ["queued", "retry_wait"].includes(delivery.status)) {
              delivery.status = "cancelled";
              delivery.resultClass = "channel_disabled";
              delivery.completedAt = fixedNow;
              delivery.nextAttemptAt = undefined;
              delivery.version += 1;
            }
          }
        }
        current.status = action === "disablement" ? "disabled" : "enabled";
        if (action === "configuration-clearance") {
          current.configured = false;
          current.configuration = {};
          current.credentialMaskedTail = "";
          current.status = "unconfigured";
        }
        current.version += 1;
        await route.fulfill({ json: apiEnvelope({ item: current }) });
      },
    );
  }

  await page.route(`**${adminPrefix}/deliveries?*`, async (route: Route) => {
    const { url } = track(route);
    if (!(await authorize(route))) return;
    state.deliveryListRequests.push(url);
    const cursor = url.searchParams.get("cursor");
    if (cursor === "delivery-page-2") {
      await route.fulfill({
        json: apiEnvelope({
          items: [
            {
              ...deliveries[0],
              channelCode: "in_app",
              completedAt: fixedNow,
              eventCode: "account.password.reset",
              recipientMask: "CUS-***-001",
              resultClass: "delivered",
              status: "delivered",
              taskNo: "TASK-DELIVERED-002",
            },
          ],
          nextCursor: "",
        }),
      });
      return;
    }
    await route.fulfill({
      json: apiEnvelope({ items: deliveries, nextCursor: "delivery-page-2" }),
    });
  });

  await page.route(`**${adminPrefix}/deliveries/*/resend-eligibility`, async (route: Route) => {
    const { url } = track(route);
    if (!(await authorize(route))) return;
    const taskNo = url.pathname.split("/").at(-2) ?? "";
    state.eligibilityRequests.push(taskNo);
    const callNumber = state.eligibilityRequests.length;
    if (options.eligibilityDeniedCalls?.includes(callNumber)) {
      await reject(route, 403, "delivery is no longer accessible");
      return;
    }
    if (options.eligibilityDelayCalls?.includes(callNumber)) {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    if (options.eligibilityFailureCalls?.includes(callNumber)) {
      await reject(route, 503, "eligibility temporarily unavailable");
      return;
    }
    const current = deliveries.find((item) => item.taskNo === taskNo);
    const mode = taskNo === "TASK-FAILED-001" ? (options.resendMode ?? "allowed") : "cooldown";
    const remainingCount = mode === "limit" ? 0 : Math.max(0, 3 - (current?.manualResendCount ?? 0));
    const allowed = mode === "allowed" && current?.status === "failed" && current.isChainTail && remainingCount > 0;
    const reasons = options.eligibilityReasons ?? (allowed
      ? []
      : mode === "limit"
        ? ["limit_reached"]
        : current?.status === "manual_review"
          ? ["task_state_changed"]
          : ["cooldown"]);
    await route.fulfill({
      json: apiEnvelope({
        item: {
          allowed,
          evaluatedAt: fixedNow,
          nextAllowedAt: mode === "cooldown" ? fixedNow + 600_000 : undefined,
          reasons,
          remainingCount,
          replacedByTaskNo: "",
          taskNo,
          taskVersion: current?.version ?? 0,
        },
      }),
    });
  });

  await page.route(`**${adminPrefix}/deliveries/*/resends`, async (route: Route) => {
    const { request, url } = track(route);
    if (!(await authorize(route, true))) return;
    const taskNo = url.pathname.split("/").at(-2) ?? "";
    const current = deliveries.find((item) => item.taskNo === taskNo);
    const body = request.postDataJSON() as JsonRecord;
    state.actionRequests.push({ body, method: request.method(), path: url.pathname });
    const operationNo = String(body.operationNo ?? "");
    const replay = resendReplays.get(operationNo);
    if (replay) {
      await route.fulfill({ json: apiEnvelope({ item: replay }) });
      return;
    }
    if (!current || current.status !== "failed" || current.version !== Number(body.expectedVersion) ||
      body.confirmed !== true || !String(body.reason ?? "").trim() || options.resendMode !== undefined && options.resendMode !== "allowed") {
      await reject(route, 409, "task state changed");
      return;
    }
    const child: MockDelivery = {
      ...current,
      completedAt: undefined,
      manualResendCount: current.manualResendCount + 1,
      previousTaskNo: current.taskNo,
      queuedAt: fixedNow,
      resultClass: "",
      retrySequence: 0,
      status: "queued",
      taskNo: "TASK-RESEND-001",
      version: 1,
    };
    current.isChainTail = false;
    deliveries.unshift(child);
    resendReplays.set(operationNo, child);
    await route.fulfill({ json: apiEnvelope({ item: child }) });
  });

  await page.route(`**${adminPrefix}/deliveries/*/unknown-closures`, async (route: Route) => {
    const { request, url } = track(route);
    if (!(await authorize(route, true))) return;
    const taskNo = url.pathname.split("/").at(-2) ?? "";
    const current = deliveries.find((item) => item.taskNo === taskNo);
    const body = request.postDataJSON() as JsonRecord;
    state.actionRequests.push({ body, method: request.method(), path: url.pathname });
    if (!current || current.status !== "manual_review" || current.version !== Number(body.expectedVersion) ||
      body.confirmed !== true || !String(body.reason ?? "").trim()) {
      await reject(route, 409, "task state changed");
      return;
    }
    current.status = "manual_unknown_closed";
    current.version += 1;
    current.completedAt = fixedNow;
    state.closureHistory.push({
      actor: 1,
      operationNo: String(body.operationNo ?? ""),
      reason: String(body.reason),
      taskNo,
    });
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  await page.route(`**${adminPrefix}/deliveries/*`, async (route: Route) => {
    const { url } = track(route);
    if (!(await authorize(route))) return;
    const taskNo = url.pathname.split("/").at(-1) ?? "";
    const current = deliveries.find((item) => item.taskNo === taskNo);
    if (!current) {
      await reject(route, 404, "delivery not found");
      return;
    }
    await route.fulfill({ json: apiEnvelope({ item: current }) });
  });

  return state;
}

export function assertNotificationMockClean(state: NotificationCenterMockState) {
  if (state.unexpectedRequests.length > 0) {
    throw new Error(
      `Undefined notification APIs were requested: ${state.unexpectedRequests.join(", ")}`,
    );
  }
  const invalid = state.allNotificationRequests.filter(
    (path) => !path.startsWith(notificationPrefix),
  );
  if (invalid.length > 0) {
    throw new Error(`Notification requests escaped the plugin prefix: ${invalid.join(", ")}`);
  }
}
