import { config } from "@host-tests/fixtures/config";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import type { Page, Route } from "@host-tests/support/playwright";

const pluginRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const pluginMessages = new Map<string, Promise<Record<string, string>>>();

function flattenMessages(
  value: unknown,
  prefix = "",
  result: Record<string, string> = {},
) {
  if (typeof value === "string") {
    if (prefix) result[prefix] = value;
    return result;
  }
  if (!value || typeof value !== "object" || Array.isArray(value))
    return result;
  for (const [key, item] of Object.entries(value)) {
    flattenMessages(item, prefix ? `${prefix}.${key}` : key, result);
  }
  return result;
}

function loadPluginMessages(locale: "en-US" | "zh-CN") {
  const existing = pluginMessages.get(locale);
  if (existing) return existing;
  const loading = Promise.all(
    ["plugin.json", "error.json", "merchant-workspace.json"].map((fileName) =>
      readFile(
        resolve(pluginRoot, "manifest", "i18n", locale, fileName),
        "utf8",
      ),
    ),
  ).then(([pluginContent, errorContent, merchantContent]) => ({
    ...(JSON.parse(pluginContent!) as Record<string, string>),
    ...flattenMessages(JSON.parse(errorContent!)),
    ...flattenMessages(JSON.parse(merchantContent!)),
  }));
  pluginMessages.set(locale, loading);
  return loading;
}

interface AutomationInterfaceFixture {
  canDelete: boolean;
  changedAt: number;
  createdAt: number;
  displayName: string;
  interfaceInstanceNo: string;
  newBusinessState: "accepting" | "stopped";
  version: number;
}

const providers = Array.from({ length: 121 }, (_, index) => ({
  displayName:
    index === 0
      ? "证书供应商插件"
      : index === 100
        ? "手机供应商插件"
        : `商品履约插件 ${index + 1}`,
  selectionToken: `short-lived-provider-${index + 1}`,
}));

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

export interface CatalogAutomationMockState {
  bindingRequests: Array<{ body?: unknown; method: string }>;
  interfaceRequests: Array<{
    body?: unknown;
    method: string;
    url: URL;
  }>;
  providerRequests: URL[];
}

export async function installCatalogAutomationMock(
  page: Page,
  options: {
    failDeleteRace?: boolean;
    failResumeEvidence?: boolean;
    failSelectionToken?: boolean;
    permissions?: string[];
  } = {},
) {
  const state: CatalogAutomationMockState = {
    bindingRequests: [],
    interfaceRequests: [],
    providerRequests: [],
  };
  await page.evaluate(() => {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
    }
  });
  const interfaces: AutomationInterfaceFixture[] = [
    {
      canDelete: true,
      changedAt: 1_787_705_660_000,
      createdAt: 1_787_705_600_000,
      displayName: "证书供应商 A 接口",
      interfaceInstanceNo: "hidden-interface-certificate",
      newBusinessState: "accepting",
      version: 3,
    },
    {
      canDelete: false,
      changedAt: 1_787_705_660_000,
      createdAt: 1_787_705_600_000,
      displayName: "手机供应商 A 接口",
      interfaceInstanceNo: "hidden-interface-phone",
      newBusinessState: "stopped",
      version: 2,
    },
  ];
  interfaces.push(
    ...Array.from({ length: 121 }, (_, index) => ({
      canDelete: true,
      changedAt: 1_787_705_660_000 + index,
      createdAt: 1_787_705_600_000 + index,
      displayName: `批量供应商接口 ${index + 1}`,
      interfaceInstanceNo: `hidden-interface-bulk-${index + 1}`,
      newBusinessState: "accepting" as const,
      version: 1,
    })),
  );

  if (options.permissions) {
    await page.route("**/api/v1/user/info", async (route: Route) => {
      const response = await route.fetch({ url: new URL(new URL(route.request().url()).pathname + new URL(route.request().url()).search, config.backendBaseURL).href });
      const payload = (await response.json()) as {
        data?: { permissions?: string[] };
      };
      if (!payload.data) throw new Error("user-info payload is unavailable");
      payload.data.permissions = [...(options.permissions ?? [])];
      await route.fulfill({ json: payload, response });
    });
    const restrictStoredAccess=(permissions:string[])=>{
      // Production builds use a different storage namespace from dev mode.
      // Patch only the existing access store in this isolated test context.
      for(const key of Object.keys(localStorage)) {
        if(!key.endsWith('-core-access')) continue;
        const raw=localStorage.getItem(key);
        if(!raw) continue;
        // Dev stores are JSON; production stores are encrypted. Never decode
        // credentials here. A fresh page loads production permissions via /user/info.
        try {
          const state=JSON.parse(raw);
          localStorage.setItem(key,JSON.stringify({...state,accessCodes:permissions}));
        } catch {continue;}
      }
    };
    await page.addInitScript(restrictStoredAccess,options.permissions);
    await page.evaluate(restrictStoredAccess,options.permissions);
  }

  await page.route(
    "**/api/v1/i18n/runtime/messages**",
    async (route: Route) => {
      const response = await route.fetch({ url: new URL(new URL(route.request().url()).pathname + new URL(route.request().url()).search, config.backendBaseURL).href });
      const payload = (await response.json()) as {
        data?: { messages?: Record<string, unknown> };
      };
      if (!payload.data) throw new Error("runtime i18n payload is unavailable");
      const locale =
        new URL(route.request().url()).searchParams.get("lang") === "en-US"
          ? "en-US"
          : "zh-CN";
      payload.data.messages ??= {};
      // Override the nested runtime dictionary as well as new flat keys.
      // Existing keys otherwise keep the installed plugin's older translation.
      for (const [key, value] of Object.entries(await loadPluginMessages(locale))) {
        const segments = key.split(".");
        let target = payload.data.messages;
        for (const segment of segments.slice(0, -1)) {
          if (!target[segment] || typeof target[segment] !== "object") target[segment] = {};
          target = target[segment] as Record<string, unknown>;
        }
        target[segments.at(-1)!] = value;
      }
      await route.fulfill({
        headers: {
          ...response.headers(),
          etag: `catalog-automation-${locale}`,
        },
        json: payload,
        status: 200,
      });
    },
  );
  await page.route("**/api/v1/menus/all", async (route: Route) => {
    const response = await route.fetch({ url: new URL(new URL(route.request().url()).pathname + new URL(route.request().url()).search, config.backendBaseURL).href });
    const payload = (await response.json()) as {
      data?: { list?: Array<Record<string, unknown>> };
    };
    if (!payload.data) throw new Error("menu payload is unavailable");
    const list = payload.data.list ?? [];
    const queue = [...list];
    let automationMenu: Record<string, unknown> | undefined;
    while (queue.length > 0) {
      const item = queue.shift();
      if (!item) continue;
      if (item.path === "/idc/catalog/automation-interfaces") {
        automationMenu = item;
        break;
      }
      const children = item.children;
      if (Array.isArray(children)) {
        queue.push(...(children as Array<Record<string, unknown>>));
      }
    }
    const english = (
      route.request().headers()["accept-language"] ?? ""
    ).startsWith("en");
    const title = english ? "Automation Interfaces" : "自动化接口";
    if (automationMenu) {
      automationMenu.meta = {
        ...((automationMenu.meta as Record<string, unknown> | undefined) ?? {}),
        title,
      };
    } else {
      list.push({
        component: "#/views/system/plugin/dynamic-page",
        id: -14_014,
        meta: { title },
        name: "CatalogAutomationMockContract",
        parentId: 0,
        path: "/idc/catalog/automation-interfaces",
      });
    }
    payload.data.list = list;
    await route.fulfill({
      json: payload,
      response,
    });
  });

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/automation/providers**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.providerRequests.push(url);
      const pageNumber = Number(url.searchParams.get("page") || "1");
      const pageSize = Number(url.searchParams.get("pageSize") || "20");
      const start = (pageNumber - 1) * pageSize;
      await route.fulfill({
        json: apiEnvelope({
          items: providers.slice(start, start + pageSize),
          total: providers.length,
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/automation/interfaces**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      const marker = "/admin/catalog/automation/interfaces/";
      const body =
        request.method() === "GET" ? undefined : request.postDataJSON();
      state.interfaceRequests.push({ body, method: request.method(), url });

      if (request.method() === "POST") {
        if (options.failSelectionToken) {
          await page.getByText(/加载菜单中|Loading Menu/iu).waitFor({
            state: "hidden",
            timeout: 15_000,
          });
          await route.fulfill({
            json: {
              code: 409,
              data: null,
              messageKey:
                "plugin.idc-core.catalogAutomation.errors.selectionExpired",
            },
            status: 409,
          });
          return;
        }
        const input = body as { displayName: string; selectionToken: string };
        const created: AutomationInterfaceFixture = {
          canDelete: true,
          changedAt: 1_787_705_720_000,
          createdAt: 1_787_705_720_000,
          displayName: input.displayName,
          interfaceInstanceNo: "hidden-interface-created",
          newBusinessState: "accepting",
          version: 1,
        };
        interfaces.unshift(created);
        await route.fulfill({ json: apiEnvelope({ interface: created }) });
        return;
      }

      if (url.pathname.includes(marker)) {
        const interfaceNo = decodeURIComponent(
          url.pathname.split(marker)[1]?.split("/")[0] || "",
        );
        const item = interfaces.find(
          (candidate) => candidate.interfaceInstanceNo === interfaceNo,
        );
        if (!item) {
          await route.fulfill({ status: 404 });
          return;
        }
        if (request.method() === "DELETE") {
          if (options.failDeleteRace) {
            await route.fulfill({
              json: {
                code: 409,
                data: null,
                messageKey:
                  "error.idc.product.automation.interface.ever.referenced",
              },
              status: 409,
            });
            return;
          }
          interfaces.splice(interfaces.indexOf(item), 1);
          await route.fulfill({ json: apiEnvelope({}) });
          return;
        }
        const input = body as Record<string, unknown>;
        if (url.pathname.endsWith("/new-business-state")) {
          if (
            options.failResumeEvidence &&
            input.newBusinessState === "accepting"
          ) {
            await route.fulfill({
              json: {
                code: 503,
                data: null,
                messageKey: "error.idc.product.automation.evidence.unavailable",
              },
              status: 503,
            });
            return;
          }
          item.newBusinessState = input.newBusinessState as
            | "accepting"
            | "stopped";
        } else {
          item.displayName = String(input.displayName);
        }
        item.version += 1;
        await route.fulfill({ json: apiEnvelope({ interface: item }) });
        return;
      }

      const namePrefix = url.searchParams.get("namePrefix") || "";
      const newBusinessState = url.searchParams.get("newBusinessState") || "";
      const providerSelectionToken =
        url.searchParams.get("providerSelectionToken") || "";
      const filtered = interfaces.filter(
        (item) =>
          (!providerSelectionToken ||
            (providerSelectionToken === "short-lived-provider-1"
              ? item.interfaceInstanceNo !== "hidden-interface-phone"
              : providerSelectionToken === "short-lived-provider-101"
                ? item.interfaceInstanceNo === "hidden-interface-phone"
                : false)) &&
          (!namePrefix || item.displayName.startsWith(namePrefix)) &&
          (!newBusinessState || item.newBusinessState === newBusinessState),
      );
      const pageNumber = Number(url.searchParams.get("page") || "1");
      const pageSize = Number(url.searchParams.get("pageSize") || "20");
      const start = (pageNumber - 1) * pageSize;
      await route.fulfill({
        json: apiEnvelope({
          items: filtered.slice(start, start + pageSize),
          total: filtered.length,
        }),
      });
    },
  );

  await page.route(
    "**/x/idc-core/api/v1/admin/catalog/product-versions/*/fulfillment-binding",
    async (route: Route) => {
      const request = route.request();
      const body =
        request.method() === "PUT" ? request.postDataJSON() : undefined;
      state.bindingRequests.push({ body, method: request.method() });
      await route.fulfill({
        json: apiEnvelope(
          request.method() === "PUT"
            ? {
                binding: {
                  boundAt: 1_787_705_720_000,
                  evidenceVerified: true,
                  interfaceDisplayName: "证书供应商 A 接口",
                  interfaceInstanceNo: "hidden-interface-certificate",
                  interfaceNewBusinessState: "accepting",
                },
              }
            : {},
        ),
      });
    },
  );

  return state;
}
