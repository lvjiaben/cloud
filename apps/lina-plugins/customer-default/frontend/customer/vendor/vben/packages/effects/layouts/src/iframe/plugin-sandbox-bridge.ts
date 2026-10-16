export const PLUGIN_PAGE_BRIDGE_PROTOCOL = 'linapro.plugin-page-bridge';
export const PLUGIN_PAGE_BRIDGE_VERSION = '1.0';

export interface PluginSandboxAPIRequest {
  body?: unknown;
  capability: string;
  downloadFileName?: string;
  host: 'admin' | 'customer';
  method: 'DELETE' | 'GET' | 'PATCH' | 'POST' | 'PUT';
  params?: Record<string, unknown>;
  path: string;
  pluginId: string;
  surfaceKey: string;
  surfaceVersion: string;
  signal?: AbortSignal;
}

export interface PluginSandboxBridgeAdapter {
  request(input: PluginSandboxAPIRequest): Promise<unknown>;
}

const safePluginIDPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/**
 * Resolves only the owner selected by the authenticated grant issuer. The
 * sandbox request never carries this value, so an iframe cannot select an
 * arbitrary plugin namespace.
 */
export function resolvePluginSandboxRequestOwner(
  surfacePluginId: string,
  grantedOwnerPluginId?: unknown,
) {
  if (grantedOwnerPluginId === undefined || grantedOwnerPluginId === null) {
    return surfacePluginId;
  }
  if (
    typeof grantedOwnerPluginId !== 'string' ||
    !safePluginIDPattern.test(grantedOwnerPluginId) ||
    grantedOwnerPluginId.length > 128
  ) {
    throw new Error('Invalid plugin page bridge owner');
  }
  return grantedOwnerPluginId;
}

/**
 * Page-bridge targets can be either source plugins, which use the normal
 * `{ code, data }` response envelope, or dynamic plugins, which return their
 * guest body directly. Keep the sandbox contract stable for both kinds.
 */
export function unwrapPluginSandboxResponseBody(response: unknown) {
  if (
    response &&
    typeof response === 'object' &&
    'code' in response &&
    (response as { code?: unknown }).code === 0 &&
    'data' in response
  ) {
    return (response as { data?: unknown }).data;
  }
  return response;
}

let bridgeAdapter: null | PluginSandboxBridgeAdapter = null;
const prewarmedSources = new Set<string>();

export function configurePluginSandboxBridge(
  adapter: PluginSandboxBridgeAdapter,
) {
  bridgeAdapter = adapter;
}

export function getPluginSandboxBridgeAdapter() {
  return bridgeAdapter;
}

export function prewarmPluginSandboxSource(source: string) {
  if (typeof document === 'undefined') return;
  const normalized = source.trim();
  if (
    !normalized.startsWith('/x-assets/') ||
    prewarmedSources.has(normalized)
  ) {
    return;
  }
  prewarmedSources.add(normalized);
  const link = document.createElement('link');
  link.as = 'document';
  link.href = normalized;
  link.rel = 'prefetch';
  link.dataset.pluginSandboxPrewarm = normalized;
  document.head.append(link);
}
