import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PluginHostMountContext } from "../src/host-context";

const api = vi.hoisted(() => ({
  accounts: vi.fn(), application: vi.fn(), applications: vi.fn(),
  evidence: vi.fn(), review: vi.fn(), statements: vi.fn(),
}));
vi.mock("../src/finance/api", () => ({ createFinanceAPI: () => api }));
vi.mock("vue", async (original) => ({
  ...(await original<typeof import("vue")>()), onMounted: vi.fn(),
}));

import { PluginRequestError } from "../src/runtime/request";
import { useCreditWorkspace } from "../src/finance/use-credit-workspace";

const application = {
  applicationNo: "application-one", createdAt: 1, currencyCode: "CNY",
  customerNo: "customer-one", requestedLimitMinor: "10000",
  requestedValidFrom: 1, requestedValidUntil: 2, requiredApprovalCount: 2,
  reviewCount: 0, statusCode: "pending_review",
};
function workspace() {
  return useCreditWorkspace({ context: {
    accessToken: "", assetURL: "", baseURL: "", container: {} as HTMLElement,
    locale: "zh-CN", messages: {}, query: {}, route: {}, routePath: "", title: "",
    t: (key, fallback) => fallback ?? key,
  } satisfies PluginHostMountContext });
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

beforeEach(() => {
  vi.resetAllMocks();
  api.applications.mockResolvedValue({ applications: [application], total: 123 });
  api.accounts.mockResolvedValue({ accounts: [{ creditAccountNo: "account-one", unbilledPrincipalMinor: "0", billedPrincipalMinor: "0" }], total: 123 });
  api.statements.mockResolvedValue({ statements: [{ statementNo: "statement-one" }], total: 123 });
  api.application.mockResolvedValue({ detail: { application }, evidence: [{ uploadNo: "private-evidence", fileName: "bank.pdf" }] });
});
afterEach(() => vi.unstubAllGlobals());

describe("credit authorization invalidation", () => {
  it.each([401, 403])("evidence downloads preserve HTTP %s for scoped invalidation", async (status) => {
    const { createFinanceAPI } = await vi.importActual<typeof import("../src/finance/api")>("../src/finance/api");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status }));
    const evidenceAPI = createFinanceAPI(() => ({ accessToken: "" } as PluginHostMountContext));
    await expect(evidenceAPI.evidence("application-one", {
      uploadNo: "private", fileName: "bank.pdf", mediaType: "application/pdf", sizeBytes: 1,
    })).rejects.toMatchObject({ status, name: "PluginRequestError" });
  });
  it("changing page size resets to page one without changing direct page navigation", async () => {
    const state = workspace();
    await state.loadPage("applications", 8, 10);
    expect(state.pageState.applications.currentPage).toBe(8);
    await state.loadPage("applications", 4, 20);
    expect(api.applications).toHaveBeenLastCalledWith(1, 20);
    expect(state.pageState.applications.currentPage).toBe(1);
  });
  it.each(["applications", "accounts", "statements"] as const)("403 clears only the %s directory and its exact total", async (kind) => {
    const state = workspace();
    await state.load();
    await state.openDetail(application);
    state.openReview(application);
    state.reviewForm.comment = "private comment";
    state.reviewForm.evidence = "private evidence";
    api[kind].mockRejectedValueOnce(new PluginRequestError(403));
    await state.loadPage(kind, 2, 10);
    expect(state[kind].value).toEqual([]);
    expect(state.pageState[kind]).toEqual({ currentPage: 1, pageSize: 10, total: 0 });
    for (const other of ["applications", "accounts", "statements"] as const) {
      if (other !== kind) expect(state.pageState[other].total).toBe(123);
    }
    if (kind === "applications") {
      expect(state.selected.value).toBeNull();
      expect(state.evidence.value).toEqual([]);
      expect(state.detailOpen.value || state.reviewOpen.value).toBe(false);
      expect(state.reviewForm.comment + state.reviewForm.evidence).toBe("");
    }
  });

  it("503 preserves the confirmed page and details, and retries the failed page", async () => {
    const state = workspace();
    await state.loadPage("applications", 2, 10);
    await state.openDetail(application);
    api.applications.mockRejectedValueOnce(new PluginRequestError(503));
    await state.loadPage("applications", 3, 10);
    expect(state.applications.value).toEqual([application]);
    expect(state.pageState.applications).toEqual({ currentPage: 2, pageSize: 10, total: 123 });
    expect(state.detailOpen.value).toBe(true);
    await state.load();
    expect(api.applications).toHaveBeenLastCalledWith(3, 10);
    expect(state.pageState.applications.currentPage).toBe(3);
  });

  it("401 clears every directory and prevents older list/detail responses restoring data", async () => {
    const state = workspace();
    await state.load();
    const oldAccount = deferred<{ accounts: unknown[]; total: number }>();
    const oldDetail = deferred<unknown>();
    api.accounts.mockReturnValueOnce(oldAccount.promise);
    api.application.mockReturnValueOnce(oldDetail.promise);
    const accountRequest = state.loadPage("accounts", 2, 10);
    const detailRequest = state.openDetail(application);
    api.statements.mockRejectedValueOnce(new PluginRequestError(401));
    await state.loadPage("statements", 2, 10);
    oldAccount.resolve({ accounts: [{ creditAccountNo: "late-account" }], total: 999 });
    oldDetail.resolve({ detail: { application }, evidence: [{ uploadNo: "late-private-evidence" }] });
    await Promise.all([accountRequest, detailRequest]);
    for (const kind of ["applications", "accounts", "statements"] as const) {
      expect(state[kind].value).toEqual([]);
      expect(state.pageState[kind].total).toBe(0);
      expect(state.pageLoading[kind]).toBe(false);
    }
    expect(state.selected.value).toBeNull();
    expect(state.evidence.value).toEqual([]);
    expect(state.detailOpen.value).toBe(false);
  });

  it("a denied detail clears stored evidence but keeps the still-authorized list", async () => {
    const state = workspace();
    await state.load();
    await state.openDetail(application);
    api.application.mockRejectedValueOnce(new PluginRequestError(403));
    await state.openDetail(application);
    expect(state.selected.value).toBeNull();
    expect(state.evidence.value).toEqual([]);
    expect(state.applications.value).toEqual([application]);
    expect(state.pageState.applications.total).toBe(123);
  });

  it("a late evidence response is discarded after its directory loses access", async () => {
    const state = workspace();
    await state.load();
    await state.openDetail(application);
    const oldEvidence = deferred<Blob>();
    api.evidence.mockReturnValueOnce(oldEvidence.promise);
    const download = state.openEvidence({ uploadNo: "private", fileName: "bank.pdf", mediaType: "application/pdf", sizeBytes: 1 });
    api.applications.mockRejectedValueOnce(new PluginRequestError(403));
    await state.loadPage("applications", 2, 10);
    // Node has no document: an attempted stale download would fail this test.
    oldEvidence.resolve(new Blob(["private"]));
    await download;
    expect(state.notice.value).toBe("");
    expect(state.evidence.value).toEqual([]);
  });
});
