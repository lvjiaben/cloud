/** Public HTTP adapters; the workspace never accesses host-private services. */
import { pluginApiPath, requestClient } from "#/api/request";

export interface PluginRow {
  id: string;
  name: string;
  version: string;
  effectiveVersion?: string;
  discoveredVersion?: string;
  upgradeAvailable: boolean;
  installed: number;
  enabled: number;
  newBusinessState: string;
  runtimeState: string;
  installedAt: number | null;
  authorizationRequired: number;
  authorizationStatus: string;
}
export interface HostService {
  owner?: string;
  service: string;
  version?: string;
  methods: string[];
  paths?: string[];
  tables?: string[];
  resources?: Array<{ ref: string }>;
}
export interface Dependencies {
  blockers?: Array<{ code: string; pluginId?: string; dependencyId?: string }>;
  reverseDependents?: Array<{ pluginId: string; name?: string }>;
  reverseBlockers?: Array<{
    code: string;
    pluginId?: string;
    dependencyId?: string;
  }>;
  cycle?: string[];
  framework?: { status: string };
}
export interface PluginDetail extends PluginRow {
  type?: string;
  distribution?: string;
  lastUpgradeFailure?: unknown;
  requestedHostServices?: HostService[];
  businessExtensions?: BusinessExtension[];
  authorizedBusinessExtensions?: BusinessExtension[];
  dependencyCheck?: Dependencies;
}
export interface BusinessExtension {
  point: string;
  version: string;
  role: string;
  owner: string;
  methods?: string[];
}
const extensionKey = (item: BusinessExtension) =>
  JSON.stringify([
    item.point,
    item.version,
    item.role,
    item.owner,
    [...(item.methods ?? [])].sort(),
  ]);
export function missingBusinessAuthorization(plugin: PluginDetail) {
  const authorized = new Set(
    (plugin.authorizedBusinessExtensions ?? []).map(extensionKey),
  );
  return (plugin.businessExtensions ?? []).some(
    (item) => item.role === "provider" && !authorized.has(extensionKey(item)),
  );
}
export interface UpgradePreview {
  effectiveVersion: string;
  discoveredVersion: string;
  dependencyCheck?: Dependencies;
  toManifest?: { requestedHostServices?: HostService[] };
  hostServicesDiff?: { authorizationRequired?: boolean };
}
export interface Category {
  id: string;
  name: string;
}
const api = (path: string) => pluginApiPath("plugin-manager", path);
const host = (id: string, suffix = "") =>
  `/plugins/${encodeURIComponent(id)}${suffix}`;
const actionOptions = { timeout: 120_000 };
export async function categories() {
  return (await requestClient.get<{ list: Category[] }>(api("categories")))
    .list;
}
export async function listPlugins(params: Record<string, unknown>) {
  const data = await requestClient.get<{ list: PluginRow[]; total: number }>(
    api("plugins"),
    { params },
  );
  return { items: data.list, total: data.total };
}
export const detail = (id: string) => requestClient.get<PluginDetail>(host(id));
export const dependencies = (id: string) =>
  requestClient.get<Dependencies>(host(id, "/dependencies"));
export const preview = (id: string) =>
  requestClient.get<UpgradePreview>(host(id, "/upgrade/preview"));
export const enable = (id: string, payload?: unknown) =>
  requestClient.put(host(id, "/enable"), payload, actionOptions);
export const disable = (id: string) =>
  requestClient.put(host(id, "/disable"), undefined, actionOptions);
export const business = (id: string, accepting: boolean) =>
  requestClient.put(
    host(id, accepting ? "/new-business/resume" : "/new-business/stop"),
  );
export const install = (id: string, payload: unknown) =>
  requestClient.post(host(id, "/install"), payload, actionOptions);
export const upgrade = (id: string, payload: Record<string, unknown>) =>
  requestClient.post(
    host(id, "/upgrade"),
    { ...payload, confirmed: true },
    actionOptions,
  );
export const uninstall = (id: string) =>
  requestClient.delete(host(id), {
    ...actionOptions,
    params: { purgeStorageData: 0 },
  });
export async function upload(file: File) {
  const body = new FormData();
  body.append("file", file, file.name);
  return requestClient.post<PluginRow>("/plugins/dynamic/package", body, {
    ...actionOptions,
    headers: { "Content-Type": "multipart/form-data" },
  });
}
export function authorization(services: HostService[]) {
  return {
    authorization: {
      services: services
        .filter(
          (item) =>
            item.methods?.length ||
            item.paths?.length ||
            item.tables?.length ||
            item.resources?.length,
        )
        .map((item) => ({
          owner: item.owner || undefined,
          service: item.service,
          version: item.version || undefined,
          methods: item.methods,
          paths: item.service === "storage" ? item.paths : undefined,
          tables: item.service === "data" ? item.tables : undefined,
          resourceRefs:
            item.service === "storage" || item.service === "data"
              ? undefined
              : item.resources?.map((resource) => resource.ref),
        })),
    },
  };
}

/** One confirmed installation produces a usable plugin, including a reinstall. */
export async function installAndStart(id: string) {
  let current = await detail(id);
  if (current.installed !== 1) {
    await install(id, authorization(current.requestedHostServices ?? []));
    current = await detail(id);
  }
  if (current.enabled !== 1 || missingBusinessAuthorization(current))
    await enable(id, authorization(current.requestedHostServices ?? []));
  if (current.newBusinessState === "stopped") await business(id, true);
}

/** Keep the lifecycle steps behind the user's single confirmation. */
export async function stopAndRemove(
  id: string,
  operation: "disable" | "uninstall",
) {
  let current = await detail(id);
  if (current.newBusinessState !== "stopped") {
    await business(id, false);
    current = await detail(id);
  }
  // Status synchronization may drop the host's business authorization snapshot.
  // Revalidate through the public endpoint; never edit private authorization data.
  if (missingBusinessAuthorization(current))
    await enable(id, authorization(current.requestedHostServices ?? []));
  await (operation === "disable" ? disable(id) : uninstall(id));
}
