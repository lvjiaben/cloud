// @vitest-environment happy-dom
import { beforeEach, expect, it, vi } from "vitest";
const state = vi.hoisted(() => ({ drawer: undefined as any, table: vi.fn() }));
vi.mock("@linapro/vben-plugin-adapter/isolated", () => ({
  mountIsolatedVbenTable: (_node: unknown, options: unknown) => {
    state.table(options);
    return { ready: Promise.resolve(), update: state.table, destroy: vi.fn() };
  },
  mountIsolatedVbenFormDrawer: (_node: unknown, options: unknown) => {
    state.drawer = options;
    return { ready: Promise.resolve(), update: vi.fn(), destroy: vi.fn() };
  },
}));
import { startMigrationPage } from "./page";
beforeEach(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  state.drawer = undefined;
  state.table.mockClear();
});
it("keeps import gated by real preflight and uses the declared bridge without host credentials", async () => {
  const postMessage = vi.fn();
  Object.defineProperty(window, "parent", {
    configurable: true,
    value: { postMessage },
  });
  const stop = startMigrationPage();
  const create = document.querySelector<HTMLButtonElement>(
    '[data-action="createBatch"]',
  )!;
  expect(create.disabled).toBe(true);
  document
    .querySelector<HTMLButtonElement>('[data-action="preflight"]')!
    .click();
  const promise = state.drawer.onSubmit({
    sourceSystem: "zjmf-finance",
    sourceVersion: "v10",
    charset: "UTF-8",
    timezone: "Asia/Shanghai",
    currencies: "CNY",
  });
  const request = postMessage.mock.calls
    .map(([value]) => value)
    .find((value) => value.capability === "idc-migration.admin.v1");
  expect(request.payload).toEqual({
    method: "POST",
    path: "admin/migration/preflight",
    body: {
      sourceSystem: "zjmf-finance",
      sourceVersion: "v10",
      charset: "UTF-8",
      timezone: "Asia/Shanghai",
      currencies: ["CNY"],
    },
  });
  window.dispatchEvent(
    new MessageEvent("message", {
      source: window.parent,
      data: {
        protocol: "linapro.plugin-page-bridge",
        direction: "host-to-plugin",
        pluginId: "idc-zjmfmigrate",
        surfaceKey: "migration-checks",
        requestId: request.requestId,
        ok: true,
        result: {
          supported: true,
          blockers: [],
          warnings: [],
          capabilities: [],
        },
      },
    }),
  );
  expect(await promise).toBe(true);
  expect(create.disabled).toBe(false);
  expect(document.body.textContent).toContain("迁移条件检查通过");
  stop();
});
