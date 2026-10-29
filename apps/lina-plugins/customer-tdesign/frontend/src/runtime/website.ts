import { externalURL, getSession, requestClient, safePath } from "#/api/request";
export type WebsiteSettings = {
  websiteurl?: string;
  websitecatalogpath?: string;
  websiteconfigurepath?: string;
  websitecartpath?: string;
};
export type WebsiteDestination = "catalog" | "configure" | "cart";
declare global {
  interface Window { __CUSTOMER_WEBSITE__?: WebsiteSettings }
}
export async function websiteSettings(): Promise<WebsiteSettings> {
  // Preview adapters may inject public deployment settings; never read a URL query as configuration.
  if (window.__CUSTOMER_WEBSITE__) return window.__CUSTOMER_WEBSITE__;
  const result = await requestClient.get<{ selectedPlugin?: string; config?: WebsiteSettings }>("/x/idc-portal/api/v1/public/customer-center/configuration");
  if (result.selectedPlugin !== "customer-tdesign") throw new Error("官网入口尚未配置，请联系商家。");
  return result.config || {};
}
export function websiteOrigin(settings: WebsiteSettings) {
  const value = externalURL(settings.websiteurl || "");
  if (!value) throw new Error("官网入口尚未配置，请联系商家。");
  const url = new URL(value);
  if (url.pathname !== "/" || url.search || url.hash) throw new Error("官网地址配置不正确，请联系商家。");
  return url.origin;
}
export function websiteTarget(settings: WebsiteSettings, kind: WebsiteDestination, product = "", search = "") {
  const origin = websiteOrigin(settings);
  let path = kind === "catalog" ? settings.websitecatalogpath : kind === "cart" ? settings.websitecartpath : settings.websiteconfigurepath;
  if (kind === "configure") {
    if (!product || !path?.includes("{product}")) throw new Error("商品选购入口尚未配置，请联系商家。");
    path = path.replaceAll("{product}", encodeURIComponent(product));
  }
  if (!path || !safePath(path)) throw new Error("官网选购入口尚未配置，请联系商家。");
  const target = new URL(path, origin);
  if (target.origin !== origin) throw new Error("官网选购入口配置不正确。");
  const params = new URLSearchParams(search);
  for (const key of ["currency", "quoteItemNo"]) {
    const value = params.get(key);
    if (value) target.searchParams.set(key, value);
  }
  return target;
}
export function allowedWebsiteReturn(settings: WebsiteSettings, path: string) {
  if (!safePath(path)) throw new Error("返回地址不可用。");
  const target = new URL(path, websiteOrigin(settings));
  const allowed = [settings.websitecatalogpath, settings.websitecartpath, settings.websiteconfigurepath].filter((v): v is string => !!v);
  const valid = allowed.some(value => {
    const template = new URL(value.replaceAll("{product}", "PRODUCT_PLACEHOLDER"), target.origin);
    const pattern = template.pathname.split("PRODUCT_PLACEHOLDER").map(v => v.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("[^/]+");
    return new RegExp(`^${pattern}$`).test(target.pathname);
  });
  if (target.origin !== websiteOrigin(settings) || !valid) throw new Error("返回地址不可用。");
  return target;
}
export async function currentWebsiteSession() {
  if (!getSession()) return "";
  await requestClient.get("/user/info");
  const gate = await requestClient.get<{required:boolean}>("/x/idc-core/api/v1/account/initial-password/status");
  if (gate.required) throw new Error("请先设置登录密码，再返回官网。");
  return getSession()?.accessToken || "";
}
export function postWebsiteSession(target: URL, accessToken: string) {
  const form = document.createElement("form");
  form.method = "POST";
  form.action = new URL("/__website/session", target.origin).href;
  for (const [name, value] of Object.entries({accessToken, returnTo:target.pathname + target.search})) {
    const input = document.createElement("input");
    input.type = "hidden";
    input.name = name;
    input.value = value;
    form.append(input);
  }
  document.body.append(form);
  form.submit();
}
