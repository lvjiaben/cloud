// @vitest-environment happy-dom
import { beforeEach, expect, it, vi } from "vitest";
const runtime = vi.hoisted(() => ({ update: vi.fn(), destroy: vi.fn() }));
vi.mock("@linapro/vben-plugin-adapter/isolated", () => ({
  mountIsolatedVbenTable: () => ({ ready: Promise.resolve(), ...runtime }),
  mountIsolatedVbenFormDrawer: () => ({
    ready: Promise.resolve(),
    destroy: vi.fn(),
  }),
}));
import { startAdminServersPage } from "./page";
beforeEach(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  runtime.update.mockClear();
});
it("loads the exact read-only owner route and accepts bounded authoritative totals", async () => {
  const postMessage = vi.fn();
  Object.defineProperty(window, "parent", {
    configurable: true,
    value: { postMessage },
  });
  const stop = startAdminServersPage();
  const message = (data: Record<string, unknown>) =>
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window.parent,
        data: {
          protocol: "linapro.plugin-page-bridge",
          direction: "host-to-plugin",
          pluginId: "idc-zjmfcloud",
          surfaceKey: "server-management",
          ...data,
        },
      }),
    );
  message({ capability: "host.context.v1", payload: { locale: "zh-CN" } });
  const request = postMessage.mock.calls
    .map(([item]) => item)
    .find((item) => item.capability === "zjmfcloud-servers.admin.v1");
  expect(request.payload).toEqual({
    method: "GET",
    path: "admin/resource/zjmfcloud-servers",
    params: { page: 1, limit: 20 },
  });
  message({
    requestId: request.requestId,
    ok: true,
    result: { items: [], total: 0, nextCursor: "" },
  });
  await Promise.resolve();
  await Promise.resolve();
  expect(runtime.update.mock.calls.at(-1)?.[0].pagination.total).toBe(0);
  expect(document.body.textContent).toContain("魔方云服务器管理");
  stop();
});
