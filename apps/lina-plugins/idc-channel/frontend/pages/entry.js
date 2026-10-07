const protocol = "linapro.plugin-page-bridge";
const version = "1.0";
const pluginId = "idc-channel";
const surfaceKey = "channel";

let sequence = 0;
const pending = new Map();
let contextReady;
const contextReceived = new Promise((resolve) => { contextReady = resolve; });
const translations = {
  "zh-CN": {
    quotaTitle: "额度与结算",
    quotaHint: "逐币种展示余额、信用、应收、实收、退款和服务成本。",
    healthTitle: "渠道健康",
    healthHint: "查看渠道状态、在售商品、有效令牌和待投递事件。",
    auditTitle: "请求审计",
    auditHint: "查看渠道请求结果和完成时间，不记录令牌原文。",
    settlementTitle: "结算",
    settlementHint: "渠道企业承担付款、信用和结算，受委托客户只承担服务归属与售后身份。",
    customerTokenTitle: "客户令牌自助管理",
    customerTokenHint: "仅商家已批准且完成实名的企业渠道可创建、查看和撤销自己的令牌。",
  },
  "en-US": {
    quotaTitle: "Finance and settlement",
    quotaHint: "View balances, credit, receivables, payments, refunds, and service costs by currency.",
    healthTitle: "Channel health",
    healthHint: "View channel status, published products, active tokens, and pending webhook deliveries.",
    auditTitle: "Request audit",
    auditHint: "View channel request outcomes and completion times without storing token secrets.",
    settlementTitle: "Settlement",
    settlementHint: "The channel enterprise pays and settles; delegated customers own services and support identity only.",
    customerTokenTitle: "Customer self-service tokens",
    customerTokenHint: "Only merchant-approved, identity-verified enterprise channels can create, inspect, and revoke their own tokens.",
  },
};
const translate = (key, fallback) => {
  const shortKey = String(key).split(".").at(-1) || String(key);
  return translations[context.locale]?.[shortKey] || fallback;
};
let context = { locale: "zh-CN", t: translate };

function send(capability, method, payload, requestId) {
  window.parent.postMessage({ protocol, version, direction: "plugin-to-host", pluginId, surfaceKey, capability, method, payload, requestId }, "*");
}

function isHostMessage(event) {
  const message = event.data;
  return event.source === window.parent && message?.protocol === protocol && message.version === version && message.direction === "host-to-plugin" && message.pluginId === pluginId && message.surfaceKey === surfaceKey;
}

window.addEventListener("message", (event) => {
  if (!isHostMessage(event)) return;
  const message = event.data;
  if (message.capability === "host.context.v1") {
    const payload = message.payload && typeof message.payload === "object" ? message.payload : {};
    context = { ...context, ...payload, t: translate };
    contextReady?.();
    return;
  }
  const requestId = typeof message.requestId === "string" ? message.requestId : "";
  const resolve = pending.get(requestId);
  if (!resolve) return;
  pending.delete(requestId);
  resolve(message.ok === true ? message.result : Promise.reject(new Error(message.error?.code || "bridge_request_failed")));
});

function request(path, options = {}) {
  sequence += 1;
  const requestId = `channel-${sequence}`;
  send("channel.admin.v1", "request", { method: String(options.method || "GET").toUpperCase(), path, params: options.params, body: options.body }, requestId);
  return new Promise((resolve, reject) => {
    pending.set(requestId, (value) => {
      if (value && typeof value.then === "function") value.then(resolve, reject);
      else resolve(value);
    });
  });
}

send("host.view.v1", "ready", {});
await Promise.race([contextReceived, new Promise((resolve) => setTimeout(resolve, 1000))]);
const mountModule = await import("./admin-mount.js");
const mounted = mountModule.mount({
  container: document.body,
  locale: context.locale,
  request,
  t: context.t,
});

const resize = () => send("host.view.v1", "resize", { height: Math.ceil(document.documentElement.scrollHeight) });
new ResizeObserver(resize).observe(document.body);
resize();
window.addEventListener("beforeunload", () => mounted?.unmount?.());
