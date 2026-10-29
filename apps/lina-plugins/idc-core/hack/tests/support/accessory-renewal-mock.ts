import { readFileSync } from "node:fs";
import type { Page, Route } from "@host-tests/support/playwright";

export const renewalPermission = "idc:admin:resource:accessory-renewal:handle";
export const invoicePermissions = ["idc:admin:billing:overview", "idc:admin:billing:invoice:list", "idc:admin:billing:invoice:query"];
export const workItems = Array.from({ length: 21 }, (_, index) => ({
  workItemNo: `renewal-work-${index + 1}`,
  invoiceNo: `renewal-invoice-${index + 1}`,
  customerNo: "customer-renewal-fixture",
  subjectNo: `snapshot-${index + 1}`,
  reasonCode: index % 2 ? "deletion_invoice_pending" : "balance_payment_failed",
  createdAt: 1_792_681_200_000,
}));

// UI-only fixtures: renewal requests are never forwarded to a real service.
// Keep the authenticated identity; only the browser's permission projection is
// narrowed for visibility tests. No account impersonation or business writes.
export async function installAccessoryRenewalMock(page: Page, codes = [renewalPermission, ...invoicePermissions]) {
  const state = {
    codes,
    failSync: false,
    failList: false,
    syncBodies: [] as unknown[],
    listQueries: [] as { page: number; pageSize: number }[],
    holdNextList: false,
    heldList: undefined as Route | undefined,
  };
  await page.route("**/api/v1/menus/all", async route => {
    const response = await route.fetch();
    const body = await response.json();
    const list = body.data?.list || [];
    const parent = list.find((item: {path: string}) => item.path === "/idc-resource-operations");
    if (parent && !parent.children?.some((item: {path: string}) => item.path === "/idc/resource/accessory-renewals")) {
      parent.children = [...(parent.children || []), {id: 999081, parentId: parent.id, name: "AccessoryRenewalWorkItems", path: "/idc/resource/accessory-renewals", component: "#/views/system/plugin/dynamic-page", meta: {title: "附属资源续费待办", i18nKey: "menu.plugin:idc-core:accessory-renewal:handle.title", icon: "lucide:alarm-clock-check", order: 26}}];
    }
    await route.fulfill({response, json: body});
  });
  await page.route("**/api/v1/user/info", async route => {
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({ response, json: { ...body, data: { ...body.data, permissions: state.codes } } });
  });
  await page.route("**/api/v1/i18n/runtime/messages?*", async route => {
    const headers = { ...route.request().headers() };
    delete headers["if-none-match"];
    const response = await route.fetch({ headers });
    const body = await response.json();
    const locale = new URL(route.request().url()).searchParams.get("lang") === "en-US" ? "en-US" : "zh-CN";
    const messages = JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/renewal.json`, import.meta.url), "utf8"));
    const menuMessages = JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/menu.json`, import.meta.url), "utf8"));
    const previous = body.data?.messages ?? {};
    await route.fulfill({ response, json: { ...body, data: { ...body.data, messages: {
      ...previous, menu: { ...previous.menu, ...menuMessages.menu }, plugin: { ...previous.plugin, "idc-core": { ...previous.plugin?.["idc-core"], ...messages.plugin["idc-core"] } },
    } } } });
  });
  await page.route("**/admin/server-accessory-renewal-notification-syncs", async route => {
    state.syncBodies.push(route.request().postDataJSON());
    await route.fulfill({ status: state.failSync ? 503 : 200, json: state.failSync
      ? { code: 50, message: "Renewal reminders temporarily unavailable" }
      : { code: 0, data: { total: workItems.length, eventNo: "renewal-event-fixture" } } });
  });
  await page.route("**/admin/server-accessory-renewal-work-items?*", async route => {
    const url = new URL(route.request().url());
    const pageNumber = Number(url.searchParams.get("page"));
    const pageSize = Number(url.searchParams.get("pageSize"));
    state.listQueries.push({ page: pageNumber, pageSize });
    if (state.holdNextList) { state.holdNextList = false; state.heldList = route; return; }
    await route.fulfill({ status: state.failList ? 503 : 200, json: state.failList
      ? { code: 50, message: "Renewal work items temporarily unavailable" }
      : { code: 0, data: { items: workItems.slice((pageNumber - 1) * pageSize, pageNumber * pageSize), total: workItems.length } } });
  });
  return state;
}
