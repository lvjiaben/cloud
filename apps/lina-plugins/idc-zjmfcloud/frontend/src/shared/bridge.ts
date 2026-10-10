const protocol = "linapro.plugin-page-bridge";
const version = "1.0";
const pluginId = "idc-zjmfcloud";

export interface BridgeResponse {
  ok?: unknown;
  requestId?: unknown;
  result?: unknown;
}

export function createBridge(
  win: Window,
  surfaceKey: string,
  capability: string,
) {
  let sequence = 0;
  const pending = new Map<string, (message: BridgeResponse) => void>();
  const send = (
    sentCapability: string,
    method: string,
    payload: unknown,
    requestId?: string,
  ) => {
    win.parent.postMessage(
      {
        protocol,
        version,
        direction: "plugin-to-host",
        pluginId,
        surfaceKey,
        capability: sentCapability,
        method,
        payload,
        requestId,
      },
      "*",
    );
  };
  const onMessage = (event: MessageEvent<Record<string, unknown>>) => {
    const message = event.data;
    if (
      event.source !== win.parent ||
      message?.protocol !== protocol ||
      message.direction !== "host-to-plugin" ||
      message.pluginId !== pluginId ||
      message.surfaceKey !== surfaceKey
    )
      return;
    const requestId =
      typeof message.requestId === "string" ? message.requestId : "";
    if (requestId && pending.has(requestId)) {
      pending.get(requestId)?.(message);
      pending.delete(requestId);
    }
  };
  win.addEventListener("message", onMessage);
  return {
    capability,
    dispose: () => {
      pending.forEach((resolve) => resolve({ ok: false }));
      pending.clear();
      win.removeEventListener("message", onMessage);
    },
    ready: () => send("host.view.v1", "ready", {}),
    request(method: string, path: string, payload: Record<string, unknown>) {
      sequence += 1;
      const requestId = `idc-zjmfcloud-${sequence}`;
      return new Promise<BridgeResponse>((resolve) => {
        const timeout = win.setTimeout(() => {
          pending.delete(requestId);
          resolve({ ok: false });
        }, 20000);
        pending.set(requestId, (response) => {
          win.clearTimeout(timeout);
          resolve(response);
        });
        send(capability, "request", { method, path, ...payload }, requestId);
      });
    },
    resize: (height: number) => send("host.view.v1", "resize", { height }),
    setTitle: (title: string) => send("host.view.v1", "set-title", { title }),
    subscribeContext(handler: (payload: Record<string, unknown>) => void) {
      const listener = (event: MessageEvent<Record<string, unknown>>) => {
        const message = event.data;
        if (
          event.source === win.parent &&
          message?.protocol === protocol &&
          message.direction === "host-to-plugin" &&
          message.pluginId === pluginId &&
          message.surfaceKey === surfaceKey &&
          message.capability === "host.context.v1"
        ) {
          handler((message.payload as Record<string, unknown>) ?? {});
        }
      };
      win.addEventListener("message", listener);
      return () => win.removeEventListener("message", listener);
    },
  };
}
