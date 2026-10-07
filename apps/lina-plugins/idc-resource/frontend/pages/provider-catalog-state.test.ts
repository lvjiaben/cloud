import { describe, expect, it, vi } from "vitest";
import { createProviderCatalogState, providerCatalogPriceRows } from "./provider-catalog-state";
import type { ProviderCatalogDetail, ProviderCatalogPage } from "./provider-catalog-state";

const product = (reference: string) => ({ reference, name: `Product ${reference}`, description: "Supplier description" });
function detail(reference: string): ProviderCatalogDetail {
  const price = { currencyCode: "CNY", period: "month" as const, periodCount: 1, amount: "92233720368547758.07", setupFee: "0.005" };
  return { product: product(reference), basePrices: [price], options: [{ reference: "group-private", name: "Memory", choices: [{ reference: "choice-private", name: "8 GB", prices: [{ ...price, amount: "12.345" }] }] }], priceScheduleComplete: false };
}

describe("upstream catalog read state", () => {
  it("uses server pages and exact totals without fetching row details", async () => {
    const catalog = createProviderCatalogState(vi.fn());
    const load = vi.fn(async () => ({ items: [product("41")], total: 121 }));
    await catalog.query("interface-a", { currentPage: 3, pageSize: 20 }, "", load);
    expect(load).toHaveBeenCalledExactlyOnceWith("interface-a", { page: 3, pageSize: 20, keyword: "" });
    expect(catalog.current).toMatchObject({ currentPage: 3, total: 121, items: [product("41")] });
    expect(catalog.current.detail).toBeUndefined();
    await catalog.query("interface-a", { currentPage: 99, pageSize: 20 }, "", async () => ({ items: [], total: 121 }));
    expect(catalog.current).toMatchObject({ currentPage: 99, total: 121, items: [] });
  });

  it("resets page after a size or search change", async () => {
    const catalog = createProviderCatalogState(vi.fn());
    const load = vi.fn(async () => ({ items: [], total: 121 }));
    await catalog.query("a", { currentPage: 3, pageSize: 20 }, "", load);
    await catalog.query("a", { currentPage: 3, pageSize: 50 }, "", load);
    expect(load).toHaveBeenLastCalledWith("a", { page: 1, pageSize: 50, keyword: "" });
    await catalog.query("a", { currentPage: 3, pageSize: 50 }, " Cloud ", load);
    expect(load).toHaveBeenLastCalledWith("a", { page: 1, pageSize: 50, keyword: "Cloud" });
  });

  it.each([new Error("network"), { response: { status: 403 } }])("clears rows, totals and selected prices on read failure", async (failure) => {
    const catalog = createProviderCatalogState(vi.fn());
    await catalog.query("a", { currentPage: 1, pageSize: 20 }, "", async () => ({ items: [product("1")], total: 1 }));
    await catalog.select("1", async () => ({ item: detail("1") }));
    expect(catalog.current.detail?.product.reference).toBe("1");
    await catalog.query("a", { currentPage: 1, pageSize: 20 }, "", async () => { throw failure; });
    expect(catalog.current).toMatchObject({ items: [], total: 0, detail: undefined, listFailed: true, listLoading: false });
  });

  it("ignores an old interface's page and selected-price response", async () => {
    const catalog = createProviderCatalogState(vi.fn());
    let finishPage!: (page: ProviderCatalogPage) => void;
    let finishDetail!: (result: { item: ProviderCatalogDetail }) => void;
    const oldPage = catalog.query("a", { currentPage: 1, pageSize: 20 }, "", () => new Promise(resolve => { finishPage = resolve; }));
    const oldDetail = catalog.select("old", () => new Promise(resolve => { finishDetail = resolve; }));
    await catalog.query("b", { currentPage: 1, pageSize: 20 }, "", async () => ({ items: [product("new")], total: 21 }));
    finishPage({ items: [product("old")], total: 99 });
    finishDetail({ item: detail("old") });
    await Promise.all([oldPage, oldDetail]);
    expect(catalog.current).toMatchObject({ interfaceNo: "b", items: [product("new")], total: 21, detail: undefined, listLoading: false, detailLoading: false });
  });

  it("does not restore an old selection after a newer selection fails", async () => {
    const catalog = createProviderCatalogState(vi.fn());
    catalog.reset("a");
    let finish!: (result: { item: ProviderCatalogDetail }) => void;
    const old = catalog.select("1", () => new Promise(resolve => { finish = resolve; }));
    await catalog.select("2", async () => { throw new Error("not available"); });
    finish({ item: detail("1") });
    await old;
    expect(catalog.current).toMatchObject({ detail: undefined, detailFailed: true, detailLoading: false });
    await catalog.select("2", async () => ({ item: detail("2") }));
    expect(catalog.current.detail?.product.reference).toBe("2");
  });

  it("rejects mismatched products and invalid pagination rather than inventing counts", async () => {
    const catalog = createProviderCatalogState(vi.fn());
    await catalog.query("a", { currentPage: 1, pageSize: 20 }, "", async () => ({ items: [product("1")], total: 0 }));
    expect(catalog.current).toMatchObject({ items: [], total: 0, listFailed: true });
    await catalog.select("1", async () => ({ item: detail("2") }));
    expect(catalog.current).toMatchObject({ detail: undefined, detailFailed: true });
  });

  it("keeps base prices and option surcharges separate with exact original decimals", () => {
    const result = providerCatalogPriceRows(detail("1"));
    expect(result.base[0]).toMatchObject({ amount: "92233720368547758.07", setupFee: "0.005", currencyCode: "CNY" });
    expect(result.options[0]).toMatchObject({ groupName: "Memory", choiceName: "8 GB", amount: "12.345" });
    expect(result.options[0]).not.toHaveProperty("reference");
    expect(providerCatalogPriceRows()).toEqual({ base: [], options: [] });
  });

  it("invalidates pending work on drawer close", async () => {
    const changed = vi.fn();
    const catalog = createProviderCatalogState(changed);
    let finish!: (page: ProviderCatalogPage) => void;
    const pending = catalog.query("a", { currentPage: 1, pageSize: 20 }, "", () => new Promise(resolve => { finish = resolve; }));
    catalog.reset();
    changed.mockClear();
    finish({ items: [product("late")], total: 1 });
    await pending;
    expect(changed).not.toHaveBeenCalled();
    expect(catalog.current.items).toEqual([]);
  });
});
