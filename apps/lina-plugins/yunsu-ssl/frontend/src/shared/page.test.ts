// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { startCertificatePage } from "./page";
import { installCertificateShell } from "./shell";

// Unit tests exercise the certificate bridge contract; the real official grid is
// covered by the plugin's Chromium surface tests and adapter sandbox checks.
const runtime = vi.hoisted(() => ({failNext: false, mounts: 0}));
vi.mock('@linapro/vben-plugin-adapter/isolated', () => ({
  mountIsolatedVbenTable(container: HTMLElement) {
    runtime.mounts += 1;
    const failed = runtime.failNext;
    runtime.failNext = false;
    return {
      ready: failed ? Promise.reject(new Error('runtime unavailable')) : Promise.resolve(),
      update(options: {rows: Record<string,unknown>[];columns: {field:string;cell?:(row:Record<string,unknown>)=>Node|string|Array<Node|string>}[]}) {
        container.replaceChildren(...options.rows.map(row => {
          const record = document.createElement('div');
          for (const column of options.columns) {
            const value = column.cell?.(row) ?? String(row[column.field] ?? '');
            record.append(...(Array.isArray(value) ? value : [value]));
          }
          return record;
        }));
      },
      destroy() { container.replaceChildren(); },
    };
  },
}));

const certificate = {
  availableActions: ["sync", "update-validation", "revalidate", "cancel"], entitlementNo: "ENT_SSL_00000001", customerName: "示例客户",
  expiresAt: 1_810_000_000_000, issuedAt: 0, primaryDomain: "example.test", productNameEnUs: "SSL Certificate", productNameZhCn: "SSL 证书",
  statusCode: "awaiting_validation", updatedAt: 1_778_000_100_000,
  validation: [{ domain: "example.test", host: "_dnsauth.example.test", method: "dns", recordType: "CNAME", value: "validation.example.test" }], validationMethod: "dns",
};

function hostMessage(payload: Record<string, unknown>) {
  window.dispatchEvent(new MessageEvent("message", { source: window.parent, data: { protocol: "linapro.plugin-page-bridge", version: "1.0", direction: "host-to-plugin", pluginId: "yunsu-ssl", surfaceKey: "certificates", ...payload } }));
}

describe("isolated SSL certificate page", () => {
  let originalParent: PropertyDescriptor | undefined;
  beforeEach(() => {
    originalParent = Object.getOwnPropertyDescriptor(window, 'parent');
    runtime.failNext = false; runtime.mounts = 0;
    document.head.innerHTML = ""; document.body.innerHTML = "";
    vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  });
  afterEach(() => {
    if (originalParent) Object.defineProperty(window, 'parent', originalParent);
    else Reflect.deleteProperty(window, 'parent');
    vi.unstubAllGlobals();
    runtime.failNext = false; runtime.mounts = 0;
  });

  it("remounts a failed runtime using the existing retry without replaying certificate actions", async () => {
    runtime.failNext = true;
    installCertificateShell('customer');
    const postMessage = vi.fn(); Object.defineProperty(window, 'parent', {configurable:true,value:{postMessage}});
    const stop = startCertificatePage({audience:'customer',capability:'service-entitlements.customer.v1',surfaceKey:'certificates'});
    hostMessage({capability:'host.context.v1',payload:{locale:'zh-CN',theme:{mode:'light',cssVariables:{}}}});
    const requests = () => postMessage.mock.calls.map(([message])=>message).filter(message=>message.payload?.path==='account/service-entitlements');
    const first = requests()[0];
    hostMessage({capability:first.capability,requestId:first.requestId,ok:true,result:{items:[certificate],nextCursor:''}});
    await vi.waitFor(()=>expect(document.querySelector<HTMLElement>('[data-testid="ssl-error"]')?.hidden).toBe(false));
    expect(runtime.mounts).toBe(1);
    document.querySelector<HTMLButtonElement>('[data-testid="ssl-retry"]')!.click();
    const retry = requests()[1];
    hostMessage({capability:retry.capability,requestId:retry.requestId,ok:true,result:{items:[certificate],nextCursor:''}});
    await Promise.resolve();
    expect(runtime.mounts).toBe(2);
    expect(document.querySelector<HTMLElement>('[data-testid="ssl-error"]')?.hidden).toBe(true);
    expect(document.querySelector<HTMLElement>('[data-testid="ssl-content"]')?.hidden).toBe(false);
    expect(postMessage.mock.calls.every(([message])=>message.payload?.method!=='POST')).toBe(true);
    stop();
  });

  it("loads once, follows host locale/theme, and does not display internal certificate numbers", async () => {
    installCertificateShell("customer");
    const postMessage = vi.fn(); Object.defineProperty(window, "parent", { configurable: true, value: { postMessage } });
    const stop = startCertificatePage({ audience: "customer", capability: "service-entitlements.customer.v1", surfaceKey: "certificates" });
    hostMessage({ capability: "host.context.v1", payload: { locale: "zh-CN", theme: { mode: "dark", cssVariables: {} } } });
    const request = postMessage.mock.calls.map(([message]) => message).find((message) => message.capability === "service-entitlements.customer.v1");
    expect(request.payload).toMatchObject({ method: "GET", path: "account/service-entitlements" });
    hostMessage({ capability: request.capability, requestId: request.requestId, ok: true, result: { items: [certificate], nextCursor: "" } });
    await Promise.resolve();
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.body.textContent).toContain("example.test");
    expect(document.body.textContent).not.toContain(certificate.entitlementNo);
    hostMessage({ capability: "host.context.v1", payload: { locale: "en-US", theme: { mode: "light", cssVariables: {} } } });
    expect(document.body.textContent).toContain("SSL certificates");
    const requests = postMessage.mock.calls.map(([message]) => message).filter((message) => message.capability === "service-entitlements.customer.v1");
    expect(requests).toHaveLength(1);
    stop();
  });

  it("keeps customer and admin owner paths separate and never posts a host token", () => {
    installCertificateShell("admin");
    const postMessage = vi.fn(); Object.defineProperty(window, "parent", { configurable: true, value: { postMessage } });
    const stop = startCertificatePage({ audience: "admin", capability: "service-entitlements.admin.v1", surfaceKey: "certificates" });
    hostMessage({ capability: "host.context.v1", payload: { locale: "zh-CN", theme: { mode: "light", cssVariables: {} } } });
    const request = postMessage.mock.calls.map(([message]) => message).find((message) => message.capability === "service-entitlements.admin.v1");
    expect(request.payload.path).toBe("admin/service-entitlements");
    expect(JSON.stringify(postMessage.mock.calls)).not.toMatch(/accessToken|authorization|bearer/i);
    stop();
  });

  it("shows only the error state when the initial certificate request fails", async () => {
    installCertificateShell("admin");
    const postMessage = vi.fn(); Object.defineProperty(window, "parent", { configurable: true, value: { postMessage } });
    const stop = startCertificatePage({ audience: "admin", capability: "service-entitlements.admin.v1", surfaceKey: "certificates" });
    hostMessage({ capability: "host.context.v1", payload: { locale: "zh-CN", theme: { mode: "light", cssVariables: {} } } });
    const request = postMessage.mock.calls.map(([message]) => message).find((message) => message.payload?.path === "admin/service-entitlements");
    hostMessage({ capability: request.capability, requestId: request.requestId, ok: false });
    await Promise.resolve();
    expect(document.querySelector<HTMLElement>('[data-testid="ssl-error"]')?.hidden).toBe(false);
    expect(document.querySelector<HTMLElement>('[data-testid="ssl-empty"]')?.hidden).toBe(true);
    stop();
  });

  it("redeems a certificate ticket through the governed host download bridge", async () => {
    installCertificateShell("customer");
    const postMessage = vi.fn(); Object.defineProperty(window, "parent", { configurable: true, value: { postMessage } });
    const stop = startCertificatePage({ audience: "customer", capability: "service-entitlements.customer.v1", surfaceKey: "certificates" });
    hostMessage({ capability: "host.context.v1", payload: { locale: "zh-CN", theme: { mode: "light", cssVariables: {} } } });
    const list = postMessage.mock.calls.map(([message]) => message).find((message) => message.payload?.path === "account/service-entitlements");
    hostMessage({ capability: list.capability, requestId: list.requestId, ok: true, result: { items: [{ ...certificate, availableActions: ["download"], statusCode: "issued" }], nextCursor: "" } });
    await Promise.resolve();
    (document.querySelector('[data-testid="ssl-table"] button:nth-of-type(2)') as HTMLButtonElement).click();
    await Promise.resolve();
    const ticketRequest = postMessage.mock.calls.map(([message]) => message).find((message) => String(message.payload?.path ?? "").includes("/downloads/certificate/tickets"));
    hostMessage({ capability: ticketRequest.capability, requestId: ticketRequest.requestId, ok: true, result: { ticket: "A".repeat(32), fileName: "example.test.zip" } });
    await Promise.resolve();
    const downloadRequest = postMessage.mock.calls.map(([message]) => message).find((message) => String(message.payload?.path ?? "").includes("service-downloads"));
    expect(downloadRequest.payload).toEqual({ method: "GET", path: `account/service-downloads/${"A".repeat(32)}`, download: { fileName: "example.test.zip" } });
    expect(JSON.stringify(downloadRequest)).not.toMatch(/authorization|bearer|accessToken/i);
    stop();
  });
});
