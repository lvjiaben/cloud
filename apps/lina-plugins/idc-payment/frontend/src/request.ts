import type { PluginHostMountContext } from "./host-context";
type Envelope<T> = { code?: number | string; data?: T };
export function createRequest(current: () => PluginHostMountContext) {
  return async function request<T>(path: string, init: RequestInit & { bodyObject?: unknown } = {}): Promise<T> {
    const controller = new AbortController();
    const timeout = globalThis.setTimeout(() => controller.abort(), 20_000);
    const { bodyObject, ...options } = init;
    try {
      const token = current().accessToken.trim();
      const response = await fetch(new URL(path, globalThis.location.origin), {
        ...options,
        body: bodyObject === undefined ? options.body : JSON.stringify(bodyObject),
        credentials: "same-origin",
        headers: { Accept: "application/json", ...(bodyObject === undefined ? {} : { "Content-Type": "application/json" }), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers },
        signal: controller.signal
      });
      const payload = await response.json() as Envelope<T> | T;
      if (!response.ok) throw Object.assign(new Error(`HTTP ${response.status}`), { status: response.status });
      if (payload && typeof payload === "object" && "data" in payload) {
        const envelope = payload as Envelope<T>;
        if (envelope.data === undefined || (envelope.code !== undefined && String(envelope.code) !== "0")) throw Object.assign(new Error(`API ${String(envelope.code ?? "unknown")}`), { code: envelope.code });
        return envelope.data;
      }
      return payload as T;
    } finally { globalThis.clearTimeout(timeout); }
  };
}
