import { existsSync, readFileSync } from "node:fs";
import { isAbsolute, relative, resolve } from "node:path";
import type { Page } from "@host-tests/support/playwright";

// Project the actual menu declaration and retain ancestors of granted child pages.
// This reproduces the host's role filtering without inventing workspace grants.
export async function installResourceWorkspaceMenus(page: Page, permissions: string[]) {
  const staticUI = process.env.E2E_STATIC_UI_DIR;
  if (staticUI) {
    const state = JSON.parse(readFileSync(new URL("../../../../../../hack/tests/temp/storage-state/admin.json", import.meta.url), "utf8"));
    await page.addInitScript((entries: Array<{ name: string; value: string }>) => {
      for (const item of entries) localStorage.setItem(item.name, item.value);
    }, state.origins[0]?.localStorage ?? []);
  }
  if (staticUI) await page.route(url => url.pathname.startsWith("/admin/"), route => {
    const pathname = new URL(route.request().url()).pathname.replace(/^\/admin\//u, "");
    const file = resolve(staticUI, pathname);
    const path = relative(staticUI, file);
    return route.fulfill({ path: path && !path.startsWith("..") && !isAbsolute(path) && existsSync(file) ? file : resolve(staticUI, "index.html") });
  });
  const manifest = readFileSync(new URL("../../../plugin.yaml", import.meta.url), "utf8");
  const menus = [...manifest.matchAll(/^  - key: (plugin:idc-resource:[^\n]+)\n([\s\S]*?)(?=^  - key: |$(?![\s\S]))/gm)];
  const specs = menus.map((match) => {
    const value = (name: string) => match[2]!.match(new RegExp(`^    ${name}: (.+)$`, "m"))?.[1] ?? "";
    return { key: match[1]!, parent: value("parent_key"), path: value("path"),
      permission: value("perms"), hidden: value("visible") === "0", type: value("type"),
      query: value("query_param") ? JSON.parse(value("query_param").slice(1, -1)) : undefined };
  });
  const selected = new Set<string>();
  for (const spec of specs.filter((item) => permissions.includes(item.permission))) {
    let current: typeof spec | undefined = spec;
    while (current) {
      selected.add(current.key);
      current = specs.find((item) => item.key === current?.parent);
    }
  }
  const route = (spec: typeof specs[number]): Record<string, unknown> => ({
    name: spec.key.replaceAll(":", "-"), path: spec.path, component: "system/plugin/dynamic-page",
    meta: { title: `menu.${spec.key}.title`, hideInMenu: spec.hidden, authority: spec.permission, query: spec.query },
    children: specs.filter((item) => selected.has(item.key) && item.parent === spec.key && item.type === "M").map(route),
  });
  const roots = specs.filter((item) => selected.has(item.key) && item.parent === "plugin:idc-core:resource-operations" && item.type === "M");
  await page.route("**/api/v1/menus/all", (request) => request.fulfill({ json: {
    code: 0, message: "success", data: { list: [{ name: "ResourceOperations", path: "/idc-resources",
      component: "BasicLayout", meta: { title: "服务器与资源" }, children: roots.map(route) }] },
  } }));
  await page.route("**/api/v1/user/info", async (request) => {
    const response = await request.fetch();
    const payload = await response.json();
    payload.data.permissions = permissions;
    payload.data.homePath = "/idc/resource/provider-configuration";
    await request.fulfill({ response, json: payload });
  });
  for (const locale of ["zh-CN", "en-US"]) {
    const plugin = JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/plugin.json`, import.meta.url), "utf8"));
    const menu = JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/menu.json`, import.meta.url), "utf8"));
    // Use current plugin and menu resources for the workspace assertions.
    const messages: Record<string, unknown> = { ...menu };
    for (const [key, value] of Object.entries(plugin)) {
      const parts = key.split(".");
      let target = messages;
      for (const part of parts.slice(0, -1)) {
        target[part] ??= {};
        target = target[part] as Record<string, unknown>;
      }
      target[parts.at(-1)!] = value;
    }
    await page.route(`**/api/v1/i18n/runtime/messages*lang=${locale}*`, async (request) => {
      await request.fulfill({ json: { code: 0, message: "success", data: { messages } } });
    });
  }
  return roots.map((root) => route(root));
}
