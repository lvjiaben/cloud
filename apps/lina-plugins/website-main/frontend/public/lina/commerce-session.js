// Website-local account session bridge; credentials remain in memory.
import {loadCustomerConfiguration} from './customer-config.js';
import {getLocale} from './locale.js';
import {t} from './commerce-copy.js';
let token='',pendingSession;
export const getAccessToken=()=>token;
export function customerURL(path){
 if(typeof path!=='string'||!/^\/(?!\/)/u.test(path)||/[\\\s\u0000-\u001f]/u.test(path)||/%(?:2f|5c|0[0-9a-f])/iu.test(path))return '';
 try{const base=new URL(window.__LINA_TEMPLATE__?.customerUrl);if(!['https:','http:'].includes(base.protocol)||base.username||base.password)return '';return new URL(path,base).href;}catch{return '';}
}
export async function syncSession() {
  // Reuse the same-origin protected session until the backend rejects it.
  if (token) return true;
  if (pendingSession) return pendingSession;
  pendingSession = (async () => {
    await loadCustomerConfiguration();
    const response = await fetch("/__website/session", {
      credentials: "same-origin",
      signal: AbortSignal.timeout(10000),
      redirect: "error",
    });
    if (response.ok) {
      const data = await response.json();
      if (data.checked && typeof data.accessToken === "string" && data.accessToken) {
        token = data.accessToken;
        return true;
      }
    }
    return new Promise((resolve, reject) => {
      const target = customerURL("/auth/website-session");
      if (!target) {
        reject(
          new Error(
            t("Customer center is not configured. Contact the administrator."),
          ),
        );
        return;
      }
      const origin = new URL(target).origin,
        nonce = crypto.randomUUID(),
        frame = document.createElement("iframe");
      frame.hidden = true;
      frame.title = t("Customer session");
      frame.referrerPolicy = "no-referrer";
      const cleanup = () => {
        clearTimeout(timer);
        window.removeEventListener("message", receive);
        frame.remove();
      };
      const receive = async (event) => {
        if (
          event.origin !== origin ||
          event.source !== frame.contentWindow ||
          event.data?.type !== "lina-website-session" ||
          event.data?.nonce !== nonce
        )
          return;
        token =
          typeof event.data.accessToken === "string"
            ? event.data.accessToken
            : "";
        cleanup();
        if(token){try{const saved=await fetch('/__website/session',{method:'PUT',credentials:'same-origin',headers:{Authorization:'Bearer '+token},redirect:'error',signal:AbortSignal.timeout(10000)});if(!saved.ok)throw new Error(t('Could not read your sign-in status. Please retry.'));}catch(error){token='';reject(error);return;}}
        resolve(Boolean(token));
      };
      const timer = setTimeout(() => {
        cleanup();
        reject(
          new Error(t("Could not read your sign-in status. Please retry.")),
        );
      }, 10000);
      window.addEventListener("message", receive);
      frame.src =
        target +
        "?" +
        new URLSearchParams({ nonce, websiteOrigin: location.origin });
      document.body.append(frame);
    });
  })()
    .catch((error) => {
      token = "";
      throw error;
    })
    .finally(() => {
      pendingSession = undefined;
    });
  return pendingSession;
}
export async function sessionIdentity(){return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(token)))).map(v=>v.toString(16).padStart(2,'0')).join('');}
export async function rememberCart(cart){try{if(token&&cart?.version)sessionStorage.setItem('website-main-cart-response',JSON.stringify({identity:await sessionIdentity(),savedAt:Date.now(),cart}));}catch{}}
export async function takeCart(){try{const raw=sessionStorage.getItem('website-main-cart-response');sessionStorage.removeItem('website-main-cart-response');const value=JSON.parse(raw||'null');if(token&&value?.identity===await sessionIdentity()&&Date.now()>=value.savedAt&&Date.now()-value.savedAt<15000)return value.cart;}catch{}return null;}
export async function signIn() {
  if (await syncSession()) return true;
  const target = customerURL("/auth/website-session");
  if (!target) throw new Error(t("Customer center is not configured. Contact the administrator."));
  location.assign(
    target +
      "?" +
      new URLSearchParams({
        login: "1",
        returnTo: location.pathname + location.search,
        websiteOrigin: location.origin,
      }),
  );
  return false;
}
export async function commerce(
  path,
  method = "GET",
  body,
  signal,
  owner = "idc-core",
) {
  const binary = body instanceof Blob;
  const paymentChannels =
    owner === "idc-payment" &&
    ((method === "GET" && /^account\/payment-channels\?currencyCode=[A-Z]{3}$/u.test(path)) || (method === "POST" && path === 'account/payments') || (method === "GET" && /^account\/payments\/[A-Za-z0-9_-]{1,40}$/u.test(path)));
  const core =
    owner === "idc-core" &&
    (/^(public|account)\/(commerce|catalog)\//u.test(path) ||
      /^account\/marketing\/coupons(?:\/claims)?$/u.test(path) ||
      (method === "GET" && path === "account/billing/overview") ||
      (method === "GET" && /^account\/billing\/invoices\/[A-Za-z0-9_-]{1,40}$/u.test(path)) ||
      (method === "POST" && /^account\/billing\/invoices\/[A-Za-z0-9_-]{1,40}\/balance-payments$/u.test(path)));
  if (!core && !paymentChannels) throw new Error("Invalid commerce path");
  const account = path.startsWith("account/");
  if (account && !token) throw new Error(t("Sign in before checking out"));
  const usedToken=token;
  const response = await fetch(`/x/${owner}/api/v1/` + path, {
    method,
    credentials: "same-origin",
    redirect: "error",
    headers: {
      Accept: "application/json",
      "Accept-Language": getLocale(),
      ...(body === undefined
        ? {}
        : {
            "Content-Type": binary
              ? "application/octet-stream"
              : "application/json",
          }),
      ...(account ? { Authorization: "Bearer " + token } : {}),
    },
    body: body === undefined ? undefined : binary ? body : JSON.stringify(body),
    signal: signal
      ? AbortSignal.any([signal, AbortSignal.timeout(12000)])
      : AbortSignal.timeout(12000),
  });
  async function expired(){
    if(account&&token===usedToken){token="";try{sessionStorage.removeItem("website-main-cart-response");}catch{}try{await fetch("/__website/session",{method:"DELETE",credentials:"same-origin",redirect:"error",signal:AbortSignal.timeout(3000)});}catch{}}
    throw new Error(t("Your session expired. Sign in again."));
  }
  if (response.status === 401) return expired();
  const payload = await response.json();
  if(String(payload.code)==="401")return expired();
  if (!response.ok || String(payload.code ?? 0) !== "0" || !payload.data)
    throw new Error(t("Could not complete the operation. Refresh and retry."));
  return payload.data;
}
