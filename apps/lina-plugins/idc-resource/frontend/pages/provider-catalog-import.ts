import type { IsolatedFormDrawerHandle, IsolatedFormField } from "@linapro/vben-plugin-adapter/isolated";
import { mountIsolatedVbenFormDrawer } from "@linapro/vben-plugin-adapter/isolated";
import type { ProviderCatalogDetail, ProviderCatalogPrice } from "./provider-catalog-state";

export interface ImportCategory { categoryNo: string; parentCategoryNo: string; nameZhCn: string; nameEnUs: string; deleted: boolean }
export interface CatalogImportInput {
  requestKey: string; interfaceInstanceNo: string; reference: string; categoryNo: string;
  nameZhCn: string; nameEnUs: string; pricingMode: "markup" | "manual";
  markupPercent?: string; prices?: ProviderCatalogPrice[];
}
export interface CatalogImportResult {
  item: { productNo: string; versionNo: string };
  missingConfiguration: Array<{ fieldKey: string; nameZhCn: string; nameEnUs: string }>;
  pricingIncomplete: boolean;
}
export interface CatalogImportClient {
  categories: () => Promise<{ categories: ImportCategory[] }>;
  importProduct: (input: CatalogImportInput) => Promise<CatalogImportResult>;
}
const decimal = /^(?:0|[1-9]\d{0,23})(?:\.\d{1,12})?$/u;
const periods = ["once", "hour", "day", "month", "year"] as const;

/** Never use JS floating point to calculate prices or silently invent missing values. */
export function catalogImportInput(values: Record<string, unknown>, count: number, interfaceNo: string, reference: string): Omit<CatalogImportInput, "requestKey"> {
  const text = (key: string) => String(values[key] ?? "").trim();
  if (values.pricingMode !== "manual" && values.pricingMode !== "markup") throw new Error("invalidFields");
  const input: Omit<CatalogImportInput, "requestKey"> = {
    interfaceInstanceNo: interfaceNo, reference, categoryNo: text("categoryNo"), nameZhCn: text("nameZhCn"), nameEnUs: text("nameEnUs"),
    pricingMode: values.pricingMode === "manual" ? "manual" : "markup",
  };
  if (!input.categoryNo || !input.nameZhCn || !input.nameEnUs || input.nameZhCn.length > 256 || input.nameEnUs.length > 256) throw new Error("invalidFields");
  if (input.pricingMode === "markup") {
    input.markupPercent = text("markupPercent");
    if (!decimal.test(input.markupPercent)) throw new Error("invalidPrice");
  } else {
    if (!Number.isSafeInteger(count) || count < 1 || count > 120) throw new Error("invalidPrice");
    const keys = new Set<string>();
    input.prices = Array.from({ length: count }, (_, index) => {
      const currencyCode = text(`currency_${index}`).toUpperCase();
      const period = text(`period_${index}`) as ProviderCatalogPrice["period"];
      const periodCount = Number(values[`count_${index}`]);
      const amount = text(`amount_${index}`);
      const setupFee = text(`setup_${index}`);
      const key = `${currencyCode}:${period}:${periodCount}`;
      if (!/^[A-Z]{3}$/u.test(currencyCode) || !periods.includes(period) || !Number.isSafeInteger(periodCount) || periodCount < 1 || periodCount > 120 ||
        (period === "once" && periodCount !== 1) || !decimal.test(amount) || !decimal.test(setupFee) || keys.has(key)) throw new Error("invalidPrice");
      keys.add(key);
      return { currencyCode, period, periodCount, amount, setupFee };
    });
  }
  return input;
}

/** Both page delivery modes use the same real Vben form/drawer and submission contract. */
export function createCatalogImporter(root: HTMLElement, options: {
  client: CatalogImportClient; interfaceNo: () => string; locale: () => string; tr: (key: string) => string;
  onError: (message: string) => void;
}) {
  const { tr } = options;
  let generation = 0;
  let drawer: IsolatedFormDrawerHandle | undefined;
  let destroyed = false;
  return {
    async open(detail: ProviderCatalogDetail) {
      const revision = ++generation;
      const interfaceNo = options.interfaceNo();
      drawer?.destroy();
      drawer = undefined;
      options.onError("");
      let categories: ImportCategory[];
      try { categories = (await options.client.categories()).categories.filter(item => !item.deleted); }
      catch { if (!destroyed && revision === generation) options.onError(tr("categoriesFailed")); return; }
      if (destroyed || revision !== generation || interfaceNo !== options.interfaceNo()) return;
      const nodes = new Map(categories.map(item => [item.categoryNo, { value: item.categoryNo,
        label: options.locale() === "zh-CN" ? item.nameZhCn || item.nameEnUs : item.nameEnUs || item.nameZhCn, children: [] as unknown[] }]));
      const tree: unknown[] = [];
      for (const item of categories) {
        const node = nodes.get(item.categoryNo)!;
        const parent = nodes.get(item.parentCategoryNo);
        if (parent && parent !== node) parent.children.push(node); else tree.push(node);
      }
      let count = Math.max(1, detail.basePrices.length);
      let values: Record<string, unknown> = { nameZhCn: detail.product.name, nameEnUs: detail.product.name, pricingMode: "markup", markupPercent: "" };
      detail.basePrices.forEach((price, index) => Object.assign(values, { [`currency_${index}`]: price.currencyCode, [`period_${index}`]: price.period, [`count_${index}`]: price.periodCount }));
      let fingerprint = "";
      let requestKey = "";
      const active = () => !destroyed && revision === generation;
      const showText = (lines: string[]) => (container: HTMLElement) => {
        container.dataset.testid = "provider-import-result";
        for (const line of lines) { const p = document.createElement("p"); p.textContent = line; p.style.marginBottom = "12px"; container.append(p); }
      };
      function fields(): IsolatedFormField[] {
        const items: IsolatedFormField[] = [
          { name: "categoryNo", label: tr("category"), component: "TreeSelect", required: true, props: { treeData: tree, treeDefaultExpandAll: true, showSearch: true, treeNodeFilterProp: "label", "data-testid": "provider-import-category" } },
          { name: "nameZhCn", label: tr("nameZhCn"), component: "Input", required: true, props: { maxlength: 256, "data-testid": "provider-import-name-zh" } },
          { name: "nameEnUs", label: tr("nameEnUs"), component: "Input", required: true, props: { maxlength: 256 } },
          { name: "pricingMode", label: tr("pricingMode"), component: "Select", required: true, props: { "data-testid": "provider-import-pricing-mode", options: ["markup", "manual"].map(value => ({ value, label: tr(value) })) } },
        ];
        if (values.pricingMode !== "manual") {
          items.push({ name: "markupPercent", label: tr("markupPercent"), component: "Input", required: true, description: tr("markupHint"), props: { inputmode: "decimal", maxlength: 37, suffix: "%", "data-testid": "provider-import-markup" } });
        } else {
          for (let index = 0; index < count; index++) {
            const prefix = `${index + 1} · `;
            items.push(
              { name: `currency_${index}`, label: prefix + tr("currency"), component: "Input", required: true, props: { maxlength: 3, placeholder: "CNY / USD", "data-testid": `provider-import-currency-${index}` } },
              { name: `period_${index}`, label: prefix + tr("period"), component: "Select", required: true, props: { options: periods.map(value => ({ value, label: tr(`periods.${value}`) })), "data-testid": `provider-import-period-${index}` } },
              { name: `count_${index}`, label: prefix + tr("periodCount"), component: "InputNumber", required: true, props: { min: 1, max: 120, precision: 0, "data-testid": `provider-import-count-${index}` } },
              { name: `amount_${index}`, label: prefix + tr("finalPrice"), component: "Input", required: true, props: { inputmode: "decimal", maxlength: 37, "data-testid": `provider-import-amount-${index}` } },
              { name: `setup_${index}`, label: prefix + tr("setupFee"), component: "Input", required: true, description: tr("setupHint"), props: { inputmode: "decimal", maxlength: 37, "data-testid": `provider-import-setup-${index}` } },
            );
          }
        }
        return items;
      }
      function actions() {
        if (values.pricingMode !== "manual") return [];
        return [
          { key: "add-price", label: tr("addPrice"), disabled: count >= 120, onClick: (current: Record<string, unknown>) => { values = { ...values, ...current }; count++; update(true); } },
          { key: "remove-price", label: tr("removePrice"), disabled: count <= 1, onClick: (current: Record<string, unknown>) => { values = { ...values, ...current }; count--; for (const key of ["currency", "period", "count", "amount", "setup"]) delete values[`${key}_${count}`]; update(true); } },
        ];
      }
      function update(reset = false) { if (active()) drawer?.update({ fields: fields(), actions: actions(), ...(reset ? { values } : {}) }); }
      drawer = mountIsolatedVbenFormDrawer(root, {
        title: tr("importTitle"), description: tr("draftHint"), locale: options.locale(),
        fields: categories.length ? fields() : [], values, readOnly: !categories.length,
        mountContent: showText([categories.length ? tr("configurationHint") : tr("noCategories")]),
        onValuesChange(current) { if (!active()) return; const previous = values.pricingMode; values = { ...values, ...current }; if (previous !== values.pricingMode) update(); },
        onClosed() { if (active()) { generation++; drawer?.destroy(); drawer = undefined; } },
        onError() { if (active()) drawer?.update({ errorText: tr("importFailed") }); },
        async onSubmit(current) {
          if (!active()) return false;
          values = { ...values, ...current };
          let input: Omit<CatalogImportInput, "requestKey">;
          try {
            input = catalogImportInput(values, count, interfaceNo, detail.product.reference);
            if (!categories.some(item => item.categoryNo === input.categoryNo)) throw new Error("invalidFields");
          } catch (error) { drawer?.update({ errorText: tr(error instanceof Error && error.message === "invalidPrice" ? "invalidPrice" : "invalidFields") }); return false; }
          const nextFingerprint = JSON.stringify(input);
          if (nextFingerprint !== fingerprint) { fingerprint = nextFingerprint; requestKey = crypto.randomUUID(); }
          drawer?.update({ errorText: "" });
          try {
            const result = await options.client.importProduct({ ...input, requestKey });
            if (!active()) return false;
            const missing = result.missingConfiguration.map(item => options.locale() === "zh-CN" ? item.nameZhCn || item.nameEnUs : item.nameEnUs || item.nameZhCn).filter(Boolean);
            const chinese = options.locale() === "zh-CN";
            const lines = [tr("importSucceeded"), chinese ? input.nameZhCn : input.nameEnUs, tr("finishDraftHint")];
            if (missing.length) lines.push(`${tr("missingConfiguration")}${chinese ? "：" : ": "}${missing.join(chinese ? "、" : ", ")}`);
            if (result.pricingIncomplete) lines.push(tr("pricingIncomplete"));
            drawer?.update({ readOnly: true, fields: [], actions: [], mountContent: showText(lines) });
          } catch { if (active()) drawer?.update({ errorText: tr("importFailed") }); }
          return false;
        },
      });
      try { await drawer.ready; } catch { if (active()) { drawer.destroy(); drawer = undefined; options.onError(tr("importFailed")); } }
    },
    destroy() { destroyed = true; generation++; drawer?.destroy(); drawer = undefined; },
  };
}
