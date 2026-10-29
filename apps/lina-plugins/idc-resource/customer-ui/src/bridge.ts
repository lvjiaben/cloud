// The parent retains authentication; this resource-owned page requests narrow customer APIs.
const protocol = "linapro.plugin-page-bridge",
  version = "1.0",
  pluginId = "idc-resource",
  surfaceKey = "purchased-services";
let sequence = 0;
const pending = new Map<
  string,
  {
    resolve: (result: any) => void;
    reject: (error: Error) => void;
    timer: ReturnType<typeof setTimeout>;
  }
>();
export function send(
  capability: string,
  method: string,
  payload: unknown,
  requestId?: string,
) {
  parent.postMessage(
    {
      protocol,
      version,
      direction: "plugin-to-host",
      pluginId,
      surfaceKey,
      capability,
      method,
      payload,
      requestId,
    },
    "*",
  );
}
export function request<T>(
  capability: string,
  method: string,
  path: string,
  payload: Record<string, unknown> = {},
): Promise<T> {
  const id = `resource-${++sequence}`;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error("Request timed out"));
    }, 30000);
    pending.set(id, { resolve, reject, timer });
    send(capability, "request", { method, path, ...payload }, id);
  });
}
export function listen(onContext: (context: Record<string, any>) => void) {
  const listener = (event: MessageEvent) => {
    const data = event.data;
    if (
      event.source !== parent ||
      data?.protocol !== protocol ||
      data.direction !== "host-to-plugin" ||
      data.pluginId !== pluginId ||
      data.surfaceKey !== surfaceKey
    )
      return;
    if (data.capability === "host.context.v1") onContext(data.payload || {});
    const action = pending.get(data.requestId);
    if (action) {
      clearTimeout(action.timer);
      pending.delete(data.requestId);
      data.ok
        ? action.resolve(data.result)
        : action.reject(new Error(data.result?.message || "Request failed"));
    }
  };
  window.addEventListener("message", listener);
  return () => {
    window.removeEventListener("message", listener);
    for (const item of pending.values()) {
      clearTimeout(item.timer);
      item.reject(new Error("Page closed"));
    }
    pending.clear();
  };
}
