import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IsolatedFormDrawerOptions } from "@linapro/vben-plugin-adapter/isolated";
const mounted = vi.hoisted(() => ({ options: undefined as IsolatedFormDrawerOptions | undefined, destroyed: false }));
vi.mock("@linapro/vben-plugin-adapter/isolated", () => ({ mountIsolatedVbenFormDrawer: (_root: unknown, options: IsolatedFormDrawerOptions) => {
  mounted.options = options;
  return { ready: Promise.resolve(), update: (next: Partial<IsolatedFormDrawerOptions>) => { mounted.options = { ...mounted.options!, ...next }; }, destroy: () => { mounted.destroyed = true; } };
} }));
import { catalogImportInput, createCatalogImporter } from "./provider-catalog-import";

const basic = () => ({ categoryNo: "local", nameZhCn: "商品", nameEnUs: "Product", pricingMode: "markup", markupPercent: "10" });
const manual = () => ({ ...basic(), pricingMode: "manual", currency_0: "CNY", period_0: "month", count_0: 1, amount_0: "92233720368547758.070", setup_0: "0.005" });
const detail = { product: { reference: "41", name: "Upstream", description: "" }, basePrices: [], options: [], priceScheduleComplete: false };
describe("upstream draft import", () => {
  beforeEach(() => { mounted.options = undefined; mounted.destroyed = false; });
  it("keeps exact prices and markup text, with no frontend rounding or inferred sale values", () => {
    expect(catalogImportInput(basic(), 1, "pfi", "41").markupPercent).toBe("10");
    const input = catalogImportInput(manual(), 1, "pfi", "41");
    expect(input.prices?.[0]).toEqual({ currencyCode: "CNY", period: "month", periodCount: 1, amount: "92233720368547758.070", setupFee: "0.005" });
    expect(input.markupPercent).toBeUndefined();
  });
  it("rejects missing category, guessed zero fees, invalid periods, duplicates and oversized price lists", () => {
    expect(() => catalogImportInput({ ...basic(), categoryNo: "" }, 1, "pfi", "41")).toThrow("invalidFields");
    for (const extra of [{ setup_0: "" }, { amount_0: "-1" }, { amount_0: "1e8" }, { count_0: 0 }, { period_0: "once", count_0: 2 }, { currency_0: "1" }]) {
      expect(() => catalogImportInput({ ...manual(), ...extra }, 1, "pfi", "41")).toThrow("invalidPrice");
    }
    expect(() => catalogImportInput({ ...manual(), currency_1: "CNY", period_1: "month", count_1: 1, amount_1: "1", setup_1: "0" }, 2, "pfi", "41")).toThrow("invalidPrice");
    expect(() => catalogImportInput(manual(), 121, "pfi", "41")).toThrow("invalidPrice");
  });
  it("reuses a key after failure, retains input, switches to a read-only result and never publishes", async () => {
    const importProduct = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ item: { productNo: "private", versionNo: "private" }, missingConfiguration: [{ fieldKey: "cpu_secret_key", nameZhCn: "处理器", nameEnUs: "Processor" }], pricingIncomplete: true });
    const importer = createCatalogImporter({} as HTMLElement, { client: { categories: async () => ({ categories: [{ categoryNo: "local", parentCategoryNo: "", nameZhCn: "分类", nameEnUs: "Category", deleted: false }] }), importProduct }, interfaceNo: () => "pfi", locale: () => "zh-CN", tr: key => key, onError: vi.fn() });
    await importer.open(detail);
    const submit = mounted.options!.onSubmit!;
    await submit(basic());
    expect(mounted.options!.errorText).toBe("importFailed");
    expect(mounted.options!.readOnly).toBe(false);
    await submit(basic());
    expect(importProduct).toHaveBeenCalledTimes(2);
    expect(importProduct.mock.calls[0]![0].requestKey).toBe(importProduct.mock.calls[1]![0].requestKey);
    expect(mounted.options!.readOnly).toBe(true);
    expect(mounted.options!.fields).toEqual([]);
    importer.destroy();
    await submit(basic());
    expect(importProduct).toHaveBeenCalledTimes(2);
  });
  it("does not resurrect an importer after its category request completes late", async () => {
    let resolve!: (value: { categories: [] }) => void;
    const importer = createCatalogImporter({} as HTMLElement, { client: { categories: () => new Promise(done => { resolve = done; }), importProduct: vi.fn() }, interfaceNo: () => "pfi", locale: () => "en-US", tr: key => key, onError: vi.fn() });
    const pending = importer.open(detail);
    importer.destroy();
    resolve({ categories: [] });
    await pending;
    expect(mounted.options).toBeUndefined();
  });
});
