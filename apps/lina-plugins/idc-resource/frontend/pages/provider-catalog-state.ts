/** Supplier identifiers remain selection values. Monetary values remain exact decimal text. */
export type ProviderCatalogProduct = {
  reference: string;
  name: string;
  description: string;
};
export type ProviderCatalogPrice = {
  currencyCode: string;
  period: "once" | "hour" | "day" | "month" | "year";
  periodCount: number;
  amount: string;
  setupFee: string;
};
export interface ProviderCatalogDetail {
  product: ProviderCatalogProduct;
  basePrices: ProviderCatalogPrice[];
  options: Array<{
    reference: string;
    name: string;
    choices: Array<{ reference: string; name: string; prices: ProviderCatalogPrice[] }>;
  }>;
  priceScheduleComplete: boolean;
}
export interface ProviderCatalogQuery { page: number; pageSize: number; keyword: string }
export interface ProviderCatalogPage { items: ProviderCatalogProduct[]; total: number }
export interface ProviderCatalogState extends ProviderCatalogPage {
  interfaceNo: string;
  currentPage: number;
  pageSize: number;
  keyword: string;
  listLoading: boolean;
  detailLoading: boolean;
  listFailed: boolean;
  detailFailed: boolean;
  detail?: ProviderCatalogDetail;
}

/** Own the current interface's page and explicit selection; late reads cannot restore stale data. */
export function createProviderCatalogState(changed: (state: ProviderCatalogState) => void) {
  let listGeneration = 0;
  let detailGeneration = 0;
  const emptyState = (interfaceNo = ""): ProviderCatalogState => ({
    interfaceNo, currentPage: 1, pageSize: 20, keyword: "", items: [], total: 0,
    listLoading: false, detailLoading: false, listFailed: false, detailFailed: false,
  });
  let state = emptyState();
  function update(values: Partial<ProviderCatalogState>) {
    state = { ...state, ...values };
    changed(state);
  }
  function reset(interfaceNo = "") {
    ++listGeneration;
    ++detailGeneration;
    state = emptyState(interfaceNo);
    changed(state);
  }
  return {
    get current() { return state; },
    reset,
    async query(
      interfaceNo: string,
      requested: { currentPage: number; pageSize: number },
      keyword: string,
      load: (interfaceNo: string, query: ProviderCatalogQuery) => Promise<ProviderCatalogPage>,
    ) {
      if (state.interfaceNo !== interfaceNo) reset(interfaceNo);
      const generation = ++listGeneration;
      ++detailGeneration;
      const normalizedKeyword = keyword.trim();
      const currentPage = requested.pageSize !== state.pageSize || normalizedKeyword !== state.keyword
        ? 1 : requested.currentPage;
      update({ currentPage, pageSize: requested.pageSize, keyword: normalizedKeyword,
        items: [], total: 0, listLoading: true, listFailed: false, detail: undefined, detailLoading: false, detailFailed: false });
      try {
        const result = await load(interfaceNo, { page: currentPage, pageSize: requested.pageSize, keyword: normalizedKeyword });
        if (generation !== listGeneration) return state;
        if (!Number.isSafeInteger(result.total) || result.total < 0 || !Array.isArray(result.items) ||
            result.items.length > requested.pageSize || result.items.length > result.total) throw new Error("Invalid supplier catalog page");
        update({ items: result.items, total: result.total });
      } catch {
        if (generation === listGeneration) update({ items: [], total: 0, listFailed: true });
      } finally {
        if (generation === listGeneration) update({ listLoading: false });
      }
      return state;
    },
    async select(reference: string, load: (interfaceNo: string, reference: string) => Promise<{ item: ProviderCatalogDetail }>) {
      const generation = ++detailGeneration;
      const interfaceNo = state.interfaceNo;
      update({ detail: undefined, detailLoading: true, detailFailed: false });
      try {
        const result = await load(interfaceNo, reference);
        if (generation !== detailGeneration) return;
        if (result.item?.product.reference !== reference) throw new Error("Supplier product selection changed");
        update({ detail: result.item });
      } catch {
        if (generation === detailGeneration) update({ detail: undefined, detailFailed: true });
      } finally {
        if (generation === detailGeneration) update({ detailLoading: false });
      }
    },
  };
}

/** Flatten already-loaded surcharges without calculating or converting money. */
export function providerCatalogPriceRows(detail?: ProviderCatalogDetail) {
  return {
    base: (detail?.basePrices ?? []).map((price, index) => ({ ...price, rowKey: `base-${index}` })),
    options: (detail?.options ?? []).flatMap((option, optionIndex) => option.choices.flatMap((choice, choiceIndex) =>
      choice.prices.map((price, priceIndex) => ({ ...price, groupName: option.name, choiceName: choice.name, rowKey: `${optionIndex}-${choiceIndex}-${priceIndex}` })),
    )),
  };
}
