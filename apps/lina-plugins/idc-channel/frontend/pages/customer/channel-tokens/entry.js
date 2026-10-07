const protocol = "linapro.plugin-page-bridge";
const version = "1.0";
const pluginId = "idc-channel";
const surfaceKey = "channel-tokens";
let sequence = 0;
const pending = new Map();
let context = { locale: "zh-CN" };
let contextReady;
const contextReceived = new Promise((resolve) => { contextReady = resolve; });

function send(capability, method, payload, requestId) {
  window.parent.postMessage({ protocol, version, direction: "plugin-to-host", pluginId, surfaceKey, capability, method, payload, requestId }, "*");
}
function valid(event) {
  const message = event.data;
  return event.source === window.parent && message?.protocol === protocol && message.version === version && message.direction === "host-to-plugin" && message.pluginId === pluginId && message.surfaceKey === surfaceKey;
}
window.addEventListener("message", (event) => {
  if (!valid(event)) return;
  if (event.data.capability === "host.context.v1") {
    const payload = event.data.payload && typeof event.data.payload === "object" ? event.data.payload : {};
    context = { ...context, ...payload };
    contextReady?.();
    return;
  }
  const requestId = typeof event.data.requestId === "string" ? event.data.requestId : "";
  const handler = pending.get(requestId);
  if (!handler) return;
  pending.delete(requestId);
  if (event.data.ok === true) handler.resolve(event.data.result);
  else handler.reject(new Error(event.data.error?.message || event.data.error?.code || "bridge_request_failed"));
});
function request(path, options = {}) {
  const requestId = `channel-account-${++sequence}`;
  send("channel.account.v1", "request", { method: String(options.method || "GET").toUpperCase(), path, body: options.body }, requestId);
  return new Promise((resolve, reject) => pending.set(requestId, { resolve, reject }));
}
send("host.view.v1", "ready", {});
await Promise.race([contextReceived, new Promise((resolve) => setTimeout(resolve, 1000))]);
const module = await import("./mount.js");
const mounted = module.mount({ ...context, container: document.body, request });
const resize = () => send("host.view.v1", "resize", { height: Math.ceil(document.documentElement.scrollHeight) });
new ResizeObserver(resize).observe(document.body);
resize();
window.addEventListener("beforeunload", () => mounted?.unmount?.());
